package downloaders

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"regexp"
	"strconv"
	"strings"
	"sync"

	"github.com/LeisureLinux/happy-downloader/v4/server/internal"
)

type LogConsumer interface {
	GetName() string
	ParseLogEntry(entry []byte, downloader Downloader)
}

type JSONLogConsumer struct{}

func NewJSONLogConsumer() LogConsumer {
	return &JSONLogConsumer{}
}

func (j *JSONLogConsumer) GetName() string { return "json-log-consumer" }

// yt-dlp renders a missing numeric field as a bare NA, which is not valid
// JSON — every audio download failed to parse because width and height do not
// exist, and the reported file path and size were lost with it. Rewriting the
// bare tokens to null keeps the template numeric while surviving the missing
// case. The anchor on the colon keeps a quoted "NA" untouched.
var bareNA = regexp.MustCompile(`:NA([,}\]])`)

func sanitizeProgressJSON(entry []byte) []byte {
	return bareNA.ReplaceAll(entry, []byte(":null$1"))
}

func (j *JSONLogConsumer) ParseLogEntry(entry []byte, d Downloader) {
	sanitized := sanitizeProgressJSON(entry)

	// Both templates are unmarshalled into both structs. A progress line has
	// no `filepath`, so it used to overwrite the saved path with "" — and a
	// postprocess line has no `percentage`, so it used to leave the status
	// alone. Decide which line this is before touching any state.
	if bytes.Contains(sanitized, []byte(`"percentage"`)) {
		var progress internal.ProgressTemplate
		if err := json.Unmarshal(sanitized, &progress); err == nil {
			j.parseProgress(progress, d)
		}
		return
	}

	if bytes.Contains(sanitized, []byte(`"filepath"`)) {
		var postprocess internal.PostprocessTemplate
		if err := json.Unmarshal(sanitized, &postprocess); err == nil {
			j.parsePostprocess(postprocess, d)
		}
	}
}

func (j *JSONLogConsumer) parseProgress(progress internal.ProgressTemplate, d Downloader) {
	if progress.Percentage == "" {
		return
	}

	// A download that was paused keeps its status. yt-dlp's last progress
	// line arrives after the kill, and it said "downloading", which turned
	// the pause back into a download — the row showed a running bar for
	// something that was stopped.
	status := internal.StatusDownloading
	previous := d.Status().Progress.Status
	switch previous {
	case internal.StatusPaused:
		status = internal.StatusPaused
	case internal.StatusPostprocessing:
		// The bar reads 100% from here on, but the file is still being
		// merged, embedded and moved. Keep saying so.
		status = internal.StatusPostprocessing
	default:
		// 100% is the end of the transfer, not the end of the download.
		if isCompletePercentage(progress.Percentage) {
			status = internal.StatusPostprocessing
		}
	}

	d.SetProgress(internal.DownloadProgress{
		Status:     status,
		Percentage: progress.Percentage,
		Speed:      progress.Speed,
		ETA:        progress.Eta,
	})

	// One line per percentage point is enough. yt-dlp emits several per
	// fragment on a slow HLS download, and the SSE history (200 lines) was
	// filled with six copies of "94.5%" while the useful error scrolled out.
	if progress.Percentage != lastLoggedPercentage(d.GetId()) {
		slog.Info("progress",
			slog.String("id", j.GetShortId(d.GetId())),
			slog.String("url", d.GetUrl()),
			slog.String("percentage", progress.Percentage),
		)
		noteLoggedPercentage(d.GetId(), progress.Percentage)
	}
}

func (j *JSONLogConsumer) parsePostprocess(p internal.PostprocessTemplate, d Downloader) {
	final := finalOutputPath(p.FilePath, p.FinalDir)
	if final == "" {
		return
	}

	// Remember the final path from the first postprocess line onward. The
	// file is not there yet, but the row no longer points at a temp file
	// that is guaranteed to disappear.
	d.UpdateSavedFilePath(final)

	if p.Width > 0 && p.Height > 0 {
		d.UpdateOutputDimensions(p.Width, p.Height)
	}

	if d.Status().Progress.Status != internal.StatusPaused {
		d.SetProgress(internal.DownloadProgress{
			Status:     internal.StatusPostprocessing,
			Percentage: "100%",
		})
	}

	// Do not record the source here. These lines are emitted while the file
	// is still in the temp directory; the source index is keyed by the path
	// on disk and would remember a file that is about to be moved away.
	// GenericDownloader.Start records it once, after the process has exited.
}

// isCompletePercentage reports whether yt-dlp has transferred the last byte.
// The value is a formatted string like " 99.9%" or "100.0%".
func isCompletePercentage(s string) bool {
	f, err := strconv.ParseFloat(strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(s), "%")), 64)
	return err == nil && f >= 100
}

// lastLoggedPercentage keeps the last value written to the log per download,
// so progress noise does not evict real errors from the in-memory history.
var (
	loggedPctMu sync.Mutex
	loggedPct   = map[string]string{}
)

func lastLoggedPercentage(id string) (previous string) {
	loggedPctMu.Lock()
	defer loggedPctMu.Unlock()
	return loggedPct[id]
}

// noteLoggedPercentage is called by parseProgress after a line was emitted.
func noteLoggedPercentage(id, percentage string) {
	loggedPctMu.Lock()
	defer loggedPctMu.Unlock()

	loggedPct[id] = percentage
	// One entry per finished download, not one per download ever seen.
	if len(loggedPct) > 256 {
		clear(loggedPct)
	}
}

func (j *JSONLogConsumer) GetShortId(id string) string {
	return strings.Split(id, "-")[0]
}

// sourceRecorder is notified when a file is written, so the source index can
// remember where the file came from.
//
// It is a function rather than an interface so that this package does not
// depend on the store: the server sets it at startup, and the tests that do
// not care about it leave it unset.
var sourceRecorder func(d Downloader, path string)

// SetSourceRecorder installs the callback that records where a file came from.
func SetSourceRecorder(fn func(d Downloader, path string)) {
	sourceRecorder = fn
}

// RememberSource records the file's source, if anything is listening.
//
// It is exported because a livestream recording is not written by yt-dlp's
// own postprocessing — it is written from the media stream — so nothing else
// records where it came from. A recording with no source showed the globe
// icon in the file browser instead of the site's.
func RememberSource(d Downloader, path string) {
	if sourceRecorder == nil || path == "" {
		return
	}

	sourceRecorder(d, path)
}

// finishedRecorder is notified when a recording is over.
//
// It is a function rather than an interface so that this package does not
// depend on the store: the server sets it at startup, and the tests that do
// not care about it leave it unset.
var finishedRecorder func(d Downloader)

// SetFinishedRecorder installs the callback that is told when a recording
// ends.
//
// A recording is the only download that leaves the queue by itself. Every
// other download stays in the list until the file browser has it. A
// recording used to stay for ever, because nothing announced that it was
// over: it was written from the media stream rather than by yt-dlp's own
// postprocessing, so the hook that would have said so was never called.
func SetFinishedRecorder(fn func(d Downloader)) {
	finishedRecorder = fn
}

// signalFinished tells whoever is listening that a recording is over.
func signalFinished(d Downloader) {
	if finishedRecorder == nil {
		return
	}

	finishedRecorder(d)
}
