package downloaders

import (
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/LeisureLinux/happy-downloader/v4/server/config"
)

// RecoverPartialRecordings finishes the recordings that were left behind.
//
// A recording is written into the partial directory and moved into the
// download directory when it is put into its container. If the process stops
// between those two — a crash, a restart, the machine going down — the
// recording is left there: everything up to the last segment, invisible to the
// file browser, and cleaned up with the rest of the temporary files the next
// time anybody looks.
//
// That is somebody's recording. It is finished here: remuxed into the download
// directory, or moved there as it is when ffmpeg cannot read it.
//
// It runs on startup because that is the only moment the partial directory can
// be known to hold nothing in use: no download has started yet.
func RecoverPartialRecordings() {
	dir := tempDir()

	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}

	home := config.Instance().Paths.DownloadPath
	if home == "" {
		return
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()
		if !strings.HasSuffix(strings.ToLower(name), ".ts") {
			continue
		}

		raw := filepath.Join(dir, name)

		info, err := entry.Info()
		if err != nil || info.Size() == 0 {
			// Nothing was recorded: the file is an artefact of a recording
			// that never got a byte, and there is nothing to keep.
			if err == nil {
				os.Remove(raw)
			}
			continue
		}

		final := filepath.Join(home, strings.TrimSuffix(name, ".ts")+".mp4")

		kept := final
		if err := remuxRecording(raw, final); err != nil {
			slog.Warn("could not put a recovered recording into its container, keeping the raw file",
				slog.String("file", name),
				slog.String("err", err.Error()),
			)

			kept = filepath.Join(home, name)
			if err := os.Rename(raw, kept); err != nil {
				slog.Error("could not keep a recovered recording",
					slog.String("file", name),
					slog.String("err", err.Error()),
				)
				continue
			}
		} else {
			os.Remove(raw)
		}

		slog.Info("recovered a recording that was left unfinished",
			slog.String("from", name),
			slog.String("to", kept),
			slog.Int64("bytes", info.Size()),
		)
	}
}
