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
	"time"

	"github.com/LeisureLinux/happy-downloader/v4/server/config"
	"github.com/LeisureLinux/happy-downloader/v4/server/filebrowser"
	"github.com/LeisureLinux/happy-downloader/v4/server/internal"
	"github.com/LeisureLinux/happy-downloader/v4/server/internal/cookies"
	"github.com/LeisureLinux/happy-downloader/v4/server/internal/downloaders"
	"github.com/LeisureLinux/happy-downloader/v4/server/internal/kv"
	"github.com/go-chi/chi/v5"
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

// A caption download takes twenty seconds or more: yt-dlp has to resolve the
// source before it can ask for a caption. So it runs in the background and the
// browser asks how it is going.
//
// Running it in the foreground meant that closing the dialog — or clicking
// anywhere else, or losing the network for a moment — left the request
// cancelled and the captions half-written. The work does not belong to the
// page that asked for it.

// CaptionJob is one caption download in progress.
type CaptionJob struct {
	ID      string            `json:"id"`
	Path    string            `json:"path"`
	State   string            `json:"state"`
	Written []string          `json:"written,omitempty"`
	Skipped map[string]string `json:"skipped,omitempty"`
	Error   string            `json:"error,omitempty"`

	started  time.Time
	finished time.Time
}

// The states a job goes through.
const (
	jobRunning   = "running"
	jobSucceeded = "succeeded"
	jobFailed    = "failed"
)

// captionLocks serialises caption downloads per file. Two requests for the
// same file would otherwise write the same file at once.
var captionLocks sync.Map

// captionJobs holds the jobs that are running or have just finished.
type captionJobs struct {
	mu   sync.RWMutex
	jobs map[string]*CaptionJob
}

func newCaptionJobs() *captionJobs {
	return &captionJobs{jobs: map[string]*CaptionJob{}}
}

// put stores a job.
func (j *captionJobs) put(job *CaptionJob) {
	j.mu.Lock()
	j.jobs[job.ID] = job
	j.mu.Unlock()
}

// get returns a job.
func (j *captionJobs) get(id string) (*CaptionJob, bool) {
	j.mu.RLock()
	job, ok := j.jobs[id]
	j.mu.RUnlock()
	return job, ok
}

// sweep removes the jobs that finished long ago, so the map does not grow
// without bound.
func (j *captionJobs) sweep() {
	j.mu.Lock()
	defer j.mu.Unlock()

	for id, job := range j.jobs {
		if !job.finished.IsZero() && time.Since(job.finished) > 10*time.Minute {
			delete(j.jobs, id)
		}
	}
}

// CaptionsHandler serves the caption endpoints.
type CaptionsHandler struct {
	index *kv.SourceIndex
	jobs  *captionJobs
}

func NewCaptionsHandler(index *kv.SourceIndex) *CaptionsHandler {
	return &CaptionsHandler{
		index: index,
		jobs:  newCaptionJobs(),
	}
}

// Download starts a caption download and returns immediately.
//
// The job runs on its own. The browser can close the dialog, navigate away
// or lose the connection and the captions still arrive.
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

	job := &CaptionJob{
		ID:      newJobID(),
		Path:    req.Path,
		State:   jobRunning,
		Skipped: map[string]string{},
		started: time.Now(),
	}

	h.jobs.put(job)
	h.jobs.sweep()

	go h.run(job, req)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)

	if err := json.NewEncoder(w).Encode(job); err != nil {
		slog.Error("could not report the caption job", slog.Any("err", err))
	}
}

