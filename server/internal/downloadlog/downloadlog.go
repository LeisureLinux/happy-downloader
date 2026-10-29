// Package downloadlog keeps a log of downloads: one line per finished
// download, and nothing else in it.
//
// The application's own log is for reading while something is happening. It
// carries every HTTP request, every line yt-dlp writes, every probe and every
// progress tick — which is what makes it useful then, and useless afterwards.
// Answering "did that download work, and if not why" means reading a thousand
// lines of something else.
//
// So this log is separate, and it is written once per download, at the moment
// the download is over. It answers four questions: did it work, what was the
// URL, where did the file go, and if it failed, why.
//
// The shape is a line of tab-separated columns, the way an access log is: it
// can be read with tail, searched with grep and cut up with awk, and a
// filename full of spaces does not break it. Tabs rather than spaces for
// exactly that reason — the filenames this writes are video titles, and video
// titles contain spaces.
package downloadlog

import (
	"encoding/csv"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/LeisureLinux/happy-downloader/v4/server/config"
)

// The columns, in order.
//
// The header is written when the file is created, and it is the only line
// that starts with a '#', so a reader can skip it with grep -v '^#'.
var headerFields = []string{
	"finished_at", "status", "from_ip", "url", "path", "size_bytes", "elapsed_s", "downloader", "reason",
}

// Outcome is one finished download.
type Outcome struct {
	// Status is "success" or "failed".
	Status string
	// FromIP is the machine that asked for the download, as the server saw
	// it. It is empty for a download whose address was not recorded.
	FromIP string
	// URL is what was asked for.
	URL string
	// Path is the file that was written, empty when nothing was.
	Path string
	// Size is the bytes on disk, zero when unknown.
	Size int64
	// Elapsed is how long the download took.
	Elapsed time.Duration
	// Downloader is which stage produced it: "generic" or "livestream".
	Downloader string
	// Reason is why it failed. Empty for a success.
	Reason string
}

const (
	StatusSuccess = "success"
	StatusFailed  = "failed"
)

// mu serialises the writes. The queue runs several downloads at once, and two
// processes appending to the same file without a lock interleave.
var mu sync.Mutex

// Record appends one line.
//
// A failure to write is reported and then ignored: the download has already
// happened, and losing a line of the log is not a reason to fail it.
func Record(o Outcome) {
	mu.Lock()
	defer mu.Unlock()

	path := Path()
	if path == "" {
		return
	}

	if err := appendFields(path, fields(o)); err != nil {
		slog.Error("could not write the download log",
			slog.String("path", path),
			slog.String("err", err.Error()),
		)
	}
}

// Path is where the log is written, or empty when there is no state directory
// to put it in.
//
// It follows the state directory rather than the download directory: the file
// browser lists the download directory, and a log in it would be offered for
// download next to the videos.
func Path() string {
	if p := config.Instance().Logging.DownloadLogPath; p != "" {
		return p
	}

	dir := config.Instance().Paths.LocalDatabasePath
	if dir == "" {
		return ""
	}

	return filepath.Join(dir, "downloads.log")
}

// fields turns an outcome into the columns.
func fields(o Outcome) []string {
	status := o.Status
	if status == "" {
		status = StatusFailed
	}

	size := "-"
	if o.Size > 0 {
		size = strconv.FormatInt(o.Size, 10)
	}

	elapsed := "-"
	if o.Elapsed > 0 {
		elapsed = fmt.Sprintf("%.1f", o.Elapsed.Seconds())
	}

	return []string{
		time.Now().Format(time.RFC3339),
		status,
		sanitize(o.FromIP),
		sanitize(o.URL),
		sanitize(o.Path),
		size,
		elapsed,
		sanitize(o.Downloader),
		sanitize(o.Reason),
	}
}

// sanitize makes a value safe to put in a column.
//
// A tab would add a column, and a newline would add a line — and a failure
// reason comes from yt-dlp, which writes whatever it likes, including both.
// An empty value becomes "-" so that a column is never visually missing.
func sanitize(v string) string {
	replacer := strings.NewReplacer("\t", " ", "\n", " ", "\r", " ")
	v = strings.TrimSpace(replacer.Replace(strings.TrimSpace(v)))

	if v == "" {
		return "-"
	}

	return v
}

// appendFields writes one line, adding the header first when the file is new.
func appendFields(path string, values []string) error {
	_, statErr := os.Stat(path)
	fresh := os.IsNotExist(statErr)

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}

	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()

	w := csv.NewWriter(f)
	w.Comma = '\t'

	write := func(fields []string) error {
		if err := w.Write(fields); err != nil {
			return err
		}
		w.Flush()
		return w.Error()
	}

	if fresh {
		// The '#' is not written by the csv writer, so it goes in front of
		// the first field.
		withHash := append([]string{"#" + headerFields[0]}, headerFields[1:]...)
		if err := write(withHash); err != nil {
			return err
		}
	}

	return write(values)
}
