package downloaders

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/LeisureLinux/happy-downloader/v4/server/internal"
)

// The postprocess template has to carry the fields the server relies on.
// `finaldir` is yt-dlp's own internal key, so it is worth asserting here
// before a version change silently turns it into an empty string.
func TestPostprocessTemplateFields(t *testing.T) {
	for _, field := range []string{
		`"filepath":"%(info.filepath)s"`,
		`"finaldir":"%(info.__finaldir)s"`,
		`"status":"%(progress.status)s"`,
	} {
		if !strings.Contains(postprocessTemplate, field) {
			t.Errorf("postprocessTemplate is missing %s", field)
		}
	}
}

// sanitizeProgressJSON must still turn `NA` into null now that the template
// has more fields after it, and the parsed result has to carry the finaldir.
func TestParsePostprocessWithMissingDimensions(t *testing.T) {
	entry := []byte(`{"filepath":"/tmp/state/partial/Video.mp4","finaldir":"/srv/media/downloads","status":"finished","width":NA,"height":NA}`)

	var got internal.PostprocessTemplate
	if err := jsonUnmarshalForTest(sanitizeProgressJSON(entry), &got); err != nil {
		t.Fatalf("postprocess line is not valid JSON after sanitizing: %v", err)
	}

	if got.FilePath != "/tmp/state/partial/Video.mp4" {
		t.Errorf("filepath = %q", got.FilePath)
	}
	if got.FinalDir != "/srv/media/downloads" {
		t.Errorf("finaldir = %q", got.FinalDir)
	}
	if got.Status != "finished" {
		t.Errorf("status = %q", got.Status)
	}
}

// A thin wrapper so the test reads like the consumer does.
func jsonUnmarshalForTest(b []byte, v *internal.PostprocessTemplate) error {
	return json.Unmarshal(b, v)
}
