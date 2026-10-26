package downloaders

import (
	"testing"

	"github.com/LeisureLinux/happy-downloader/v4/server/internal"
)

// A restart used to re-run everything that had ever finished, because the
// restored downloader reported itself as unfinished.
func TestRestoreKeepsCompletedState(t *testing.T) {
	d := NewGenericDownload("", nil)
	snap := internal.ProcessSnapshot{
		Id:       "abc",
		URL:      "https://example.com/v",
		Progress: internal.DownloadProgress{Status: internal.StatusCompleted},
	}

	if err := d.RestoreFromSnapshot(&snap); err != nil {
		t.Fatal(err)
	}

	if !d.IsCompleted() {
		t.Error("a completed snapshot restored as unfinished")
	}
	if d.GetUrl() != "https://example.com/v" {
		t.Errorf("URL = %q, want the requested URL", d.GetUrl())
	}
}

// Entries written before the URL was recorded in the snapshot only have the
// metadata copy, which is empty when the metadata stage failed.
func TestRestoreFallsBackToMetadataURL(t *testing.T) {
	d := NewGenericDownload("", nil)
	snap := internal.ProcessSnapshot{Id: "abc"}
	snap.Info.URL = "https://example.com/from-metadata"

	if err := d.RestoreFromSnapshot(&snap); err != nil {
		t.Fatal(err)
	}

	if d.GetUrl() != "https://example.com/from-metadata" {
		t.Errorf("URL = %q, want the metadata fallback", d.GetUrl())
	}
}
