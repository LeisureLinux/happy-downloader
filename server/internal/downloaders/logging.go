package downloaders

import (
	"encoding/json"
	"log/slog"
	"regexp"
	"strings"

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
	var progress internal.ProgressTemplate
	var postprocess internal.PostprocessTemplate

	if err := json.Unmarshal(sanitizeProgressJSON(entry), &progress); err == nil {
		if progress.Percentage != "" {
			// A download that was paused keeps its status. yt-dlp's last
			// progress line arrives after the kill, and it said
			// "downloading", which turned the pause back into a download —
			// the row showed a running bar for something that was stopped.
			status := internal.StatusDownloading
			if d.Status().Progress.Status == internal.StatusPaused {
				status = internal.StatusPaused
			}

			d.SetProgress(internal.DownloadProgress{
				Status:     status,
				Percentage: progress.Percentage,
				Speed:      progress.Speed,
				ETA:        progress.Eta,
			})

			slog.Info("progress",
				slog.String("id", j.GetShortId(d.GetId())),
				slog.String("url", d.GetUrl()),
				slog.String("percentage", progress.Percentage),
			)
		}
	}

	if err := json.Unmarshal(sanitizeProgressJSON(entry), &postprocess); err == nil {
		d.UpdateSavedFilePath(postprocess.FilePath)
		rememberSource(d, postprocess.FilePath)
		if postprocess.Width > 0 && postprocess.Height > 0 {
			d.UpdateOutputDimensions(postprocess.Width, postprocess.Height)
		}
	}
}

func (j *JSONLogConsumer) GetShortId(id string) string {
	return strings.Split(id, "-")[0]
}

//TODO: split in different files

type FFMpegLogConsumer struct{}

func NewFFMpegLogConsumer() LogConsumer {
	return &JSONLogConsumer{}
}

func (f *FFMpegLogConsumer) GetName() string { return "ffmpeg-log-consumer" }

func (f *FFMpegLogConsumer) ParseLogEntry(entry []byte, d Downloader) {
	slog.Info("ffmpeg output",
		slog.String("id", d.GetId()),
		slog.String("url", d.GetUrl()),
		slog.String("output", string(entry)),
	)
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

// rememberSource records the file's source, if anything is listening.
func rememberSource(d Downloader, path string) {
	if sourceRecorder == nil || path == "" {
		return
	}

	sourceRecorder(d, path)
}
