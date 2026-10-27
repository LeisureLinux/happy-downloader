package filebrowser

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSrtToVTT(t *testing.T) {
	srt := "1\n00:00:01,000 --> 00:00:04,000\nHello there\n\n2\n00:00:05,500 --> 00:00:08,250\nSecond line\n"

	vtt := srtToVTT(srt)

	if !hasPrefix(vtt, "WEBVTT\n") {
		t.Errorf("missing the WEBVTT header:\n%s", vtt)
	}

	if !contains(vtt, "00:00:01.000 --> 00:00:04.000") {
		t.Errorf("the first cue was not converted:\n%s", vtt)
	}

	if !contains(vtt, "00:00:05.500 --> 00:00:08.250") {
		t.Errorf("the second cue was not converted:\n%s", vtt)
	}

	if !contains(vtt, "Hello there") {
		t.Error("the caption text was dropped")
	}
}

// SubRip allows coordinates after the end timestamp; WebVTT does not.
func TestSrtToVTTDropsCoordinates(t *testing.T) {
	srt := "1\n00:00:01,000 --> 00:00:04,000 X1:10 X2:20 Y1:30 Y2:40\nText\n"

	vtt := srtToVTT(srt)

	if contains(vtt, "X1:") {
		t.Errorf("the coordinates survived:\n%s", vtt)
	}

	if !contains(vtt, "00:00:01.000 --> 00:00:04.000") {
		t.Errorf("the timing was not converted:\n%s", vtt)
	}
}

// A cue index is a number on its own line, and the timing follows it.
func TestSrtToVTTSkipsCueIndex(t *testing.T) {
	srt := "1\n00:00:01,000 --> 00:00:04,000\nText\n"

	if contains(srtToVTT(srt), "\n1\n") {
		t.Error("the cue index was kept")
	}
}

func TestConvertTimingLeavesOtherLinesAlone(t *testing.T) {
	if got := convertTiming("not a timing"); got != "not a timing" {
		t.Errorf("got %q", got)
	}
}

func hasPrefix(s, prefix string) bool {
	return len(s) >= len(prefix) && s[:len(prefix)] == prefix
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (func() bool {
		for i := 0; i+len(sub) <= len(s); i++ {
			if s[i:i+len(sub)] == sub {
				return true
			}
		}
		return false
	})()
}

// The caption file is named after the video's stem, not its full name:
// "Video.mp4" goes with "Video.en.srt". Matching on the full name finds
// nothing.
func TestSubtitlesForMatchesTheStem(t *testing.T) {
	dir := t.TempDir()

	write := func(name string) {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	write("Video.mp4")
	write("Video.en.srt")
	write("Video.zh-Hans.srt")
	write("Video.part")
	write("Other.en.srt")

	found := subtitlesFor(dir, "Video.mp4")

	if len(found) != 2 {
		t.Fatalf("found %d tracks, want 2: %+v", len(found), found)
	}

	if found[0].Language != "en" || found[1].Language != "zh-Hans" {
		t.Errorf("got languages %q and %q", found[0].Language, found[1].Language)
	}
}

// A video called "Part 1" must not pick up the captions of "Part 10".
func TestSubtitlesForDoesNotMatchLongerNames(t *testing.T) {
	dir := t.TempDir()

	if err := os.WriteFile(filepath.Join(dir, "Part 1.mp4"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "Part 10.en.srt"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}

	if found := subtitlesFor(dir, "Part 1.mp4"); len(found) != 0 {
		t.Errorf("found %+v, want none", found)
	}
}
