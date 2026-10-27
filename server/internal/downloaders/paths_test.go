package downloaders

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LeisureLinux/happy-downloader/v4/server/config"
	"github.com/LeisureLinux/happy-downloader/v4/server/internal"
)

// The output template has to be relative for yt-dlp's temporary path to be
// honoured at all: "--paths temp:" is ignored when "--output" is absolute,
// which is stated in yt-dlp's own option help.
//
// That is why the download directory holds only finished files and the
// partial file lives somewhere else — and it is easy to break by accident, by
// going back to an absolute path.
func TestRelativeOutputName(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()

	paths := &config.Instance().Paths
	previousRoot := paths.DownloadPath
	t.Cleanup(func() { paths.DownloadPath = previousRoot })

	paths.DownloadPath = root

	cases := []struct {
		name    string
		output  string
		want    string
		wantErr bool
	}{
		{
			name:   "a file in the download directory",
			output: filepath.Join(root, "Video.mp4"),
			want:   "Video.mp4",
		},
		{
			name:   "a file in a subdirectory",
			output: filepath.Join(root, "sub", "Video.mp4"),
			want:   filepath.Join("sub", "Video.mp4"),
		},
		{
			// The template is what yt-dlp expands, so it arrives with the
			// placeholders in it.
			name:   "a template",
			output: filepath.Join(root, "Video.%(ext)s"),
			want:   "Video.%(ext)s",
		},
		{
			name:    "a path that leaves the download directory",
			output:  filepath.Join(outside, "Video.mp4"),
			wantErr: true,
		},
		{
			name:    "a path that climbs out",
			output:  filepath.Join(root, "..", "elsewhere", "Video.mp4"),
			wantErr: true,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := relativeOutputName(c.output, "")

			if c.wantErr {
				if err == nil {
					t.Fatalf("expected an error, got %q", got)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if got != c.want {
				t.Errorf("got %q, want %q", got, c.want)
			}
		})
	}
}

// The partial files have to go somewhere that is not the download directory,
// or the file browser lists a half-written video as a finished one.
func TestTempDirIsOutsideTheDownloadDirectory(t *testing.T) {
	root := t.TempDir()
	state := t.TempDir()

	paths := &config.Instance().Paths
	previousRoot := paths.DownloadPath
	previousState := paths.LocalDatabasePath
	t.Cleanup(func() {
		paths.DownloadPath = previousRoot
		paths.LocalDatabasePath = previousState
	})

	paths.DownloadPath = root
	paths.LocalDatabasePath = state

	home, temp, err := outputPaths()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if home != root {
		t.Errorf("the home path is %q, want %q", home, root)
	}

	if temp == "" {
		t.Fatal("no temporary directory was prepared")
	}

	if rel, err := filepath.Rel(root, temp); err == nil && !filepath.IsAbs(rel) &&
		rel != ".." && len(rel) > 1 && rel[:3] != ".."+string(filepath.Separator) {
		t.Errorf("the temporary directory %q is inside the download directory %q", temp, root)
	}

	// It has to exist, or yt-dlp cannot write the partial file there.
	if _, err := os.Stat(temp); err != nil {
		t.Errorf("the temporary directory does not exist: %v", err)
	}
}

// The path arguments are the whole mechanism: the home path, the temporary
// path, and a relative template.
func TestPathArgs(t *testing.T) {
	args := pathArgs("/downloads", "/state/partial", "Video.%(ext)s")

	joined := strings.Join(args, " ")

	for _, want := range []string{"-P home:/downloads", "-P temp:/state/partial", "-o Video.%(ext)s"} {
		if !strings.Contains(joined, want) {
			t.Errorf("the arguments %q are missing %q", joined, want)
		}
	}
}

// A stopped download is paused, not finished. Treating it as finished put it
// in the file browser as a file that was never written.
func TestStopLeavesTheDownloadPaused(t *testing.T) {
	g := NewGenericDownload("https://example.com/video", []string{}).(*GenericDownloader)

	// No process has been started, so the stop cannot kill anything — but the
	// status must still not say "finished".
	_ = g.Stop()

	if g.IsCompleted() {
		t.Error("a stopped download reports itself as completed")
	}

	if got := g.Status().Progress.Status; got != internal.StatusPaused {
		t.Errorf("the status is %d, want %d (paused)", got, internal.StatusPaused)
	}
}
