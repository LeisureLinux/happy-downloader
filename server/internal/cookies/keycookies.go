package cookies

import "strings"

// The one cookie per site that decides whether a download works.
//
// A cookie file holds hundreds of cookies per site: page tokens, experiment
// flags, counters, device fingerprints. Almost all of them are disposable —
// the site hands them out again on the next request. A handful are not: they
// are the session, and without them the site answers as a guest, which is
// what "Sign in to confirm you're not a bot" and a 403 are made of.
//
// So the useful question about a cookie export is not how many cookies it
// has. It is whether the session cookie for the site is there and still
// alive. Counting the other ones answers nothing: a site can have seventy
// cookies and still be signed out, and it can have four expired page tokens
// and work perfectly.
//
// The names are the ones each site actually uses, checked against a real
// export for the sites present in it. A site that is not here has no name
// known here, and is not reported at all rather than guessed at.
var keyCookiesBySite = map[string]string{
	"youtube.com":     "LOGIN_INFO",
	"bilibili.com":    "SESSDATA",
	"douyin.com":      "sessionid",
	"tiktok.com":      "sessionid",
	"xiaohongshu.com": "web_session",
	"weibo.com":       "SUB",
	"reddit.com":      "reddit_session",
	"vimeo.com":       "vimeo",
	"instagram.com":   "sessionid",
	"x.com":           "auth_token",
	"twitter.com":     "auth_token",
	"twitch.tv":       "auth-token",
	"nicovideo.jp":    "user_session",
	"facebook.com":    "c_user",
}

// KeyCookieFor is the name of the session cookie a domain's site uses, if the
// site is one whose name is known.
//
// The match is by domain suffix, so "www.douyin.com" and "douyin.com" both
// answer with "sessionid".
func KeyCookieFor(domain string) (string, bool) {
	host := strings.TrimPrefix(strings.ToLower(strings.TrimSpace(domain)), ".")
	host = strings.TrimPrefix(host, "www.")

	// The longest match wins, so a domain under a site's own subdomain still
	// resolves to that site.
	best := ""
	name := ""

	for site, cookie := range keyCookiesBySite {
		if host != site && !strings.HasSuffix(host, "."+site) {
			continue
		}
		if len(site) > len(best) {
			best = site
			name = cookie
		}
	}

	if best == "" {
		return "", false
	}

	return name, true
}
