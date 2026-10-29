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

	if success[1] != KindDownload {
		t.Errorf("kind: want %q, got %q", KindDownload, success[1])
	}
	if success[2] != StatusSuccess {
		t.Errorf("status: want %q, got %q", StatusSuccess, success[2])
	}
	if success[3] != "192.168.68.168" {
		t.Errorf("from_ip: want the address, got %q", success[3])
	}
	if success[4] != "https://example.com/v/1" {
		t.Errorf("url: got %q", success[4])
	}
	// The filename has a space in it; the tab separator is what keeps it in
	// one column.
	if success[5] != "/media/a video.mp4" {
		t.Errorf("path: got %q", success[5])
	}
	if success[7] != "1024" {
		t.Errorf("size: got %q", success[7])
	}
	if success[8] != "2.0" {
		t.Errorf("elapsed: got %q", success[8])
	}

	failure := strings.Split(lines[2], "\t")
	if failure[2] != StatusFailed {
		t.Errorf("status: want %q, got %q", StatusFailed, failure[2])
	}
	if failure[10] != "ERROR: unsupported URL" {
		t.Errorf("reason: got %q", failure[10])
	}
	// A failed download has no file, so the column is a placeholder rather
	// than an empty string that reads like a missing column.
	if failure[5] != "-" {
		t.Errorf("path of a failed download: want \"-\", got %q", failure[5])
	}
	if failure[7] != "-" {
		t.Errorf("size of a failed download: want \"-\", got %q", failure[7])
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

	if !strings.Contains(fields[10], "first line second line with a tab") {
		t.Errorf("the reason should be flattened, got %q", fields[10])
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

// TestRecordOperationCoversWhatHappensToAFile is the wider log: a move, a
// delete, a renamed folder. The question "what happened to that video" has one
// answer, and it is not only downloads.
func TestRecordOperationCoversWhatHappensToAFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "downloads.log")
	useTempConfig(t, path)

	Record(Outcome{
		Kind:   KindMove,
		Status: StatusSuccess,
		FromIP: "192.168.68.168",
		Path:   "/downloads/a.mp4",
		To:     "/downloads/AI/a.mp4",
	})

	Record(Outcome{
		Kind:   KindRename,
		Status: StatusSuccess,
		Path:   "/downloads/Old",
		To:     "/downloads/New",
	})

	Record(Outcome{
		Kind:   KindDelete,
		Status: StatusFailed,
		Path:   "/downloads/b.mp4",
		Reason: "permission denied",
	})

	lines := readLines(t, path)

	if len(lines) != 4 {
		t.Fatalf("want a header and three lines, got %d: %q", len(lines), lines)
	}

	move := strings.Split(lines[1], "\t")
	if move[1] != KindMove || move[5] != "/downloads/a.mp4" || move[6] != "/downloads/AI/a.mp4" {
		t.Errorf("a move should record both ends: %q", lines[1])
	}
	// The columns a download fills and a move does not are placeholders, so
	// the line still has the same shape as every other line.
	if move[4] != "-" || move[7] != "-" {
		t.Errorf("unused columns should be placeholders: %q", lines[1])
	}

	del := strings.Split(lines[3], "\t")
	if del[2] != StatusFailed || del[10] != "permission denied" {
		t.Errorf("a failed delete should carry its reason: %q", lines[3])
	}
}

// TestAnOlderFormatIsMovedAside is the migration.
//
// The columns gained a "kind" when operations joined downloads. A file written
// before that has a different header, and appending to it would produce a log
// whose columns mean different things on different lines — which nothing can
// read.
func TestAnOlderFormatIsMovedAside(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "downloads.log")
	useTempConfig(t, path)

	older := "#finished_at\tstatus\tfrom_ip\turl\tpath\tsize_bytes\telapsed_s\tdownloader\treason\n" +
		"2026-10-29T10:00:00+08:00\tsuccess\t1.2.3.4\thttps://x\t/a.mp4\t1\t2.0\tgeneric\t-\n"

	if err := os.WriteFile(path, []byte(older), 0o600); err != nil {
		t.Fatalf("writing the older log: %v", err)
	}

	Record(Outcome{Kind: KindDownload, Status: StatusSuccess, URL: "https://y", Path: "/b.mp4"})

	// The old file is kept, under a name that says it is the previous one.
	if _, err := os.Stat(path + ".1"); err != nil {
		t.Errorf("the older log should have been kept aside: %v", err)
	}

	lines := readLines(t, path)
	if len(lines) != 2 {
		t.Fatalf("want a fresh header and one line, got %d: %q", len(lines), lines)
	}

	if !strings.HasPrefix(lines[0], "#finished_at\tkind\tstatus") {
		t.Errorf("the new file should start with the current header: %q", lines[0])
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
