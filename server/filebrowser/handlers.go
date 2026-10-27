package filebrowser

import (
	"archive/zip"
	"encoding/base64"
	"encoding/json"
	"io"
	"io/fs"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/LeisureLinux/happy-downloader/v4/server/config"
	"github.com/LeisureLinux/happy-downloader/v4/server/internal"
	"github.com/LeisureLinux/happy-downloader/v4/server/internal/kv"
	"github.com/go-chi/chi/v5"
)

/*
	File based operation handlers (should be moved to rest/handlers.go) or in
	a entirely self-contained package
*/

var (
	// The media extensions the player can open.
	//
	// The old pattern had ".mvk" — a typo for ".mkv" — so Matroska files were
	// not recognised as video and were given no length. It also matched "/gmi",
	// which matches no extension at all.
	videoRe = regexp.MustCompile(`(?i)\.(mkv|mov|mp4|m4v|webm|avi|flv|wmv|mpg|mpeg|ts|m2ts|ogv)`)
)

func isVideo(d fs.DirEntry) bool {
	return videoRe.MatchString(d.Name())
}

var audioRe = regexp.MustCompile(`(?i)\.mp3|\.m4a|\.opus|\.flac|\.wav|\.aac|\.ogg|\.weba`)

func isAudio(d fs.DirEntry) bool {
	return audioRe.MatchString(d.Name())
}

// isMedia reports whether a file is something a player could open, as opposed
// to a caption file or a leftover.
func isMedia(d fs.DirEntry) bool {
	return isVideo(d) || isAudio(d)
}

func isValidEntry(d fs.DirEntry) bool {
	return !strings.HasPrefix(d.Name(), ".") &&
		!strings.HasSuffix(d.Name(), ".part") &&
		!strings.HasSuffix(d.Name(), ".ytdl")
}

// isCaptionFile reports whether a file is a caption track rather than media.
//
// The caption files are listed as part of the video they belong to, not as
// entries of their own: they cannot be played, and the player offers them
// through the video's own track list.
func isCaptionFile(name string) bool {
	return captionExts[strings.ToLower(filepath.Ext(name))]
}

type DirectoryEntry struct {
	Name        string    `json:"name"`
	Path        string    `json:"path"`
	Size        int64     `json:"size"`
	ModTime     time.Time `json:"modTime"`
	IsVideo     bool      `json:"isVideo"`
	IsDirectory bool      `json:"isDirectory"`

	// Duration in seconds, for media files. Zero when it cannot be determined.
	Duration float64 `json:"duration,omitempty"`

	// Subtitles are the caption files sitting next to this one, matched by
	// language. A download with captions produces one file per language, named
	// after the video with the language code appended, and the browser has to
	// find them on disk rather than be told where they are.
	Subtitles []SubtitleFile `json:"subtitles,omitempty"`
}

// SubtitleFile is one caption track on disk.
type SubtitleFile struct {
	// Path is the file to load.
	Path string `json:"path"`
	// Language is the code parsed out of the filename, e.g. "en" or "zh-Hans".
	Language string `json:"language"`
	// Label is what the player shows.
	Label string `json:"label"`
}

func walkDir(root string) (*[]DirectoryEntry, error) {
	dirs, err := os.ReadDir(root)
	if err != nil {
		return nil, err
	}

	var files []DirectoryEntry

	for _, d := range dirs {
		if !isValidEntry(d) || isCaptionFile(d.Name()) {
			continue
		}

		path := filepath.Join(root, d.Name())

		info, err := d.Info()
		if err != nil {
			return nil, err
		}

		entry := DirectoryEntry{
			Path:        path,
			Name:        d.Name(),
			Size:        info.Size(),
			IsVideo:     isVideo(d),
			IsDirectory: d.IsDir(),
			ModTime:     info.ModTime(),
		}

		// Only media files carry a length and can have captions beside them.
		if !entry.IsDirectory && isMedia(d) {
			entry.Duration = mediaDuration(path)
			entry.Subtitles = subtitlesFor(root, d.Name())
		}

		files = append(files, entry)
	}

	return &files, err
}

type ListRequest struct {
	SubDir  string `json:"subdir"`
	OrderBy string `json:"orderBy"`
}

