package filebrowser

import (
	"strings"
)

// Finding a file's source from the metadata inside it.
//
// yt-dlp's --embed-metadata writes the page's URL into the container's
// comment field, and the title alongside it. So a file that was downloaded
// before the server started remembering — or one that was put there by hand
// with the same tool — still knows where it came from.
//
// Reading it costs a process spawn, so it is a fallback rather than the first
// choice: the server's own index is faster and does not depend on the file
// having been tagged at all.
//
// It is also what answers for a file whose index record was left behind by a
// move that happened before the index learned to move. The record names a path
// that is no longer there, and the file itself still knows.

// SourceFromMetadata reads the source URL out of a media file.
//
// It is a separate call for the callers that have no probe of the file to
// hand — the caption downloader, for one — and it reads the file again. The
// file browser does not use it: it reads the source out of the same probe
// that gives it the length and the codecs, because spawning ffprobe twice
// for every file in a listing is what made the browser take fifteen seconds.
//
// The answer is cached with the rest of the probe, so the second caller does
// not pay for it again either.
func SourceFromMetadata(path string) (string, bool) {
	probed := mediaInfoFor(path)

	return probed.source, probed.source != ""
}

// sourceFromTags picks the source URL out of a tag set.
//
// The comment field is where yt-dlp puts the page's URL, so it is the one
// that is trusted.
//
// The description is only a fallback, and it is a poor one: it is the video's
// own description, which usually starts with a link to somewhere else — a
// course, a sponsor, a discount code. Taking the first URL in it gave a video
// from YouTube the icon of whatever the description happened to advertise.
//
// So the description is used only when it holds a URL that looks like the
// page the video was on: the same host as the comment, or a short link to it.
// A short link is how a description usually refers to its own video — the
// uploader's own tweet, for instance — and it is a t.co link that has to be
// followed.
func sourceFromTags(tags map[string]string) (string, bool) {
	comment := strings.TrimSpace(tags["comment"])

	if url, ok := urlInText(comment); ok {
		return url, true
	}

	description := strings.TrimSpace(tags["description"])
	if description == "" {
		return "", false
	}

	// Every URL in the description, not only the first: the source is often
	// not the first link.
	urls := urlsInText(description)
	if len(urls) == 0 {
		return "", false
	}

	// A short link in the description is the uploader pointing at their own
	// video, so it is the source.
	for _, url := range urls {
		if isShortLink(url) {
			return url, true
		}
	}

	return "", false
}

// isShortLink reports whether a URL is a link shortener.
//
// The description of a video usually carries a shortened link to the video
// itself, and that is the one that says where it came from.
func isShortLink(url string) bool {
	for _, host := range []string{"t.co", "youtu.be", "fb.me", "amzn.to", "youtu.be"} {
		if strings.Contains(url, host) {
			return true
		}
	}

	return false
}

// urlInText finds the first http URL in a piece of text.
func urlInText(text string) (string, bool) {
	urls := urlsInText(text)
	if len(urls) == 0 {
		return "", false
	}

	return urls[0], true
}

// urlsInText finds every http URL in a piece of text.
func urlsInText(text string) []string {
	const scheme = "http"

	var urls []string

	for {
		index := strings.Index(text, scheme)
		if index == -1 {
			break
		}

		end := index
		for end < len(text) && !isURLTerminator(text[end]) {
			end++
		}

		// Trailing punctuation belongs to the sentence, not the URL. A URL
		// that ends in a period is not a URL anyone can fetch.
		url := strings.TrimRight(text[index:end], ".,;:)]}\"'")

		if url != scheme {
			urls = append(urls, url)
		}

		if end >= len(text) {
			break
		}

		text = text[end:]
	}

	return urls
}

func isURLTerminator(c byte) bool {
	switch c {
	case ' ', '\t', '\n', '\r', '<', '>', '"', '\'':
		return true
	}
	return false
}
