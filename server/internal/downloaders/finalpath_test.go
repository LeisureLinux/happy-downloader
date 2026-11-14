package downloaders

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/LeisureLinux/happy-downloader/v4/server/config"
	"github.com/LeisureLinux/happy-downloader/v4/server/internal"
)

func TestFinalOutputPath(t *testing.T) {
	home := t.TempDir()
	outside := t.TempDir()

	previous := config.Instance().Paths
	t.Cleanup(func() { config.Instance().Paths = previous })
	config.Instance().Paths = config.PathsConfig{
		DownloadPath:      home,
		LocalDatabasePath: filepath.Join(outside, "state"),
	}

	temp := tempDir()

	cases := []struct {
		name     string
		reported string
		finalDir string
		want     string
	}{
		{
			name:     "a temp path with a finaldir",
			reported: filepath.Join(temp, "Lena", "Video.mp4"),
			finalDir: filepath.Join(home, "Lena"),
			want:     filepath.Join(home, "Lena", "Video.mp4"),
		},
		{
			name:     "a temp path without a finaldir keeps the subdirectory",
			reported: filepath.Join(temp, "Linux", "Video.mp4"),
			want:     filepath.Join(home, "Linux", "Video.mp4"),
		},
		{
			name:     "a temp path at the root",
			reported: filepath.Join(temp, "Video.mp4"),
			want:     filepath.Join(home, "Video.mp4"),
		},
		{
			name:     "an already-final path is idempotent",
			reported: filepath.Join(home, "Linux", "Video.mp4"),
			want:     filepath.Join(home, "Linux", "Video.mp4"),
		},
		{
			name:     "a relative finaldir resolves against the download path",
			reported: filepath.Join(temp, "Video.mp4"),
			finalDir: "Linux",
			want:     filepath.Join(home, "Linux", "Video.mp4"),
		},
		{
			name:     "a bare filename",
			reported: "Video.mp4",
			want:     filepath.Join(home, "Video.mp4"),
		},
		{
			name:     "a path outside the temp directory is left alone",
			reported: filepath.Join(outside, "Video.mp4"),
			want:     filepath.Join(outside, "Video.mp4"),
		},
		{
			name:     "an empty path",
			reported: "",
			want:     "",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := finalOutputPath(c.reported, c.finalDir)
			if got != c.want {
				t.Fatalf("got %q, want %q", got, c.want)
			}
			// Every translation has to be stable: the path is rewritten more
			// than once (postprocess lines, then the exit handler).
			if again := finalOutputPath(got, ""); again != c.want {
				t.Fatalf("not idempotent: second pass gave %q", again)
			}
		})
	}
}

