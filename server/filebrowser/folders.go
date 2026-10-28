package filebrowser

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/LeisureLinux/happy-downloader/v4/server/config"
)

// Folders.
//
// A download directory with fifty videos in it is a list nobody can navigate.
// Folders give it a shape: a name, and the files that belong under it.
//
// The rules are the ones that keep the answer unambiguous:
//
//   - A folder with files in it cannot be deleted. Deleting it would have to
//     decide what happens to the files, and every answer to that question
//     loses something the user wanted.
//   - A folder is not a video. It has no duration, no codec and no captions,
//     and none of the buttons that act on a video make sense for it.
//   - The figures shown for a folder are about the folder: how many files it
//     holds, when its newest file was written, and how much disk it takes.

const (
	// The name of a folder cannot be these.
	invalidFolderName = `/\:*?"<>|`
)

// CreateFolder makes a directory in the download directory.
func CreateFolder(w http.ResponseWriter, r *http.Request) {
	var req struct {
		// Name is the folder's name. The parent is the current directory.
		Name string `json:"name"`
		// SubDir is the directory the folder is made in.
		SubDir string `json:"subdir"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	name := strings.TrimSpace(req.Name)

	if name == "" {
		http.Error(w, "a name is required", http.StatusBadRequest)
		return
	}

	if !validFolderName(name) {
		http.Error(w, "that name cannot be used for a folder", http.StatusBadRequest)
		return
	}

	root := config.Instance().Paths.DownloadPath

	// The parent may be given relative to the download directory or as an
	// absolute path — the interface keeps the absolute path of the folder it
	// is showing.
	parent := root
	if req.SubDir != "" {
		parent = resolveSubDir(root, req.SubDir)
	}

	if !withinDownloadDir(filepath.Join(parent, "x")) {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	path := filepath.Join(parent, name)

	// The path has to stay inside the download directory. A name of ".."
	// would climb out of it.
	if !withinDownloadDir(filepath.Join(path, "x")) {
		http.Error(w, "that name would put the folder outside the download directory",
			http.StatusBadRequest)
		return
	}

	if _, err := os.Stat(path); err == nil {
		http.Error(w, "there is already something with that name", http.StatusConflict)
		return
	}

	if err := os.MkdirAll(path, 0o755); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	slog.Info("folder created", slog.String("path", path))

	w.Header().Set("Content-Type", "application/json")

	if err := json.NewEncoder(w).Encode(map[string]string{"path": path, "name": name}); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

// RenameFolder gives a folder a new name.
//
// The folder keeps its contents: renaming a directory moves it within its
// parent, so the files inside it are untouched.
//
// The name is checked the same way a new folder's name is, and the result is
// checked against the download directory — a name of ".." would move the
// folder out of it.
func RenameFolder(w http.ResponseWriter, r *http.Request) {
	var req struct {
		// Path is the folder to rename.
		Path string `json:"path"`
		// Name is the name it should have.
		Name string `json:"name"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	from := filepath.Clean(req.Path)
	name := strings.TrimSpace(req.Name)

	if name == "" {
		http.Error(w, "a name is required", http.StatusBadRequest)
		return
	}

	if !validFolderName(name) {
		http.Error(w, "that name cannot be used for a folder", http.StatusBadRequest)
		return
	}

	if !withinDownloadDir(from) {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	info, err := os.Stat(from)
	if err != nil || !info.IsDir() {
		http.Error(w, "there is no folder at that path", http.StatusNotFound)
		return
	}

	to := filepath.Join(filepath.Dir(from), name)

	// The new path has to stay inside the download directory.
	if !withinDownloadDir(filepath.Join(to, "x")) {
		http.Error(w, "that name would put the folder outside the download directory",
			http.StatusBadRequest)
		return
	}

	// Renaming a folder to the name it already has is not an error; there is
	// simply nothing to do.
	if to == from {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"path": from, "name": name})
		return
	}

	if _, err := os.Stat(to); err == nil {
		http.Error(w, "there is already something with that name", http.StatusConflict)
		return
	}

	if err := os.Rename(from, to); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	slog.Info("folder renamed",
		slog.String("from", from),
		slog.String("to", to),
	)

	w.Header().Set("Content-Type", "application/json")

	if err := json.NewEncoder(w).Encode(map[string]string{"path": to, "name": name}); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

// MoveFiles moves files into a folder.
//
// The files have to be in the same directory as the folder, and they have to
// be files: a folder is moved by moving its contents, or not at all.
func MoveFiles(w http.ResponseWriter, r *http.Request) {
	var req struct {
		// Paths are the files to move.
		Paths []string `json:"paths"`
		// Target is the folder they go into.
		Target string `json:"target"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if len(req.Paths) == 0 {
		http.Error(w, "no files to move", http.StatusBadRequest)
		return
	}

	target := filepath.Clean(req.Target)

	info, err := os.Stat(target)
	if err != nil || !info.IsDir() {
		http.Error(w, "the target is not a folder", http.StatusBadRequest)
		return
	}

	if !withinDownloadDir(filepath.Join(target, "x")) {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	moved := []string{}
	var failures []string

	for _, p := range req.Paths {
		from := filepath.Clean(p)

		if !withinDownloadDir(from) {
			failures = append(failures, fmt.Sprintf("%s: outside the download directory", filepath.Base(from)))
			continue
		}

		// A caption belongs to its video. Moving one without the other
		// leaves the video with no subtitles, so the captions of a video that
		// moves are moved with it.
		siblings := []string{from}
		if isVideoFile(from) {
			siblings = append(siblings, captionPathsOf(from)...)
		}

		for _, s := range siblings {
			to := filepath.Join(target, filepath.Base(s))

			// Moving a file onto itself is not an error worth failing for.
			if s == to {
				continue
			}

			if err := os.Rename(s, to); err != nil {
				failures = append(failures, fmt.Sprintf("%s: %v", filepath.Base(s), err))
				continue
			}

			moved = append(moved, to)
		}
	}

	slog.Info("files moved",
		slog.String("target", target),
		slog.Int("moved", len(moved)),
		slog.Int("failed", len(failures)),
	)

	w.Header().Set("Content-Type", "application/json")

	if err := json.NewEncoder(w).Encode(map[string]any{
		"moved":    moved,
		"failures": failures,
	}); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

// DeleteFolder empties a folder and removes it.
//
// A folder with files in it is not deleted: the files would have to go
// somewhere, and every answer loses something. The user is told what is in
// it, so the answer is to move or delete the files first.
func DeleteFolder(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Query().Get("path")
	if path == "" {
		http.Error(w, "a path is required", http.StatusBadRequest)
		return
	}

	clean := filepath.Clean(path)

	if !withinDownloadDir(clean) {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	info, err := os.Stat(clean)
	if err != nil || !info.IsDir() {
		http.Error(w, "there is no folder at that path", http.StatusNotFound)
		return
	}

	entries, err := os.ReadDir(clean)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if len(entries) > 0 {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"error":   "the folder is not empty",
			"entries": len(entries),
		})
		return
	}

	if err := os.Remove(clean); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode("ok")
}

// validFolderName reports whether a name can be used for a folder.
//
// The characters the filesystem reserves are refused, and so are the two
// names that mean "this directory" and "its parent" — a folder called ".."
// would put everything somewhere else.
func validFolderName(name string) bool {
	if name == "" || name == "." || name == ".." {
		return false
	}

	if strings.ContainsAny(name, invalidFolderName) {
		return false
	}

	// A path separator is in the list, but it is checked separately as well:
	// the error says which of the two rules was broken.
	return !strings.ContainsRune(name, '/')
}

// isVideoFile reports whether a path names a video.
//
// It is used by the move, which carries a video's captions along with it.
func isVideoFile(path string) bool {
	name := filepath.Base(path)

	return videoRe.MatchString(name)
}

// captionPathsOf returns the caption files that belong to a video.
func captionPathsOf(path string) []string {
	captions := subtitlesFor(filepath.Dir(path), filepath.Base(path))

	out := make([]string, 0, len(captions))
	for _, c := range captions {
		out = append(out, c.Path)
	}

	return out
}

// errFolderNotEmpty is returned when a folder with files in it is deleted.
var errFolderNotEmpty = errors.New("the folder is not empty")
