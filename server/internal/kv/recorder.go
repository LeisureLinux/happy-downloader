package kv

import (
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/LeisureLinux/happy-downloader/v4/server/internal/downloaders"
)

// SourceRecorder builds the index entry for a file that was just written.
//
// It lives here rather than in the server so that the index's shape is
// decided in one place: what is remembered about a file, and how it is
// derived from a download.
type SourceRecorder struct {
	Index *SourceIndex
}

// Record remembers where a file came from.
//
// The download knows its URL and the caption languages the source offered;
// the postprocessing event knows the path that was written. Together they
// are what the file browser needs later, when the user asks for captions for
// a download that finished an hour ago.
func (r *SourceRecorder) Record(d downloaders.Downloader, path string) {
	if r == nil || r.Index == nil {
		return
	}

	if path == "" {
		return
	}

	// The event reports the path before the extension is known for merged
	// files, so the one on disk is the truth.
	if info, err := os.Stat(path); err == nil && !info.IsDir() {
		path = filepath.Clean(path)
	} else {
		return
	}

	snapshot := d.Status()

	rec := SourceRecord{
		Path:    path,
		URL:     snapshot.URL,
		Title:   snapshot.Info.Title,
		SavedAt: time.Now().Unix(),
	}

	for _, c := range snapshot.Captions {
		rec.Captions = append(rec.Captions, SourceCaption{
			Language:  c.Language,
			Name:      c.Name,
			Automatic: c.Automatic,
		})
	}

	if rec.URL == "" {
		return
	}

	r.Index.Remember(rec)

	slog.Info("source recorded",
		slog.String("path", path),
		slog.Int("captions", len(rec.Captions)),
	)
}