func TestFindFinishedFile(t *testing.T) {
	home := t.TempDir()
	previous := config.Instance().Paths
	t.Cleanup(func() { config.Instance().Paths = previous })
	config.Instance().Paths = config.PathsConfig{
		DownloadPath:      home,
		LocalDatabasePath: filepath.Join(t.TempDir(), "state"),
	}

	d := NewGenericDownload("https://example.com/v", nil).(*GenericDownloader)
	d.Metadata.Title = "My Video"
	d.SetOutput(internal.DownloadOutput{Filename: "%(title)s.%(ext)s"})
	started := time.Now().Add(-time.Minute)

	if got := findFinishedFile(d, started); got != "" {
		t.Fatalf("found a file before one was written: %q", got)
	}

	// A file left by an earlier attempt is not this run's output. This is
	// the retry case: claiming it would log a failure as a success.
	old := filepath.Join(home, "My Video.mp4")
	if err := os.WriteFile(old, []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}
	stale := started.Add(-time.Minute)
	if err := os.Chtimes(old, stale, stale); err != nil {
		t.Fatal(err)
	}
	if got := findFinishedFile(d, started); got != "" {
		t.Fatalf("a stale file was claimed: %q", got)
	}

	partial := filepath.Join(home, "My Video.mp4.part")
	if err := os.WriteFile(partial, []byte("partial"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := findFinishedFile(d, started); got != "" {
		t.Fatalf("a partial file was claimed: %q", got)
	}

	// A file written during this run is the output.
	if err := os.Chtimes(old, time.Now(), time.Now()); err != nil {
		t.Fatal(err)
	}
	if got := findFinishedFile(d, started); got != old {
		t.Fatalf("got %q, want %q", got, old)
	}

	// A rename request is matched by its own template, not the title.
	d.SetOutput(internal.DownloadOutput{Filename: "renamed.%(ext)s"})
	if got := findFinishedFile(d, started); got != "" {
		t.Fatalf("a custom name matched the title: %q", got)
	}
	renamed := filepath.Join(home, "renamed.mp4")
	if err := os.WriteFile(renamed, []byte("video"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := findFinishedFile(d, started); got != renamed {
		t.Fatalf("got %q, want %q", got, renamed)
	}
}

func TestFileExists(t *testing.T) {
	dir := t.TempDir()

	good := filepath.Join(dir, "video.mp4")
	if err := os.WriteFile(good, []byte("data"), 0o644); err != nil {
		t.Fatal(err)
	}
	empty := filepath.Join(dir, "empty.mp4")
	if err := os.WriteFile(empty, nil, 0o644); err != nil {
		t.Fatal(err)
	}

	for _, c := range []struct {
		path string
		want bool
	}{
		{good, true},
		{empty, false},
		{filepath.Join(dir, "missing.mp4"), false},
		{dir, false},
		{"", false},
	} {
		if got := fileExists(c.path); got != c.want {
			t.Errorf("fileExists(%q) = %v, want %v", c.path, got, c.want)
		}
	}
}

func TestIsCompletePercentage(t *testing.T) {
	for _, c := range []struct {
		in   string
		want bool
	}{
		{"100.0%", true},
		{" 100.0%", true},
		{"100%", true},
		{" 99.9%", false},
		{"", false},
		{"NA", false},
	} {
		if got := isCompletePercentage(c.in); got != c.want {
			t.Errorf("isCompletePercentage(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}

// A progress line must not erase the saved path, and a postprocess line must
// not turn a paused download back into a running one.
func TestParseLogEntrySeparatesProgressFromPostprocess(t *testing.T) {
	home := t.TempDir()
	previous := config.Instance().Paths
	t.Cleanup(func() { config.Instance().Paths = previous })
	config.Instance().Paths = config.PathsConfig{
		DownloadPath:      home,
		LocalDatabasePath: filepath.Join(t.TempDir(), "state"),
	}

	d := NewGenericDownload("https://x.com/a/status/1/video/1", nil).(*GenericDownloader)
	d.UpdateSavedFilePath(filepath.Join(home, "Video.mp4"))
	c := NewJSONLogConsumer()

	c.ParseLogEntry([]byte(`{"eta":12,"percentage":" 50.0%","speed":1024}`), d)
	if got := d.Status().Output.SavedFilePath; got != filepath.Join(home, "Video.mp4") {
		t.Fatalf("a progress line erased the saved path: %q", got)
	}
	if got := d.Status().Progress.Status; got != internal.StatusDownloading {
		t.Fatalf("a 50%% line gave status %d, want downloading", got)
	}

	c.ParseLogEntry([]byte(`{"eta":0,"percentage":"100.0%","speed":0}`), d)
	if got := d.Status().Progress.Status; got != internal.StatusPostprocessing {
		t.Fatalf("a 100%% line gave status %d, want postprocessing", got)
	}
	if got := d.Status().Output.SavedFilePath; got != filepath.Join(home, "Video.mp4") {
		t.Fatalf("the 100%% line erased the saved path: %q", got)
	}

	final := filepath.Join(home, "Video.mp4")
	c.ParseLogEntry([]byte(`{"filepath":"`+filepath.Join(tempDir(), "Video.mp4")+`","finaldir":"`+home+`","status":"finished","width":NA,"height":NA}`), d)
	if got := d.Status().Output.SavedFilePath; got != final {
		t.Fatalf("postprocess saved %q, want %q", got, final)
	}
	if got := d.Status().Progress.Status; got != internal.StatusPostprocessing {
		t.Fatalf("postprocess gave status %d, want postprocessing", got)
	}
}

func TestParseProgressKeepsPausedStatus(t *testing.T) {
	d := NewGenericDownload("https://example.com/v", nil).(*GenericDownloader)
	d.SetProgress(internal.DownloadProgress{Status: internal.StatusPaused})

	NewJSONLogConsumer().ParseLogEntry([]byte(`{"eta":0,"percentage":"100.0%","speed":0}`), d)
	if got := d.Status().Progress.Status; got != internal.StatusPaused {
		t.Fatalf("a late progress line unpaused the download: status %d", got)
	}
}
