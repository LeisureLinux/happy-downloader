package downloaders

import (
	"net/url"
	"strconv"
	"strings"
)

// Rewriting a pasted link into the one the extractor recognises.
//
// People paste links from the app, not from the address bar, and the two are
// different addresses for the same video:
//
//   - Douyin's share sheet gives "douyin.com/jingxuan?modal_id=7700252144473034035",
//     which is the video inside a feed. The extractor only knows
//     "douyin.com/video/7700252144473034035".
//   - A link with tracking parameters — "?share_source=copy_web&vd_source=..." —
//     names the same video as one without them.
//
// So the link is put into the form the extractor wants before it is given to
// yt-dlp. Without this the answer is "Unsupported URL", which says nothing
// about which part of the link was wrong.

// NormalizeURL rewrites a URL into the form the site's extractor accepts.
//
// It only touches the links it can be sure about: a video id that is already
// in the link is moved into the path the extractor expects. Everything else is
// left exactly as it was, because a rewrite that guesses is worse than one
// that does nothing — it sends the extractor a URL that names a different
// video.
func NormalizeURL(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return raw
	}

	parsed, err := url.Parse(raw)
	if err != nil || parsed.Host == "" {
		return raw
	}

	host := strings.ToLower(parsed.Hostname())
	host = strings.TrimPrefix(host, "www.")

	switch host {
	case "douyin.com", "iesdouyin.com":
		return normalizeDouyin(parsed)
	}

	return raw
}

// normalizeDouyin puts a Douyin link into the form the extractor takes.
//
// The extractor knows two shapes: "douyin.com/video/<id>" and a short link
// from "v.douyin.com". A feed link carries the id in a query parameter
// instead — "modal_id" for the video that is open, "aweme_id" for the one a
// link points at — and that id is the same number the video path uses.
func normalizeDouyin(parsed *url.URL) string {
	// A short link is already right, and it is followed by yt-dlp itself.
	if strings.HasSuffix(parsed.Hostname(), "v.douyin.com") {
		return parsed.String()
	}

	// The video path is already right.
	if strings.HasPrefix(parsed.Path, "/video/") {
		return parsed.String()
	}

	for _, key := range []string{"modal_id", "aweme_id"} {
		id := parsed.Query().Get(key)

		if id == "" {
			continue
		}

		// Only a plain number is moved across. Anything else is a token the
		// extractor would not accept either, and guessing at it would
		// produce a URL that names a different video.
		if _, err := strconv.ParseUint(id, 10, 64); err != nil {
			continue
		}

		return "https://www.douyin.com/video/" + id
	}

	return parsed.String()
}
