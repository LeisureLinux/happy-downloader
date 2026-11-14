// Package kv stores the in-memory state of the downloader.
//
// This file holds the identity function used to decide whether two pasted
// links are the same video. It lives here, rather than in an archive or a
// site-profile package, because deduplication is a property of the queue:
// the queue is what must not run two processes at the same file.
package kv

import (
	"net/url"
	"strings"

	"github.com/LeisureLinux/happy-downloader/v4/server/internal/downloaders"
)

// trackingParams are the query parameters that describe how a link was
// opened rather than which video it points at. Two links to one video that
// differ only in these are one download.
//
// The list is deliberately conservative: an unknown parameter may identify
// the video (a Douyin modal_id, an iQiyi tvid), and dropping it would merge
// two different videos into one key — a worse mistake than keeping a
// duplicate. `a` and `s` are X/Twitter's share tokens.
var trackingParams = map[string]bool{
	"list": true, "index": true, "t": true, "start": true,
	"time_continue": true, "feature": true, "si": true, "pp": true,
	"spm_id_from": true, "vd_source": true, "share_source": true,
	"utm_source": true, "utm_medium": true, "utm_campaign": true,
	"from": true, "ref": true, "s": true, "a": true,
}

// KeyForURL reduces a video page URL to the identity of the video.
//
// A channel or playlist URL is not a video and never produces a key: two
// different channels must not be merged into one download. An empty key
// means "do not deduplicate this URL".
func KeyForURL(raw string) string {
	raw = strings.TrimSpace(downloaders.NormalizeURL(raw))
	if raw == "" {
		return ""
	}

	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		// yt-dlp accepts scheme-less URLs; parse again with a scheme rather
		// than giving up and returning an unusable key.
		u, err = url.Parse("https://" + strings.TrimPrefix(raw, "//"))
		if err != nil || u.Host == "" {
			return ""
		}
	}

	host := strings.ToLower(u.Hostname())
	host = strings.TrimPrefix(host, "www.")
	host = strings.TrimPrefix(host, "m.")

	// A YouTube short link and the watch page are the same video.
	if host == "youtu.be" {
		id := strings.Trim(u.Path, "/")
		if id != "" {
			return "https://youtube.com/watch?v=" + id
		}
	}

	path := strings.TrimSuffix(u.Path, "/")
	if host == "youtube.com" && (strings.HasPrefix(path, "/@") ||
		strings.HasPrefix(path, "/channel/") || strings.HasPrefix(path, "/c/") ||
		strings.HasPrefix(path, "/user/") || path == "/playlist") {
		return ""
	}

	q := u.Query()
	for k := range q {
		if trackingParams[strings.ToLower(k)] {
			q.Del(k)
		}
	}

	out := url.URL{Scheme: "https", Host: host, Path: path}
	if encoded := q.Encode(); encoded != "" {
		out.RawQuery = encoded
	}

	return out.String()
}
