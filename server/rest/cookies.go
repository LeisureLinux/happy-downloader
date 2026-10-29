package rest

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"

	"github.com/LeisureLinux/happy-downloader/v4/server/internal/cookies"
)

// Importing a cookie export.
//
// The automatic export belongs to whoever set up the host: a browser profile
// read by the refresh script and copied over by a timer. It is not always the
// credential the person using the interface wants. A user who signs into
// YouTube on their own machine and exports the cookies with a browser
// extension has exactly the session they want, and no timer is going to
// produce it.
//
// So the file can be uploaded here, and while it works it is the cookie the
// downloads use. After it has been refused twice it is removed, and the
// automatic export takes over again.

// MaxImportSize is the largest upload accepted.
//
// A cookie export is a few kilobytes. The limit is here so that a mistake —
// or a browser that uploads its whole profile — does not fill the disk.
const MaxImportSize = 4 << 20 // 4 MiB

// ImportCookies stores an uploaded cookie export.
func (h *Handler) ImportCookies() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()

		store := cookies.ImportedGlobal()
		if store == nil {
			http.Error(w, "cookie import is not available", http.StatusServiceUnavailable)
			return
		}

		// Read a byte more than the limit, so a file that is exactly at the
		// limit is accepted and one that is over it is refused rather than
		// silently truncated.
		body, err := io.ReadAll(io.LimitReader(r.Body, MaxImportSize+1))
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		if len(body) > MaxImportSize {
			http.Error(w, "the file is too large to be a cookie export",
				http.StatusRequestEntityTooLarge)
			return
		}

		if len(body) == 0 {
			http.Error(w, "the file is empty", http.StatusBadRequest)
			return
		}

		imported, err := store.Import(body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		// Nothing has to be published: the downloads ask which export is the
		// newer of the two when they are built, so an import takes effect on
		// the next download without anything being told.

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(map[string]any{
			"path":       imported.Path,
			"format":     imported.Format,
			"count":      imported.Count,
			"modifiedAt": imported.ModifiedAt.Format("2006-01-02T15:04:05Z07:00"),
		}); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}
}

// RemoveImportedCookies forgets the imported export.
//
// The automatic export takes over again. The uploaded file is deleted rather
// than ignored, so a credential that is no longer wanted is not left on the
// disk.
func (h *Handler) RemoveImportedCookies() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		store := cookies.ImportedGlobal()
		if store == nil {
			http.Error(w, "cookie import is not available", http.StatusServiceUnavailable)
			return
		}

		store.Forget()

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode("ok")
	}
}

// ImportedCookieDir returns the directory the uploaded export is written to.
//
// It is under the state directory, which is already the place this server
// writes to and is not the download directory — so an uploaded credential
// never shows up in the file browser.
func ImportedCookieDir(stateDir string) string {
	if stateDir == "" {
		return filepath.Join(os.TempDir(), "happy-downloader")
	}

	return filepath.Join(stateDir, "cookies")
}
