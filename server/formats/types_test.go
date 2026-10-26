package formats

import (
	"encoding/json"
	"testing"
)

// The wire format is what the browser reads, and a renamed field is silent:
// the frontend type simply stops matching and the value arrives as undefined.
// That is exactly how automatic_captions went missing — the Go tag was
// snake_case, the TypeScript type was camelCase, and the subtitle picker saw
// an empty list for a video that had 177 generated tracks.
//
// These are the names the frontend depends on. Changing one is a breaking
// change and should require changing this test.
func TestMetadataWireFormat(t *testing.T) {
	m := Metadata{
		Type:              "video",
		Title:             "t",
		Thumbnail:         "u",
		Duration:          213,
		Subtitles:         map[string]any{"en": []any{}},
		AutomaticCaptions: map[string]any{"en": []any{}},
		Formats:           []Format{{Format_id: "137", Ext: "mp4"}},
	}

	raw, err := json.Marshal(m)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var got map[string]json.RawMessage
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	// Fields the frontend reads by name. The passthrough ones keep yt-dlp's
	// spelling; only the ones this server invents use camelCase.
	for _, key := range []string{
		"_type", "title", "thumbnail", "duration", "url", "entries",
		"formats", "best",
		"subtitles", "automatic_captions",
	} {
		if _, ok := got[key]; !ok {
			t.Errorf("missing wire field %q; frontend reads it by this name", key)
		}
	}
}

// A format's ext is what the dialog uses to reason about the merge target.
func TestFormatWireFormat(t *testing.T) {
	raw, err := json.Marshal(Format{Format_id: "137", Ext: "mp4", Resolution: "1920x1080"})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var got map[string]json.RawMessage
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	for _, key := range []string{"format_id", "ext", "resolution"} {
		if _, ok := got[key]; !ok {
			t.Errorf("missing format field %q", key)
		}
	}
}
