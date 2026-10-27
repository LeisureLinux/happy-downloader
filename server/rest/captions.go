package rest

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"

	"github.com/LeisureLinux/happy-downloader/v4/server/config"
	"github.com/LeisureLinux/happy-downloader/v4/server/internal"
	"github.com/LeisureLinux/happy-downloader/v4/server/internal/downloaders"
	"github.com/LeisureLinux/happy-downloader/v4/server/internal/kv"
)

// Captions for a file that was already downloaded.
//
// Downloading captions is a separate request from downloading the video, and it
// has to be: YouTube rate-limits the caption endpoint far harder than it
// rate-limits video, and a throttled caption request that aborts the whole
// job throws away a video that already succeeded. So the video is fetched
// without captions and the captions are fetched afterwards, on demand, per
// language.
//
// The languages offered are the ones the source actually has. The server
// knows them from the probe, and the ones with no caption are simply not
// offered.

// CaptionRequest asks for the captions of an already-downloaded file.
type CaptionRequest struct {
	// Path is the file on disk whose captions are wanted.
	Path string `json:"path"`
	// Languages are the caption codes to fetch, e.g. "en" and "zh-Hans".
	Languages []string `json:"languages"`
}

// CaptionResult reports what was fetched.
type CaptionResult struct {
	// Path is the file the captions belong to.
	Path string `json:"path"`
	// URL is the source the captions came from.
	URL string `json:"url"`
	// Written lists the caption files that were produced.
	Written []string `json:"written"`
	// Skipped lists the languages that were not fetched, with the reason.
	Skipped map[string]string `json:"skipped,omitempty"`
}

// captionLocks serialises caption downloads per file. Two requests for the
// same file would otherwise write the same file at once.
var captionLocks sync.Map

// CaptionsHandler serves the caption download endpoint.
type CaptionsHandler struct {
	index *kv.SourceIndex
}

func NewCaptionsHandler(index *kv.SourceIndex) *CaptionsHandler {
	return &CaptionsHandler{index: index}
}

