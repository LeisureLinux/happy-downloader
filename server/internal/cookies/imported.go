package cookies

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// The cookie export the user imported through the interface.
//
// The automatic export — a browser profile read by the refresh script, copied
// onto the host by a timer — is the fallback. This one is what the user
// uploaded, and it wins while it works.
//
// The reason it is worth a separate concept: the automatic export belongs to
// whoever set up the host, and it is not always the credential the person
// using the interface has. A user who signs into YouTube on their own
// machine and exports the cookies from an extension has exactly the session
// they want, and no timer is going to produce it.
//
// So the uploaded file is preferred, and the automatic one is still there
// underneath: when the uploaded file has failed twice it is removed, and the
// automatic export takes over again.
//
// A failed upload is not the same as a failed session. A file that cannot be
// parsed is rejected at upload time — there is no point storing it — and the
// retry counter is only about a session that parsed fine and was then
// refused by the site.

// Imported is the state of the uploaded cookie export.
type Imported struct {
	// Path is where the uploaded file was written.
	Path string `json:"path"`
	// Format is the shape it was recognised as.
	Format string `json:"format"`
	// Count is how many cookies it holds.
	Count int `json:"count"`
	// ModifiedAt is when it was uploaded.
	ModifiedAt time.Time `json:"modifiedAt"`
	// Failures is how many times a download using it was refused.
	Failures int `json:"failures"`
}

// The active store.
//
// The store is created once at startup and published here, so that the
// layers that need it — the HTTP handlers, the status report — can find it
// without every one of them taking it as a parameter. The downloader cannot
// import this package's store (the two would depend on each other), so it
// gets the path through its own hook instead.
var (
	globalMu    sync.RWMutex
	globalStore *Store
)

// SetGlobal publishes the store.
func SetGlobal(s *Store) {
	globalMu.Lock()
	globalStore = s
	globalMu.Unlock()
}

// ImportedGlobal returns the store, or nil when there is none.
func ImportedGlobal() *Store {
	globalMu.RLock()
	defer globalMu.RUnlock()

	return globalStore
}

// Store keeps the imported export and the record of how it is doing.
type Store struct {
	// dir is where the uploaded file is written.
	dir string

	mu sync.RWMutex
	// current is the imported export, if there is one.
	current *Imported
}

// NewStore creates a store rooted at a directory.
func NewStore(dir string) *Store {
	return &Store{dir: dir}
}

// The file name the uploaded export is written to.
//
// A fixed name, so that uploading a new one replaces the old one rather than
// leaving a pile of them behind.
const importedFileName = "imported-cookies.json"

// Path returns the file the uploaded export is written to.
func (s *Store) Path() string {
	return filepath.Join(s.dir, importedFileName)
}

// Import writes an uploaded export and makes it the one in use.
//
// The format is recognised from the content, so the same endpoint takes the
// JSON that a browser extension produces and a Netscape cookies.txt. A file
// that cannot be recognised is refused: storing it would only produce a
// download that fails later, with the reason a long way from the mistake.
func (s *Store) Import(content []byte) (*Imported, error) {
	converted, format, err := Convert(content)
	if err != nil {
		return nil, err
	}

	summary := Summarize(converted)

	// A guest session is not a credential. YouTube hands out cookies to
	// anyone who visits, so a file with no login cookie is a file that looks
	// like an export and behaves like an anonymous visitor.
	if missing := Required(summary.Names); len(missing) > 0 {
		return nil, fmt.Errorf(
			"the file is missing the login cookies %s, so it is a guest session rather than a signed-in one",
			strings.Join(missing, ", "))
	}

	if err := os.MkdirAll(s.dir, 0o700); err != nil {
		return nil, err
	}

	// Written to a temporary name and renamed, so a reader never sees a
	// half-written file.
	tmp := s.Path() + ".tmp"

	if err := os.WriteFile(tmp, content, 0o600); err != nil {
		return nil, err
	}

	if err := os.Rename(tmp, s.Path()); err != nil {
		os.Remove(tmp)
		return nil, err
	}

	imported := &Imported{
		Path:       s.Path(),
		Format:     string(format),
		Count:      summary.Total,
		ModifiedAt: time.Now(),
	}

	s.mu.Lock()
	s.current = imported
	s.mu.Unlock()

	return imported, nil
}

// Current returns the imported export, if there is one.
func (s *Store) Current() *Imported {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.current == nil {
		return nil
	}

	// A copy, so the caller cannot change the stored one.
	out := *s.current
	return &out
}

// Failed records that a download using the imported export was refused.
//
// After two failures the export is removed: it is not working, and the
// automatic export underneath is a better answer than a credential the site
// has rejected. The file is deleted rather than merely ignored, so a
// credential that no longer works is not left lying around.
func (s *Store) Failed() *Imported {
	s.mu.Lock()

	if s.current == nil {
		s.mu.Unlock()
		return nil
	}

	s.current.Failures++

	if s.current.Failures < 2 {
		out := *s.current
		s.mu.Unlock()
		return &out
	}

	// The second failure: the export is not working.
	path := s.current.Path
	s.current = nil
	s.mu.Unlock()

	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		// The file stays on disk if it cannot be removed. The import is
		// forgotten anyway, so it is not used.
		fmt.Printf("could not remove the imported cookie file %s: %v\n", path, err)
	}

	return nil
}

// Forget removes the imported export.
func (s *Store) Forget() {
	s.mu.Lock()
	path := s.current.Path
	s.current = nil
	s.mu.Unlock()

	os.Remove(path)
}

// Restore reloads the imported export from disk.
//
// The upload is remembered across a restart: the file is still there, and the
// user did not ask for it to be forgotten.
func (s *Store) Restore() {
	content, err := os.ReadFile(s.Path())
	if err != nil {
		return
	}

	converted, format, err := Convert(content)
	if err != nil {
		return
	}

	summary := Summarize(converted)

	info, err := os.Stat(s.Path())
	if err != nil {
		return
	}

	s.mu.Lock()
	s.current = &Imported{
		Path:       s.Path(),
		Format:     string(format),
		Count:      summary.Total,
		ModifiedAt: info.ModTime(),
	}
	s.mu.Unlock()
}