func ListDownloaded(w http.ResponseWriter, r *http.Request) {
	root := config.Instance().Paths.DownloadPath
	req := new(ListRequest)

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	files, err := walkDir(filepath.Join(root, req.SubDir))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if req.OrderBy == "modtime" {
		sort.SliceStable(*files, func(i, j int) bool {
			return (*files)[i].ModTime.After((*files)[j].ModTime)
		})
	}

	w.WriteHeader(http.StatusOK)

	if err := json.NewEncoder(w).Encode(files); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

type DeleteRequest = DirectoryEntry

func DeleteFile(w http.ResponseWriter, r *http.Request) {
	req := new(DeleteRequest)

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := os.Remove(req.Path); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode("ok")
}

func SendFile(w http.ResponseWriter, r *http.Request) {
	path := chi.URLParam(r, "id")

	if path == "" {
		http.Error(w, "inexistent path", http.StatusBadRequest)
		return
	}

	path, err := url.QueryUnescape(path)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	decoded, err := base64.StdEncoding.DecodeString(path)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	filename := string(decoded)

	root := config.Instance().Paths.DownloadPath

	if strings.Contains(filepath.Dir(filepath.Clean(filename)), filepath.Clean(root)) {
		http.ServeFile(w, r, filename)
		return
	}

	w.WriteHeader(http.StatusUnauthorized)
}

func DownloadFile(w http.ResponseWriter, r *http.Request) {
	path := chi.URLParam(r, "id")

	if path == "" {
		http.Error(w, "inexistent path", http.StatusBadRequest)
		return
	}

	path, err := url.QueryUnescape(path)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	decoded, err := base64.StdEncoding.DecodeString(path)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	filename := string(decoded)

	root := config.Instance().Paths.DownloadPath

	if strings.Contains(filepath.Dir(filepath.Clean(filename)), filepath.Clean(root)) {
		w.Header().Add("Content-Disposition", "inline; filename=\""+filepath.Base(filename)+"\"")
		w.Header().Set("Content-Type", "application/octet-stream")

		fd, err := os.Open(filename)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		io.Copy(w, fd)
		return
	}

	w.WriteHeader(http.StatusUnauthorized)
}

// SendSubtitle serves a caption file to the player.
//
// It goes through the same base64 path as the media files and the same
// containment check, so a caption file cannot be fetched from outside the
// download directory.
func SendSubtitle(w http.ResponseWriter, r *http.Request) {
	path := chi.URLParam(r, "id")

	if path == "" {
		http.Error(w, "inexistent path", http.StatusBadRequest)
		return
	}

	path, err := url.QueryUnescape(path)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	decoded, err := base64.StdEncoding.DecodeString(path)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	filename := string(decoded)

	root := config.Instance().Paths.DownloadPath

	if !strings.Contains(filepath.Dir(filepath.Clean(filename)), filepath.Clean(root)) {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	ext := strings.ToLower(filepath.Ext(filename))

	// The browser's <track> element reads WebVTT. A SubRip file served as
	// text/vtt is not parsed, so an SRT is converted on the way out. The
	// conversion is a matter of the comma before the timestamp and the
	// optional coordinate suffix on the last line of a cue.
	body, err := os.ReadFile(filename)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	switch ext {
	case ".vtt":
		w.Header().Set("Content-Type", "text/vtt; charset=utf-8")
		w.Write(body)
	case ".srt", ".ass", ".ssa":
		w.Header().Set("Content-Type", "text/vtt; charset=utf-8")
		w.Write([]byte(srtToVTT(string(body))))
	default:
		w.Header().Set("Content-Type", "application/octet-stream")
		w.Write(body)
	}
}

// srtToVTT converts SubRip to WebVTT.
//
// The formats differ in three ways: WebVTT wants a WEBVTT header, timestamps
// use a dot instead of a comma, and cue timings may carry coordinates that
// WebVTT does not allow.
func srtToVTT(srt string) string {
	var out strings.Builder

	out.WriteString("WEBVTT\n\n")

	lines := strings.Split(strings.ReplaceAll(srt, "\r\n", "\n"), "\n")

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)

		// A cue index: the next line is the timing.
		if i+1 < len(lines) && trimmed != "" && !strings.Contains(trimmed, "-->") {
			if strings.Contains(lines[i+1], "-->") {
				continue
			}
		}

		if strings.Contains(trimmed, "-->") {
			converted := convertTiming(trimmed)
			out.WriteString(converted)
			out.WriteString("\n")
			continue
		}

		out.WriteString(line)
		out.WriteString("\n")
	}

	return out.String()
}

// convertTiming rewrites one cue timing line.
func convertTiming(line string) string {
	parts := strings.Split(line, "-->")
	if len(parts) != 2 {
		return line
	}

	start := strings.TrimSpace(parts[0])
	end := strings.TrimSpace(parts[1])

	// The end may carry coordinates: "00:00:01,000 --> 00:00:02,000 X1:...".
	if idx := strings.IndexAny(end, " \t"); idx > 0 {
		end = end[:idx]
	}

	return strings.Replace(start, ",", ".", 1) + " --> " + strings.Replace(end, ",", ".", 1)
}

func BulkDownload(mdb *kv.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ps := slices.DeleteFunc(*mdb.All(), func(e internal.ProcessSnapshot) bool {
			return e.Progress.Status != internal.StatusCompleted
		})

		if len(ps) == 0 {
			return
		}

		zipWriter := zip.NewWriter(w)

		w.Header().Add(
			"Content-Disposition",
			"inline; filename=download-archive-"+time.Now().Format(time.RFC3339)+".zip",
		)
		w.Header().Set("Content-Type", "application/zip")

		for _, p := range ps {
			wr, err := zipWriter.Create(filepath.Base(p.Output.SavedFilePath))
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}

			fd, err := os.Open(p.Output.SavedFilePath)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}

			if _, err := io.Copy(wr, fd); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
		}

		if err := zipWriter.Close(); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}
}
