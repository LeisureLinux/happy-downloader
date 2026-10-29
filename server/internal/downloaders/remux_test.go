package downloaders

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// makeMedia writes a few seconds of video, using ffmpeg.
//
// It is skipped when ffmpeg is not installed: the remux is the thing under
// test, and there is nothing to test it with if the tool is missing.
func makeMedia(t *testing.T, dir, name string) string {
	t.Helper()

	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("ffmpeg is not installed")
	}

	path := filepath.Join(dir, name)

	cmd := exec.Command(ffmpeg,
		"-y", "-v", "error",
		"-f", "lavfi", "-i", "testsrc=size=160x120:rate=10",
		"-t", "2",
		"-c:v", "libx264", "-preset", "ultrafast", "-pix_fmt", "yuv420p",
		path,
	)

	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("making a test file: %v\n%s", err, out)
	}

	return path
}

// TestRemuxRecordingProducesAPlayableFile is the step that turns a recording
// into something that plays.
//
// It is the part that happens after stop is pressed and before the file
// appears in the browser, and the part a person is waiting for: the recording
// is written as MPEG-TS whatever it is called, and a player believes the
// extension.
func TestRemuxRecordingProducesAPlayableFile(t *testing.T) {
	dir := t.TempDir()

	// What a recording looks like before the remux: MP4 bytes in a file
	// called ".ts". ffmpeg detects the container from the content, which is
	// what makes the remux work for a stream of any shape.
	raw := makeMedia(t, dir, "recording.ts")
	final := filepath.Join(dir, "recording.mp4")

	if err := remuxRecording(raw, final); err != nil {
		t.Fatalf("remuxing: %v", err)
	}

	info, err := os.Stat(final)
	if err != nil {
		t.Fatalf("the remuxed file is missing: %v", err)
	}

	if info.Size() == 0 {
		t.Fatal("the remuxed file is empty")
	}

	// Named ".mp4" is not the same as being one: the file has to say so
	// itself, or the remux copied the wrong thing.
	ffprobe, err := exec.LookPath("ffprobe")
	if err != nil {
		return
	}

	out, err := exec.Command(ffprobe,
		"-v", "error",
		"-show_entries", "format=format_name",
		"-of", "default=nw=1:nk=1",
		final,
	).Output()

	if err != nil {
		t.Fatalf("probing the remuxed file: %v", err)
	}

	if got := string(out); got == "" || got == "mpegts\n" {
		t.Errorf("the file is still MPEG-TS, not a container that plays: %q", got)
	}
}

// TestFinishRecordingKeepsTheRawFileWhenTheRemuxFails covers what the file
// browser shows when ffmpeg is missing or refuses.
//
// The recording is kept, and its extension says what it is: a file that plays
// in a player that knows MPEG-TS is better than no file.
func TestFinishRecordingKeepsTheRawFileWhenTheRemuxFails(t *testing.T) {
	dir := t.TempDir()

	raw := makeMedia(t, dir, "recording.ts")

	// A final path in a directory that does not exist, so ffmpeg cannot
	// write it.
	final := filepath.Join(dir, "missing", "recording.mp4")

	live := NewLiveStreamDownloader("http://example.invalid/stream", nil).(*LiveStreamDownloader)

	if got := live.finishRecording(raw, final); got != raw {
		t.Errorf("want the raw file back when the remux fails, got %q", got)
	}

	if _, err := os.Stat(raw); err != nil {
		t.Errorf("the raw recording should have been kept: %v", err)
	}
}
