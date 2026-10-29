package filebrowser

import (
	"archive/zip"
	"encoding/base64"
	"encoding/json"
	"io"
	"io/fs"
	"log/slog"
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
	"github.com/LeisureLinux/happy-downloader/v4/server/internal/downloadlog"
	"github.com/LeisureLinux/happy-downloader/v4/server/internal/kv"
	middlewares "github.com/LeisureLinux/happy-downloader/v4/server/middleware"
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

// isValidEntry reports whether a file in the download directory is something
// the user might want to see.
//
// The partial files are excluded because a download in progress is not a
// finished file: yt-dlp writes ".part" while it downloads and ".ytdl" while
// it tracks the fragments of a segmented stream, and ".temp" while it merges
// and post-processes. Listing any of them makes a half-written file look
// like a download that had finished.
//
// They are kept out of the download directory in the first place now — the
// partial files live in their own directory — but a file from an older
// download, or one that was moved there by hand, is still filtered out.
func isValidEntry(d fs.DirEntry) bool {
	return !strings.HasPrefix(d.Name(), ".") &&
		!strings.HasSuffix(d.Name(), ".part") &&
		!strings.HasSuffix(d.Name(), ".ytdl") &&
		!strings.HasSuffix(d.Name(), ".temp")
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

	// The codecs of the file, as ffprobe names them: "av1", "h264", "opus",
	// "aac". The browser cannot decode a file it does not understand, and it
	// cannot tell from the name that it will not: a file named ".mp4" can hold
	// AV1, which is exactly what the downloader produces by default and what
	// a phone without an AV1 decoder cannot play. The player checks these
	// before it plays and says so, instead of showing a black rectangle.
	VideoCodec string `json:"videoCodec,omitempty"`
	AudioCodec string `json:"audioCodec,omitempty"`

	// Subtitles are the caption files sitting next to this one, matched by
	// language. A download with captions produces one file per language, named
	// after the video with the language code appended, and the browser has to
	// find them on disk rather than be told where they are.
	Subtitles []SubtitleFile `json:"subtitles,omitempty"`

	// SourceSite is the site the file came from, e.g. "youtube" or
	// "bilibili".
	//
	// The file name says nothing about it, and the answer is worth having:
	// a row of files is easier to read when each one says where it came
	// from, and it is what the icon in front of the name is for.
	SourceSite string `json:"sourceSite,omitempty"`

	// SourceURL is the page the file came from.
	SourceURL string `json:"sourceURL,omitempty"`

	// The figures of a folder, which are about the folder rather than about
	// a file: how many files it holds, when its newest file was written, and
	// how much disk it takes.
	//
	// They are empty for a file, which has its own.
	FileCount int `json:"fileCount,omitempty"`

	// FolderModTime is when the newest file in the folder was written.
	FolderModTime time.Time `json:"folderModTime,omitempty"`
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

	// An empty slice rather than a nil one: a nil slice is encoded as
	// "null", and the interface calls .map() on the answer. An empty folder
	// was a crash.
	files := []DirectoryEntry{}

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

		// A folder's figures are about its contents.
		if entry.IsDirectory {
			count, newest, size := folderFigures(path)
			entry.FileCount = count
			entry.FolderModTime = newest
			entry.Size = size
		}

		// Only media files carry a length and can have captions beside them.
		if !entry.IsDirectory && isMedia(d) {
			info := mediaInfoFor(path)
			entry.Duration = info.duration
			entry.VideoCodec = info.videoCodec
			entry.AudioCodec = info.audioCodec
			entry.Subtitles = subtitlesFor(root, d.Name())

			// Where it came from, so the row can say so.
			//
			// The index is the first choice, and the file's own metadata is
			// the fallback: a file that was moved before the index learned
			// to move has a record naming a path that is no longer there,
			// and the file itself still carries the URL yt-dlp wrote into it.
			//
			// The metadata has already been read for the length and the
			// codecs, so the URL is taken from there rather than from a
			// second ffprobe — spawning one for every file in the listing
			// is what made the file browser take fifteen seconds.
			url, site, ok := sourceOf(path, info)
			if ok {
				entry.SourceURL = url

				// A site that was recorded by hand wins over one derived
				// from the URL, because there may be no URL at all.
				if site != "" {
					entry.SourceSite = site
				} else {
					entry.SourceSite = SiteOf(url)
				}
			}
		}

		files = append(files, entry)
	}

	return &files, err
}

type ListRequest struct {
	SubDir  string `json:"subdir"`
	OrderBy string `json:"orderBy"`
}

