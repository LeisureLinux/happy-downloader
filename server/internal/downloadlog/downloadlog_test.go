package downloadlog

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/LeisureLinux/happy-downloader/v4/server/config"
)

// useTempConfig points the log at a temporary directory.
func useTempConfig(t *testing.T, path string) {
	t.Helper()

	c := &config.Config{}
	c.Logging.DownloadLogPath = path
	config.SetInstance(c)

	t.Cleanup(func() { config.SetInstance(&config.Config{}) })
}

func readLines(t *testing.T, path string) []string {
	t.Helper()

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading the log: %v", err)
	}

	return strings.Split(strings.TrimRight(string(content), "\n"), "\n")
}

func TestRecordWritesOneLinePerDownload(t *testing.T) {
	path := filepath.Join(t.TempDir(), "downloads.log")
	useTempConfig(t, path)

	Record(Outcome{
		Status:     StatusSuccess,
		FromIP:     "192.168.68.168",
		URL:        "https://example.com/v/1",
		Path:       "/media/a video.mp4",
		Size:       1024,
		Elapsed:    2 * time.Second,
		Downloader: "generic",
	})

	Record(Outcome{
		Status:     StatusFailed,
		FromIP:     "192.168.68.168",
		URL:        "https://example.com/v/2",
		Downloader: "generic",
		Reason:     "ERROR: unsupported URL",
	})

	lines := readLines(t, path)

	if len(lines) != 3 {
		t.Fatalf("want a header and two lines, got %d: %q", len(lines), lines)
	}

	if !strings.HasPrefix(lines[0], "#finished_at") {
		t.Errorf("the first line should be the header, got %q", lines[0])
	}

	success := strings.Split(lines[1], "\t")
	if len(success) != len(headerFields) {
		t.Fatalf("want %d columns, got %d: %q", len(headerFields), len(success), lines[1])
	}

	if success[1] != StatusSuccess {
		t.Errorf("status: want %q, got %q", StatusSuccess, success[1])
	}
	if success[2] != "192.168.68.168" {
		t.Errorf("from_ip: want the address, got %q", success[2])
	}
	if success[3] != "https://example.com/v/1" {
		t.Errorf("url: got %q", success[3])
	}
	// The filename has a space in it; the tab separator is what keeps it in
	// one column.
	if success[4] != "/media/a video.mp4" {
		t.Errorf("path: got %q", success[4])
	}
	if success[5] != "1024" {
		t.Errorf("size: got %q", success[5])
	}
	if success[6] != "2.0" {
		t.Errorf("elapsed: got %q", success[6])
	}

	failure := strings.Split(lines[2], "\t")
	if failure[1] != StatusFailed {
		t.Errorf("status: want %q, got %q", StatusFailed, failure[1])
	}
	if failure[8] != "ERROR: unsupported URL" {
		t.Errorf("reason: got %q", failure[8])
	}
	// A failed download has no file, so the column is a placeholder rather
	// than an empty string that reads like a missing column.
	if failure[4] != "-" {
		t.Errorf("path of a failed download: want \"-\", got %q", failure[4])
	}
	if failure[5] != "-" {
		t.Errorf("size of a failed download: want \"-\", got %q", failure[5])
	}
}

func TestRecordKeepsOneLineWhenTheReasonIsMultiLine(t *testing.T) {
	path := filepath.Join(t.TempDir(), "downloads.log")
	useTempConfig(t, path)

	Record(Outcome{
		Status: StatusFailed,
		URL:    "https://example.com/v",
		Reason: "ERROR: first line\nsecond line\twith a tab",
	})

	lines := readLines(t, path)

	if len(lines) != 2 {
		t.Fatalf("a multi-line reason must not add lines: %q", lines)
	}

	fields := strings.Split(lines[1], "\t")
	if len(fields) != len(headerFields) {
		t.Fatalf("a tab in the reason must not add a column: %q", lines[1])
	}

	if !strings.Contains(fields[8], "first line second line with a tab") {
		t.Errorf("the reason should be flattened, got %q", fields[8])
	}
}

func TestHeaderIsWrittenOnce(t *testing.T) {
	path := filepath.Join(t.TempDir(), "downloads.log")
	useTempConfig(t, path)

	for i := 0; i < 3; i++ {
		Record(Outcome{Status: StatusSuccess, URL: "https://example.com/v"})
	}

	lines := readLines(t, path)
	headers := 0
	for _, l := range lines {
		if strings.HasPrefix(l, "#") {
			headers++
		}
	}

	if headers != 1 {
		t.Errorf("want exactly one header, got %d", headers)
	}
	if len(lines) != 4 {
		t.Errorf("want a header and three lines, got %d", len(lines))
	}
}

func TestPathFollowsTheStateDirectory(t *testing.T) {
	c := &config.Config{}
	c.Paths.LocalDatabasePath = "/var/lib/happydl"
	config.SetInstance(c)
	t.Cleanup(func() { config.SetInstance(&config.Config{}) })

	if got := Path(); got != "/var/lib/happydl/downloads.log" {
		t.Errorf("want the log in the state directory, got %q", got)
	}

	// An explicit path wins, which is what lets an installation put it
	// wherever its log collection lives.
	c.Logging.DownloadLogPath = "/var/log/happydl/downloads.log"
	if got := Path(); got != "/var/log/happydl/downloads.log" {
		t.Errorf("want the configured path, got %q", got)
	}
}
