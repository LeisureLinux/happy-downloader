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
	"bufio"
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
	"finished_at", "kind", "status", "from_ip", "url", "path", "to",
	"size_bytes", "elapsed_s", "downloader", "reason",
}

// The kinds of line in this log.
//
// It began as a log of downloads and is now a log of what happened to the
// files: a recording that finished, a file that was moved, a folder that was
// deleted. The question "what happened to that video" is one question.
const (
	KindDownload = "download"
	KindMove     = "move"
	KindCopy     = "copy"
	KindDelete   = "delete"
	KindRename   = "rename"
	KindFolder   = "folder"
)

// Outcome is one finished thing: a download, or an operation on a file.
type Outcome struct {
	// Kind is KindDownload, KindMove, KindDelete and so on.
	Kind string
	// Status is "success" or "failed".
	Status string
	// FromIP is the machine that asked for the download, as the server saw
	// it. It is empty for a download whose address was not recorded.
	FromIP string
	// URL is what was asked for.
	URL string
	// Path is the file that was written, or the one that was acted on.
	Path string
	// To is where it went, for a move or a rename.
	To string
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

	kind := o.Kind
	if kind == "" {
		kind = KindDownload
	}

	return []string{
		time.Now().Format(time.RFC3339),
		kind,
		status,
		sanitize(o.FromIP),
		sanitize(o.URL),
		sanitize(o.Path),
		sanitize(o.To),
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

// hasDifferentHeader reports whether a log file was written by a different
// version of this format.
func hasDifferentHeader(path string) (bool, bool) {
	f, err := os.Open(path)
	if err != nil {
		return false, false
	}
	defer f.Close()

	reader := bufio.NewReader(f)
	first, err := reader.ReadString('\n')
	if err != nil && first == "" {
		return false, false
	}

	want := "#" + headerFields[0] + "\t" + strings.Join(headerFields[1:], "\t")

	return strings.TrimRight(first, "\n") != want, true
}

// appendFields writes one line, adding the header first when the file is new.
func appendFields(path string, values []string) error {
	_, statErr := os.Stat(path)
	fresh := os.IsNotExist(statErr)

	// A file written by an older shape of this log is moved aside rather than
	// appended to: the columns would not line up, and a log whose columns
	// mean different things on different lines cannot be read by anything.
	if !fresh {
		if older, ok := hasDifferentHeader(path); ok && older {
			os.Rename(path, path+".1")
			fresh = true
		}
	}

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
