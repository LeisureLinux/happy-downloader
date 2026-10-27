package rest

import "testing"

// The language code alone is not a list anyone can choose from: a popular
// video has 165 of them, and "de-DE" says nothing to a person who does not
// know the code. The name is what makes the list readable.
func TestCaptionTracksUseTheName(t *testing.T) {
	subtitles := map[string]any{
		"en": []any{
			map[string]any{"ext": "json3"},
			map[string]any{"ext": "srt", "name": "English"},
		},
	}

	tracks := captionTracks(subtitles, nil)

	if len(tracks) != 1 {
		t.Fatalf("got %d tracks, want 1", len(tracks))
	}

	if tracks[0].Name != "English" {
		t.Errorf("Name = %q, want English", tracks[0].Name)
	}
}

// A track with no name anywhere falls back to its code rather than showing
// nothing.
func TestCaptionTracksFallBackToTheCode(t *testing.T) {
	subtitles := map[string]any{
		"xx": []any{map[string]any{"ext": "srt"}},
	}

	tracks := captionTracks(subtitles, nil)

	if len(tracks) != 1 || tracks[0].Name != "xx" {
		t.Errorf("got %+v, want the code as the name", tracks)
	}
}

// The generated tracks come after the authored ones.
func TestCaptionTracksOrderAuthoredFirst(t *testing.T) {
	subtitles := map[string]any{"en": []any{map[string]any{"name": "English"}}}
	automatic := map[string]any{"en": []any{map[string]any{"name": "English (auto-generated)"}}}

	tracks := captionTracks(subtitles, automatic)

	if len(tracks) != 2 {
		t.Fatalf("got %d tracks, want 2", len(tracks))
	}

	if tracks[0].Automatic {
		t.Error("the authored track was not first")
	}
}
