package filebrowser

import (
	"net/url"
	"strings"
)

// Which site a file came from.
//
// The names are the ones the icons in the interface are keyed by, and they
// are short on purpose: they travel in the listing of every file.
//
// A host that is not recognised gets its own hostname, so a file from a site
// nobody has an icon for still says where it came from — which is the
// information the icon was there to carry.

// SiteOf returns the site name for a URL.
//
// The host is stripped of the www prefix, because "www.youtube.com" and
// "youtube.com" are the same site and the icon should not depend on which
// one the link happened to use.
func SiteOf(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}

	site, _ := SiteOfHost(parsed.Hostname())
	return site
}

// SiteOfHost is SiteOf for a bare host, and it says whether the site is one
// the application knows.
//
// The distinction matters where a host is listed rather than labelled with an
// icon: a known site is worth naming ("youtube"), and an unknown one is only
// meaningful as the domain itself — the fallback in SiteOf reduces
// "accounts.google.com" to "accounts", which is not a site and reads as one.
func SiteOfHost(rawHost string) (site string, known bool) {
	host := strings.ToLower(strings.TrimSpace(rawHost))
	host = strings.TrimPrefix(host, "www.")
	host = strings.Trim(host, ".")

	if host == "" {
		return "", false
	}

	// The known sites, matched on the part of the host that identifies them.
	//
	// A subdomain is not a different site: "music.youtube.com" is YouTube and
	// "www.bilibili.com" is Bilibili.
	// The short links and the delivery domains.
	//
	// A link pasted from an app is a short link — "t.cn/abc" for Weibo,
	// "b23.tv/abc" for Bilibili — and the file that comes out of one is
	// often served from the site's own delivery host: Reddit's videos live
	// on "packaged-media.redd.it", not on "reddit.com". Matching only the
	// obvious host gave a Reddit video the name of its CDN, and a Weibo
	// short link the name of the shortener.
	switch host {
	case "t.cn", "weibo.cn":
		return "weibo", true
	case "packaged-media.redd.it", "redd.it", "i.redd.it", "preview.redd.it", "v.redd.it":
		return "reddit", true
	}

	switch {
	case strings.HasSuffix(host, "youtube.com"), strings.HasSuffix(host, "youtu.be"):
		return "youtube", true
	case strings.HasSuffix(host, "bilibili.com"), strings.HasSuffix(host, "b23.tv"):
		return "bilibili", true
	case strings.HasSuffix(host, "x.com"), strings.HasSuffix(host, "twitter.com"):
		return "x", true
	case strings.HasSuffix(host, "xiaohongshu.com"), strings.HasSuffix(host, "xhslink.com"):
		return "xiaohongshu", true
	case strings.HasSuffix(host, "douyin.com"), strings.HasSuffix(host, "iesdouyin.com"):
		return "douyin", true
	case strings.HasSuffix(host, "tiktok.com"):
		return "tiktok", true
	case strings.HasSuffix(host, "reddit.com"):
		return "reddit", true
	case strings.HasSuffix(host, "instagram.com"):
		return "instagram", true
	case strings.HasSuffix(host, "vimeo.com"):
		return "vimeo", true
	case strings.HasSuffix(host, "twitch.tv"):
		return "twitch", true
	case strings.HasSuffix(host, "facebook.com"), strings.HasSuffix(host, "fb.watch"):
		return "facebook", true
	case strings.HasSuffix(host, "nicovideo.jp"):
		return "niconico", true
	}

	// The first label of the host, so "example.co.uk" reads as "example".
	// A file from a site nobody has an icon for still says where it came
	// from, which is what the icon was there to carry.
	host = strings.TrimSuffix(host, ".com")
	host = strings.TrimSuffix(host, ".net")
	host = strings.TrimSuffix(host, ".org")
	host = strings.TrimSuffix(host, ".co.uk")
	host = strings.TrimSuffix(host, ".tv")

	if dot := strings.Index(host, "."); dot > 0 {
		host = host[:dot]
	}

	return host, false
}
