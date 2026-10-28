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
	// Failures is how many times the export was refused.
	Failures int `json:"failures"`
	// Counted says whether the current attempt has already been counted.
	Counted bool `json:"-"`
	// FirstRefusal is when the export was first refused.
	FirstRefusal time.Time `json:"-"`
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

// How long the export has to keep failing before it is given up on.
//
// The two refusals have to be apart in time. A caption dialog that fails
// once is retried — the interface does it by itself, and so does a person
// clicking again — and those retries are seconds apart. An export that is
// refused twice within a minute has not been given a chance.
//
// The export is also not removed on the spot: it stays for a while, because
// a cookie that has just expired is a cookie that a fresh export would fix,
// and the automatic export underneath is not always a session the person
// using the interface wants.
const (
	// The two refusals must be this far apart.
	refusalSeparation = 5 * time.Minute
	// How long the export survives after its second refusal.
	refusalGrace = 30 * time.Minute
)

// Failed records that the imported export was refused.
//
// The export is removed when it has been refused twice, with the two
// refusals at least a few minutes apart, and it is kept for a while after
// that so the person can import a fresh one before the automatic export
// takes over.
//
// One failure is one attempt, not one message. yt-dlp reports the rotated
// cookie once per request, and a download makes several requests, so a
// single failed download produced several warnings — and counting them
// would have removed the export after one attempt rather than two.
func (s *Store) Failed() *Imported {
	s.mu.Lock()

	if s.current == nil {
		s.mu.Unlock()
		return nil
	}

	// The attempt is already counted. The refusal is reported again by the
	// next request of the same run, and counting that again would make the
	// second attempt look like the second failure.
	if s.current.Counted {
		out := *s.current
		s.mu.Unlock()
		return &out
	}

	s.current.Counted = true
	s.current.Failures++

	// The first refusal is when the clock starts.
	if s.current.FirstRefusal.IsZero() {
		s.current.FirstRefusal = time.Now()
		out := *s.current
		s.mu.Unlock()
		return &out
	}

	// The second refusal has to be far enough from the first. Two retries
	// seconds apart are one failure that the interface retried, not two
	// failures.
	if time.Since(s.current.FirstRefusal) < refusalSeparation {
		out := *s.current
		s.mu.Unlock()
		return &out
	}

	// The export is not working. It is kept on disk for a while, so the
	// person can import a fresh one; it is simply not used in the meantime.
	path := s.current.Path
	s.current = nil
	s.mu.Unlock()

	// The file is removed later, not now: the automatic export underneath
	// is a fallback, not necessarily the session this person wants.
	go s.removeLater(path)

	return nil
}

// removeLater deletes the imported file after the grace period.
//
// The file is not deleted at once because the export might be recoverable:
// a cookie that has just expired is fixed by exporting it again, and the
// person may well do that within the hour.
func (s *Store) removeLater(path string) {
	time.Sleep(refusalGrace)

	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		fmt.Printf("could not remove the imported cookie file %s: %v\n", path, err)
	}
}

// NewAttempt starts counting a new attempt.
//
// It is called when a request begins, so that the warnings of one attempt are
// counted once rather than once each.
func NewAttempt() {
	store := ImportedGlobal()
	if store == nil {
		return
	}

	store.newAttempt()
}

// newAttempt is the store's own part of it.
func (s *Store) newAttempt() {
	s.mu.Lock()
	if s.current != nil {
		s.current.Counted = false
	}
	s.mu.Unlock()
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
