package queue

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/LeisureLinux/happy-downloader/v4/server/config"
	"github.com/LeisureLinux/happy-downloader/v4/server/internal"
	"github.com/LeisureLinux/happy-downloader/v4/server/internal/downloaders"
)

// recordOutcome must honour the downloader's own verdict. A retry that fails
// before writing a byte is still a failure even though the first attempt's
// file is still sitting in the download directory.
func TestRecordOutcomeDoesNotInferSuccessFromAnOldFile(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "downloads.log")
	previous := config.Instance().Logging
	t.Cleanup(func() { config.Instance().Logging = previous })
	config.Instance().Logging.DownloadLogPath = logPath

	d := downloaders.NewGenericDownload("https://example.com/v", nil).(*downloaders.GenericDownloader)
	d.SetFromIP("192.168.68.126")
	d.SetProgress(internal.DownloadProgress{Status: internal.StatusErrored})
	d.SetFailureReason("ERROR: postprocessor failed")

	old := filepath.Join(dir, "Video.mp4")
	if err := os.WriteFile(old, []byte("from the first attempt"), 0o644); err != nil {
		t.Fatal(err)
	}
	d.UpdateSavedFilePath(old)

	recordOutcome(d, os.ErrClosed, time.Now().Add(-time.Second))

	got, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}

	line := string(got)
	if !strings.Contains(line, "\tfailed\t") {
		t.Fatalf("an errored download was recorded as a success:\n%s", line)
	}
	if !strings.Contains(line, "ERROR: postprocessor failed") {
		t.Fatalf("the failure reason was lost:\n%s", line)
	}
	if !strings.Contains(line, "192.168.68.126") {
		t.Fatalf("the requesting address was lost:\n%s", line)
	}
}