// ListResponse is the answer to a listing.
//
// It is an object rather than a bare array because the interface needs to
// know where the root is: the "up" button goes to the parent directory, and
// the parent of a first-level folder is the download directory — which the
// interface cannot work out on its own, because it does not know where the
// download directory is. Without this the button climbed out of the download
// directory and one more press listed /home/axu/media.
type ListResponse struct {
	// Files are the entries of the directory.
	Files []DirectoryEntry `json:"files"`
	// Root is the download directory.
	Root string `json:"root"`
	// SubDir is the directory that was listed.
	SubDir string `json:"subDir"`
	// Parent is the directory the "up" button goes to, or empty when the
	// listing is already at the root.
	Parent string `json:"parent,omitempty"`
}

func ListDownloaded(w http.ResponseWriter, r *http.Request) {
	root := config.Instance().Paths.DownloadPath
	req := new(ListRequest)

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// The subdirectory may be given as a path relative to the download
	// directory or as an absolute one — the interface keeps the absolute
	// path of the folder it is showing, so that is what arrives.
	//
	// filepath.Join would have joined the two, and an absolute path in the
	// second argument is not what Join does with it: it produced
	// "<root>/home/axu/media/downloads/folder", which is nowhere.
	dir := root
	if req.SubDir != "" {
		dir = resolveSubDir(root, req.SubDir)

		// The root of the listing is the download directory. A path that
		// names it is the root, not a subdirectory of it.
		if filepath.Clean(dir) == filepath.Clean(root) {
			dir = root
		}
	}

	if !withinDownloadDir(filepath.Join(dir, "x")) {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	files, err := walkDir(dir)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if req.OrderBy == "modtime" {
		sort.SliceStable(*files, func(i, j int) bool {
			return (*files)[i].ModTime.After((*files)[j].ModTime)
		})
	}

	// The parent of the listing, which is empty at the root: the root has
	// no "up", and the interface hides the button rather than sending a
	// path that would climb out of the download directory.
	parent := ""
	if filepath.Clean(dir) != filepath.Clean(root) {
		parent = filepath.Dir(filepath.Clean(dir))
	}

	w.WriteHeader(http.StatusOK)

	if err := json.NewEncoder(w).Encode(ListResponse{
		Files:  *files,
		Root:   filepath.Clean(root),
		SubDir: filepath.Clean(dir),
		Parent: parent,
	}); err != nil {
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

	path := filepath.Clean(req.Path)

	if !withinDownloadDir(path) {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	// A folder is not deleted here. A folder with files in it has to have
	// them moved or deleted first, and an empty one is removed by the folder
	// endpoint — which is where the rule about non-empty folders lives.
	if info, err := os.Stat(path); err == nil && info.IsDir() {
		http.Error(w, "that is a folder; empty it first", http.StatusBadRequest)
		return
	}

	if err := os.Remove(path); err != nil {
		recordNote(r, downloadlog.KindDelete, path, "", err.Error(), false)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	recordNote(r, downloadlog.KindDelete, path, "", "", true)

	// The file is gone, so its source record has nothing to point at.
	forgetSource(path)

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
		// The content type is set before the file is served, because
		// http.ServeFile would otherwise work it out from the extension and
		// say "video/x-matroska" for a .mkv.
		//
		// Chromium does not treat Matroska as playable media: a .mkv
		// triggers a download rather than playing, even though the file
		// holds exactly the same streams as a .webm. The two containers
		// differ only in what they are allowed to contain, and a video
		// downloaded as one plays as the other.
		//
		// So a .mkv is served as video/webm. That is what makes the same
		// file play on a phone: the .webm next to it plays there, and the
		// only difference between them is the container.
		if ct := contentTypeFor(filename); ct != "" {
			w.Header().Set("Content-Type", ct)
		}

		http.ServeFile(w, r, filename)
		return
	}

	w.WriteHeader(http.StatusUnauthorized)
}

// contentTypeFor returns the content type a media file is served with.
//
// A Matroska file is served as video/webm, because Chromium plays that and
// not the other: "accessing .mkv files in Chromium triggers a download because
// the browser does not recognize the Matroska container format as playable
// media" — even though the streams inside are identical to a WebM's.
func contentTypeFor(path string) string {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".mkv":
		return "video/webm"
	case ".webm":
		return "video/webm"
	case ".mp4", ".m4v":
		return "video/mp4"
	case ".mov":
		return "video/quicktime"
	}

	return ""
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

	if !strings.Contains(filepath.Dir(filepath.Clean(filename)), filepath.Clean(root)) {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	// The file is sent as it is.
	//
	// It used to be wrapped in a zip with its captions, because a browser
	// would either block a second download or leave the captions in a
	// folder of their own. That was a poor answer on a phone: the video
	// arrived as a .zip that nothing could play, and unzipping it there is
	// a chore.
	//
	// The captions are separate downloads now, offered as separate buttons.
	// The interface asks /filebrowser/downloadable what belongs to the file
	// and shows one button per file.
	serveSingleFile(w, filename)
}

// serveSingleFile sends one file as it is.
func serveSingleFile(w http.ResponseWriter, filename string) {
	w.Header().Add("Content-Disposition", "inline; filename=\""+filepath.Base(filename)+"\"")
	w.Header().Set("Content-Type", "application/octet-stream")

	fd, err := os.Open(filename)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	defer fd.Close()

	if _, err := io.Copy(w, fd); err != nil {
		slog.Error("could not send the file", slog.String("path", filename), slog.Any("err", err))
	}
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

// sourceForgetter is told when a file is deleted, so the source index does
// not keep entries for files that are no longer there.
//
// A function rather than an interface so this package does not have to depend
// on the store.
var sourceForgetter func(path string)

// SetSourceForgetter installs the callback.
func SetSourceForgetter(fn func(path string)) {
	sourceForgetter = fn
}

// The lookup for a file's source.
//
// The index lives in another package, and this package only needs one thing
// from it: where the file came from. So it is a function rather than an
// interface, the same way the forgetter is.
var sourceLookup func(path string) (url, site string, ok bool)

// SetSourceLookup installs the callback.
//
// It answers with the site as well as the URL, because a file whose URL could
// not be recovered can still have its site known — and a site name is what the
// icon needs.
func SetSourceLookup(fn func(path string) (url, site string, ok bool)) {
	sourceLookup = fn
}

// The callback for a file that has moved.
//
// The index is keyed by the file's path, so a file that moves into a folder
// leaves its record behind — and that record is what says where the file came
// from. Without this, a file that had been moved showed no source at all.
var sourceMover func(from, to string)

// SetSourceMover installs the callback.
func SetSourceMover(fn func(from, to string)) {
	sourceMover = fn
}

// moveSource records that a file has moved.
func moveSource(from, to string) {
	if sourceMover != nil {
		sourceMover(from, to)
	}
}

// sourceOf finds where a file came from.
//
// The index is consulted first because it is free. The file's own metadata is
// the fallback, and it is what answers for the files that were moved before
// the index learned to move: their record names a path that is no longer
// there, and the file still carries the URL inside it.
//
// The metadata comes from the ffprobe run that read the length and the
// codecs, so it is free as well. It used to be a second ffprobe of its own,
// which cost a process spawn for every file in the listing.
func sourceOf(path string, info mediaInfo) (url, site string, ok bool) {
	if sourceLookup != nil {
		if url, site, ok := sourceLookup(path); ok {
			return url, site, true
		}
	}

	if info.source != "" {
		return info.source, "", true
	}

	return "", "", false
}

// recordNote writes one line to the operations log.
//
// What happens to a file after it is downloaded is the rest of the same
// question: a video that is not in the folder it was put in was moved by
// somebody, and the log is where that is written down. The address is taken
// from the request, never from the body.
func recordNote(r *http.Request, kind, path, to, reason string, ok bool) {
	status := downloadlog.StatusSuccess
	if !ok {
		status = downloadlog.StatusFailed
	}

	fromIP := ""
	if r != nil {
		fromIP = middlewares.ClientIP(r)
	}

	downloadlog.Record(downloadlog.Outcome{
		Kind:   kind,
		Status: status,
		FromIP: fromIP,
		Path:   path,
		To:     to,
		Reason: reason,
	})
}

func forgetSource(path string) {
	if sourceForgetter != nil {
		sourceForgetter(path)
	}
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
			add := func(path string) error {
				wr, err := zipWriter.Create(filepath.Base(path))
				if err != nil {
					return err
				}

				fd, err := os.Open(path)
				if err != nil {
					return err
				}

				defer fd.Close()

				_, err = io.Copy(wr, fd)

				return err
			}

			if err := add(p.Output.SavedFilePath); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}

			// The captions travel with the video, as they do for a single
			// download.
			for _, c := range subtitlesFor(
				filepath.Dir(filepath.Clean(p.Output.SavedFilePath)),
				filepath.Base(p.Output.SavedFilePath),
			) {
				if err := add(c.Path); err != nil {
					slog.Error("could not add a caption file to the archive",
						slog.String("path", c.Path), slog.Any("err", err))
				}
			}
		}

		if err := zipWriter.Close(); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}
}