// Status reports how a job is going.
func (h *CaptionsHandler) Status(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	job, ok := h.jobs.get(id)
	if !ok {
		http.Error(w, "no such job", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	if err := json.NewEncoder(w).Encode(job); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

// run does the work of one job.
func (h *CaptionsHandler) run(job *CaptionJob, req CaptionRequest) {
	result, err := h.fetch(req)

	h.jobs.mu.Lock()
	defer h.jobs.mu.Unlock()

	job.finished = time.Now()

	if err != nil {
		job.State = jobFailed
		job.Error = err.Error()

		slog.Error("could not fetch the captions",
			slog.String("path", req.Path),
			slog.Any("languages", req.Languages),
			slog.Any("err", err),
		)

		return
	}

	job.State = jobSucceeded
	job.Written = result.Written
	job.Skipped = result.Skipped

	slog.Info("captions fetched",
		slog.String("path", req.Path),
		slog.Int("written", len(result.Written)),
		slog.Int("skipped", len(result.Skipped)),
	)
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
		// Both kinds of caption. --write-subs alone gets only the authored
		// tracks, and most videos have none of those: YouTube offers a
		// generated track for nearly every video, and often for a hundred
		// languages. Asking for only the authored ones produced "there are no
		// subtitles for the requested languages" on a video that plainly had
		// them.
		"--write-subs",
		"--write-auto-subs",
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
		// A refused credential is worth more than an error message: it is
		// the signal that the cookie the user imported is not working.
		//
		// The caption fetch is where an expired cookie shows up first —
		// YouTube rate-limits and invalidates the caption endpoint before
		// it refuses a video — so a failure here was never counted towards
		// the two refusals that replace the import.
		if cookies.LooksRefused(string(out)) {
			cookies.ReportRefusal()
		}

		return nil, fmt.Errorf("%w: %s", err, strings.TrimSpace(string(out)))
	}

	// A warning rather than an error is still a refusal. yt-dlp reports the
	// rotated cookie as a warning and carries on with a guest session, so
	// the command succeeds and the captions are simply not there.
	if cookies.LooksRefused(string(out)) {
		cookies.ReportRefusal()
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

	return result, nil
}

// sourceURL finds the URL a downloaded file came from.
//
// The file browser works in paths and the index works in URLs, so the two are
// joined here. A file that was put there by something else has no record, and
// its captions cannot be found — which is worth saying plainly rather than
// failing with something cryptic.
func (h *CaptionsHandler) sourceURL(path string) (string, bool) {
	if h.index != nil {
		rec, ok := h.index.Lookup(path)
		if ok && rec.URL != "" {
			return rec.URL, true
		}
	}

	// A file downloaded before the index existed, or one put there by hand
	// with the same tool, still carries its source in the metadata:
	// yt-dlp's --embed-metadata writes the page URL into the comment field.
	// Reading it means the old files work too, rather than only the ones
	// fetched since.
	if url, ok := filebrowser.SourceFromMetadata(path); ok {
		return url, true
	}

	return "", false
}

// AvailableCaptions lists the caption languages a file's source offered.
func (h *CaptionsHandler) Available(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Query().Get("path")
	if path == "" {
		http.Error(w, "a path is required", http.StatusBadRequest)
		return
	}

	clean := filepath.Clean(path)

	url, ok := h.sourceURL(clean)
	if !ok {
		slog.Warn("no source for this file", slog.String("path", clean))

		http.Error(w, "this file has no remembered source", http.StatusNotFound)
		return
	}

	languages, err := probeCaptionLanguages(url)
	if err != nil {
		// The probe takes twenty seconds and fails for reasons the user
		// cannot see from a 500. The message goes to the log, where it is
		// readable, and a short one goes to the browser.
		slog.Error("could not read the caption languages",
			slog.String("path", clean),
			slog.String("url", url),
			slog.Any("err", err),
		)

		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	slog.Info("caption languages listed",
		slog.String("path", clean),
		slog.Int("count", len(languages)),
	)

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

	// The output is captured rather than streamed, so the refusal has to be
	// recognised from it afterwards. This probe is the first thing that
	// fails when a cookie has expired — it is what runs when the caption
	// dialog opens — so a refusal here has to count.
	out, err := cmd.CombinedOutput()
	if err != nil {
		if cookies.LooksRefused(string(out)) {
			cookies.ReportRefusal()
		}

		return nil, err
	}

	// A warning rather than an error is still a refusal: yt-dlp carries on
	// with a guest session, and the languages it reports are the guest's.
	if cookies.LooksRefused(string(out)) {
		cookies.ReportRefusal()
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

// captionName pulls the human-readable name out of yt-dlp's caption data.
//
// The value for a language is a list of formats, and the name is on one of
// them — not always the first. Without it the list shows 165 bare codes,
// which is not a list anyone can choose from.
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

// newJobID makes an identifier for a job.
func newJobID() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())
}
