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
	if rawURL == "" {
		return ""
	}

	parsed, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}

	host := strings.ToLower(parsed.Hostname())
	host = strings.TrimPrefix(host, "www.")

	if host == "" {
		return ""
	}

	// The known sites, matched on the part of the host that identifies them.
	//
	// A subdomain is not a different site: "music.youtube.com" is YouTube and
	// "www.bilibili.com" is Bilibili.
	switch {
	case strings.HasSuffix(host, "youtube.com"), strings.HasSuffix(host, "youtu.be"):
		return "youtube"
	case strings.HasSuffix(host, "bilibili.com"), strings.HasSuffix(host, "b23.tv"):
		return "bilibili"
	case strings.HasSuffix(host, "x.com"), strings.HasSuffix(host, "twitter.com"):
		return "x"
	case strings.HasSuffix(host, "xiaohongshu.com"), strings.HasSuffix(host, "xhslink.com"):
		return "xiaohongshu"
	case strings.HasSuffix(host, "douyin.com"), strings.HasSuffix(host, "iesdouyin.com"):
		return "douyin"
	case strings.HasSuffix(host, "tiktok.com"):
		return "tiktok"
	case strings.HasSuffix(host, "reddit.com"):
		return "reddit"
	case strings.HasSuffix(host, "instagram.com"):
		return "instagram"
	case strings.HasSuffix(host, "vimeo.com"):
		return "vimeo"
	case strings.HasSuffix(host, "twitch.tv"):
		return "twitch"
	case strings.HasSuffix(host, "facebook.com"), strings.HasSuffix(host, "fb.watch"):
		return "facebook"
	case strings.HasSuffix(host, "nicovideo.jp"):
		return "niconico"
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

	return host
}
