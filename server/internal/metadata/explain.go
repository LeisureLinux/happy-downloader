package metadata

import (
	"regexp"
	"strings"

	"github.com/LeisureLinux/happy-downloader/v4/server/internal/sites"
)

// Explaining a failure that says only "Unsupported URL".
//
// yt-dlp's answer for a link it has no extractor for is:
//
//	ERROR: Unsupported URL: https://www.kuaishou.com/short-video/...
//
// which names the problem without saying what to do about it. The person
// cannot tell whether the link is wrong, the site is not supported, or the
// application is at fault.
//
// So the message is rewritten into something that says which of the three it
// is. The site is named, because "this site is not supported" is a different
// answer from "this link is not a video link" and a person needs to know
// which one they are looking at.

// siteNames are the sites the application is known to work with.
//
// A link to one of them is either a link the extractor cannot read — a feed
// link rather than a video link — or one that needs something the request did
// not have. The catalogue decides which hosts those are; this is only the
// lookup for a message.
var siteNames = func() map[string]string {
	names := map[string]string{}

	for _, site := range sites.All() {
		for _, host := range site.Hosts {
			names[host] = site.Name
		}
	}

	return names
}()

// The sites that have no downloader at all.
//
// They are listed apart from the others because the answer is a different one:
// for a site that has an extractor, a link it cannot read is the wrong shape
// of link. For a site that has none, there is nothing that would work — the
// video cannot be downloaded from this application at all, whatever the link
// looks like.
var unsupportedSites = map[string]string{
	"kuaishou.com": "Kuaishou",
}

// The site's own name, if it is one the application knows.
func siteName(host string) (string, bool) {
	host = strings.ToLower(host)
	host = strings.TrimPrefix(host, "www.")

	// The longest match wins, so "m.youtube.com" and "youtube.com" both
	// find YouTube.
	best := ""
	name := ""

	for suffix, site := range siteNames {
		if host != suffix && !strings.HasSuffix(host, "."+suffix) {
			continue
		}

		if len(suffix) > len(best) {
			best = suffix
			name = site
		}
	}

	return name, name != ""
}

// unsupportedSite returns the name of a site that has no downloader.
func unsupportedSite(host string) (string, bool) {
	host = strings.ToLower(strings.TrimPrefix(host, "www."))

	for suffix, site := range unsupportedSites {
		if host == suffix || strings.HasSuffix(host, "."+suffix) {
			return site, true
		}
	}

	return "", false
}

// The host of a URL, without the scheme or the path.
var hostPattern = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9+.-]*://([^/?#]+)`)

func hostOf(raw string) string {
	m := hostPattern.FindStringSubmatch(strings.TrimSpace(raw))
	if len(m) < 2 {
		return ""
	}

	host := m[1]

	// A port is not part of the name.
	if colon := strings.LastIndex(host, ":"); colon > 0 {
		if !strings.Contains(host[colon+1:], "]") {
			host = host[:colon]
		}
	}

	return host
}

// ExplainUnsupported rewrites yt-dlp's "Unsupported URL" into something that
// says what to do about it.
//
// It only speaks about the case it knows: a link to a site the application
// supports, in a form the extractor cannot read. Anything else is left as it
// is, because a rewrite that guesses is worse than the original.
func ExplainUnsupported(message string) string {
	if !strings.Contains(message, "Unsupported URL") {
		return message
	}

	url := ""
	if i := strings.Index(message, "https://"); i >= 0 {
		url = message[i:]
		url = strings.TrimSpace(url)
	}

	host := hostOf(url)

	// A site with no downloader at all: no link to it would work.
	if name, found := unsupportedSite(host); found {
		return "there is no downloader for " + name +
			" — the application cannot download from that site at all"
	}

	name, known := siteName(host)
	if !known {
		// The site is not one the application works with. Saying so is more
		// use than "unsupported URL", which could equally mean that the
		// link itself is wrong.
		if host == "" {
			return message
		}

		return "there is no downloader for " + host +
			" — the application can only download from the sites it knows"
	}

	return "the link is not one " + name +
		" can be read from — it needs to be the address of a single video, " +
		"not a page that shows one"
}
