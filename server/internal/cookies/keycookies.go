package cookies

import (
	"github.com/LeisureLinux/happy-downloader/v4/server/internal/sites"
)

// The session cookie a site uses comes from the catalogue in internal/sites.
//
// It is the one cookie per site that decides whether a download works. A
// cookie file holds hundreds per site — page tokens, experiment flags,
// counters, device fingerprints — and almost all of them are disposable. The
// few that are not are the session, and without them the site answers as a
// guest, which is what "Sign in to confirm you're not a bot" is made of.
//
// The names are not guessed. A site the catalogue has no name for is left
// alone rather than reported with a made-up one.

// KeyCookieFor is the name of the session cookie a domain's site uses, if the
// site is one whose name is known.
//
// The match is by domain suffix, so "www.douyin.com" and "douyin.com" both
// answer with "sessionid".
func KeyCookieFor(domain string) (string, bool) {
	site, ok := sites.ForHost(domain)
	if !ok || site.KeyCookie == "" {
		return "", false
	}

	return site.KeyCookie, true
}