// Download fetches captions for a file that is already on disk.
func (h *CaptionsHandler) Download(w http.ResponseWriter, r *http.Request) {
	var req CaptionRequest

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if req.Path == "" || len(req.Languages) == 0 {
		http.Error(w, "a path and at least one language are required", http.StatusBadRequest)
		return
	}

	result, err := h.fetch(req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	if err := json.NewEncoder(w).Encode(result); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

// fetch does the work, and is separate so it can be tested without a server.
func (h *CaptionsHandler) fetch(req CaptionRequest) (*CaptionResult, error) {
	root := config.Instance().Paths.DownloadPath

	clean := filepath.Clean(req.Path)
	if !strings.Contains(filepath.Dir(clean), filepath.Clean(root)) {
		return nil, errors.New("the path is outside the download directory")
	}

	if _, err := os.Stat(clean); err != nil {
		return nil, fmt.Errorf("the file is not there: %w", err)
	}

	url, ok := h.sourceURL(clean)
	if !ok {
		return nil, errors.New("this file has no remembered source, so its captions cannot be found")
	}

	lock, _ := captionLocks.LoadOrStore(clean, &sync.Mutex{})
	mu := lock.(*sync.Mutex)
	mu.Lock()
	defer mu.Unlock()

	result := &CaptionResult{
		Path:    clean,
		URL:     url,
		Skipped: map[string]string{},
	}

	// The caption file has to sit next to the video and share its name, or
	// the file browser will not find it and the player will not offer it.
	stem := strings.TrimSuffix(clean, filepath.Ext(clean))

	args := []string{
		"--skip-download",
		"--write-subs",
		"--convert-subs", "srt",
		"--sub-langs", strings.Join(req.Languages, ","),
		"-o", stem + ".%(ext)s",
	}

	// Same conditions as the video download: the same cookies, the same
	// proxy, the same PO Token. A caption request that resolves under
	// different conditions than the video gets a guest session.
	argv := downloaders.BuildArgs(url, args...)

	cmd := downloaders.NewCommand(argv)

	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("%w: %s", err, strings.TrimSpace(string(out)))
	}

	for _, lang := range req.Languages {
		candidates := []string{
			stem + "." + lang + ".srt",
			stem + "." + lang + ".vtt",
		}

		found := ""
		for _, c := range candidates {
			if _, err := os.Stat(c); err == nil {
				found = c
				break
			}
		}

		if found == "" {
			result.Skipped[lang] = "the source did not offer this language"
			continue
		}

		result.Written = append(result.Written, found)
	}

	slog.Info("captions fetched",
		slog.String("path", clean),
		slog.Int("written", len(result.Written)),
		slog.Int("skipped", len(result.Skipped)),
	)

	return result, nil
}

// sourceURL finds the URL a downloaded file came from.
//
// The file browser works in paths and the index works in URLs, so the two are
// joined here. A file that was put there by something else has no record, and
// its captions cannot be found — which is worth saying plainly rather than
// failing with something cryptic.
func (h *CaptionsHandler) sourceURL(path string) (string, bool) {
	if h.index == nil {
		return "", false
	}

	rec, ok := h.index.Lookup(path)
	if !ok {
		return "", false
	}

	return rec.URL, rec.URL != ""
}

// AvailableCaptions lists the caption languages a file's source offered.
//
// It comes from the probe, which is kept for ten minutes after a download, so
// this is only reliable for recent files. For older ones the languages are
// found by asking yt-dlp, which is slower but always right.
func (h *CaptionsHandler) Available(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Query().Get("path")
	if path == "" {
		http.Error(w, "a path is required", http.StatusBadRequest)
		return
	}

	clean := filepath.Clean(path)

	url, ok := h.sourceURL(clean)
	if !ok {
		http.Error(w, "this file has no remembered source", http.StatusNotFound)
		return
	}

	languages, err := probeCaptionLanguages(url)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	if err := json.NewEncoder(w).Encode(languages); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

// probeCaptionLanguages asks yt-dlp which caption languages a source has.
func probeCaptionLanguages(url string) ([]internal.CaptionTrack, error) {
	argv := downloaders.BuildArgs(url, "-J")

	cmd := downloaders.NewCommand(argv)

	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	var parsed struct {
		Subtitles         map[string]any `json:"subtitles"`
		AutomaticCaptions map[string]any `json:"automatic_captions"`
	}

	if err := json.Unmarshal(out, &parsed); err != nil {
		return nil, err
	}

	return captionTracks(parsed.Subtitles, parsed.AutomaticCaptions), nil
}

// captionName pulls the human-readable name out of yt-dlp's caption data.
//
// The value for a language is a list of formats, and the first one carries the
// name — "English (auto-generated)" and the like. Without it the list shows
// 165 bare codes, which is not a list anyone can choose from.
func captionName(value any) string {
	list, ok := value.([]any)
	if !ok {
		return ""
	}

	// The name is not on every entry: a hand-written track has it on the
	// second format while the first carries only a URL. So the list is
	// searched rather than taking the first entry.
	for _, entry := range list {
		item, ok := entry.(map[string]any)
		if !ok {
			continue
		}

		if name, ok := item["name"].(string); ok && name != "" {
			return name
		}
	}

	return ""
}

// captionTracks flattens yt-dlp's caption maps into a list.
func captionTracks(subtitles, automatic map[string]any) []internal.CaptionTrack {
	var tracks []internal.CaptionTrack

	add := func(source map[string]any, auto bool) {
		for lang, value := range source {
			name := captionName(value)
			if name == "" {
				name = lang
			}

			tracks = append(tracks, internal.CaptionTrack{
				Language:  lang,
				Name:      name,
				Automatic: auto,
			})
		}
	}

	add(subtitles, false)
	add(automatic, true)

	slices.SortFunc(tracks, func(a, b internal.CaptionTrack) int {
		if a.Automatic != b.Automatic {
			if a.Automatic {
				return 1
			}
			return -1
		}
		return strings.Compare(a.Language, b.Language)
	})

	return tracks
}
