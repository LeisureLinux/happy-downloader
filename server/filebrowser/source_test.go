package filebrowser

import "testing"

// yt-dlp's --embed-metadata puts the page URL in the comment field.
func TestSourceFromTags(t *testing.T) {
	tags := map[string]string{
		"comment": "https://www.youtube.com/watch?v=KBkHNI_nRLY",
		"title":   "The Complete JAVA Ecosystem Explained in 18 Minutes",
	}

	url, ok := sourceFromTags(tags)
	if !ok {
		t.Fatal("no source was found")
	}

	if url != "https://www.youtube.com/watch?v=KBkHNI_nRLY" {
		t.Errorf("url = %q", url)
	}
}

// Some containers carry the URL in the description instead.
func TestSourceFromTagsDescription(t *testing.T) {
	tags := map[string]string{
		"description": "A description.\nhttps://youtu.be/abc\nMore text.",
	}

	url, ok := sourceFromTags(tags)
	if !ok {
		t.Fatal("no source was found")
	}

	if url != "https://youtu.be/abc" {
		t.Errorf("url = %q", url)
	}
}

// A description that is only prose is not a source.
func TestSourceFromTagsIgnoresProse(t *testing.T) {
	tags := map[string]string{
		"description": "In this video we look at the Java ecosystem.",
	}

	if _, ok := sourceFromTags(tags); ok {
		t.Error("prose was taken for a URL")
	}
}

// The trailing punctuation of a sentence is not part of the URL.
func TestURLInTextStopsAtWhitespace(t *testing.T) {
	url, ok := urlInText("see https://youtu.be/abc. Thanks!")
	if !ok {
		t.Fatal("no URL was found")
	}

	if url != "https://youtu.be/abc" {
		t.Errorf("url = %q, want the trailing period dropped", url)
	}
}
