package filebrowser

import (
	"net/url"
	"strings"

	"github.com/LeisureLinux/happy-downloader/v4/server/internal/sites"
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

	// The catalogue is the one place that knows which hosts mean which site.
	// It used to be a switch here, a map in the cookie package and a list in
	// the interface, and they drifted apart.
	if site, ok := sites.ForHost(host); ok {
		return site.Key, true
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
