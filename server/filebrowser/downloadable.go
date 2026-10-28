package filebrowser

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/LeisureLinux/happy-downloader/v4/server/config"
)

// Downloading a file and its captions.
//
// The captions used to arrive inside a zip with the video. That was a mistake
// for anyone who wanted the video itself: a phone saves a .zip and plays
// nothing, and unzipping on a phone is a chore.
//
// So the video and each caption file are separate downloads now, and the
// interface offers them as such. The endpoint that lists what can be
// downloaded exists for that: the file browser asks it, and shows a button
// for each file rather than one button that produces an archive.

// Downloadable is one file that can be downloaded.
type Downloadable struct {
	// Path is the file on disk.
	Path string `json:"path"`
	// Name is the name it is downloaded under.
	Name string `json:"name"`
	// URL is where to fetch it.
	URL string `json:"url"`
	// Kind is "video" or "caption", so the interface can tell them apart.
	Kind string `json:"kind"`
	// Language is the caption's language code, for a caption.
	Language string `json:"language,omitempty"`
	// Label is the caption's human-readable name.
	Label string `json:"label,omitempty"`
	// Size is the file's size in bytes.
	Size int64 `json:"size"`
}

// ListDownloadable reports the files that can be downloaded for one file.
//
// It is the same list the download button uses, so the interface and the
// server cannot disagree about what exists.
func ListDownloadable(w http.ResponseWriter, r *http.Request) {
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

	if _, err := os.Stat(clean); err != nil {
		http.Error(w, "the file is not there", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	if err := json.NewEncoder(w).Encode(downloadablesFor(clean)); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

// downloadablesFor builds the list of files that belong to one video.
func downloadablesFor(path string) []Downloadable {
	out := []Downloadable{}

	if info, err := os.Stat(path); err == nil {
		out = append(out, Downloadable{
			Path: path,
			Name: filepath.Base(path),
			URL:  downloadURL(path),
			Kind: "video",
			Size: info.Size(),
		})
	}

	for _, c := range subtitlesFor(filepath.Dir(path), filepath.Base(path)) {
		size := int64(0)
		if info, err := os.Stat(c.Path); err == nil {
			size = info.Size()
		}

		out = append(out, Downloadable{
			Path:     c.Path,
			Name:     filepath.Base(c.Path),
			URL:      downloadURL(c.Path),
			Kind:     "caption",
			Language: c.Language,
			Label:    c.Label,
			Size:     size,
		})
	}

	return out
}

// downloadURL is the address a file is fetched from.
//
// The path is encoded the way the download endpoint decodes it — standard
// base64, then URL-escaped — so the two cannot disagree about the encoding.
func downloadURL(path string) string {
	encoded := base64.StdEncoding.EncodeToString([]byte(filepath.Clean(path)))

	return "/filebrowser/d/" + url.QueryEscape(encoded)
}

// withinDownloadDir reports whether a path is inside the download directory.
//
// The check is on the directory of the path, because the path is a file
// inside it. It is a prefix check rather than a comparison, so the download
// directory "/data/downloads" does not also contain "/data/downloads-old".
func withinDownloadDir(path string) bool {
	root := filepath.Clean(config.Instance().Paths.DownloadPath)

	dir := filepath.Clean(filepath.Dir(path))

	return dir == root || strings.HasPrefix(dir, root+string(filepath.Separator))
}

// resolveSubDir works out which directory a listing is about.
//
// The subdirectory arrives either as a path relative to the download
// directory, or as an absolute one — the interface keeps the absolute path of
// the folder it is showing, so that is what it sends.
//
// filepath.Join would have been wrong for the second case: it joins, so an
// absolute path became "<root>/home/axu/media/downloads/folder", which is
// nowhere. The result is checked against the download directory afterwards,
// so a path that climbs out of it is refused rather than served.
func resolveSubDir(root, sub string) string {
	clean := filepath.Clean(sub)

	if filepath.IsAbs(clean) {
		return clean
	}

	return filepath.Join(root, clean)
}
