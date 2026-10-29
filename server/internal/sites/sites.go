// Package sites is the one place that knows which video sites this
// application recognises.
//
// It used to be five places: a list in the dialog, a switch of icons, a map
// from host to name in the file browser, a map of session cookies, and a set
// of per-site arguments. They disagreed. Facebook had an icon and a session
// cookie but was missing from the dialog; a recording of a YouTube stream was
// named "Twitch" because a different five-line list had its own idea of what a
// fallback was called.
//
// A site is an identity — a key, a name, the hosts that mean it, the cookie
// that decides whether it works — and everything that needs to know a site
// reads it from here.
//
// Note what this is not: it is not a list of what can be downloaded. yt-dlp
// has an extractor for about 1755 sites and a generic one that reads embedded
// media from any page, so a site that is not here still downloads. What is
// missing is the things this application adds on top — the icon, the brand
// colour, the session cookie it knows the name of.
package sites

import "strings"

// Site is one video site.
type Site struct {
	// Key is the short name the interface keys everything by: "youtube",
	// "bilibili". It is what an icon and a colour are found by.
	Key string `json:"key"`
	// Name is how the site is written in a sentence.
	Name string `json:"name"`
	// Hosts are the domains that mean this site. A subdomain counts, so
	// "music.youtube.com" is YouTube; the delivery hosts are listed too,
	// because a file downloaded from one of them came from this site.
	Hosts []string `json:"hosts"`
	// KeyCookie is the one cookie that decides whether a download from this
	// site works. A site with no name here still downloads; nothing is
	// guessed about its cookies.
	KeyCookie string `json:"keyCookie,omitempty"`
	// LoginCookies are the cookies that make a cookie file a signed-in
	// session rather than a guest one, for the sites where that is knowable.
	//
	// It is a stricter set than KeyCookie: one cookie tells the interface
	// whether the site works, and the import refuses a file that is missing
	// any of these. A site without a list here has no rule, and its cookies
	// are accepted as they are — a browser that has merely visited a site has
	// cookies for it that are not a session.
	LoginCookies []string `json:"loginCookies,omitempty"`
}

// catalogue is the list, in the order the interface shows it: the sites most
// likely to be used come first.
var catalogue = []Site{
	{
		Key:       "youtube",
		Name:      "YouTube",
		Hosts:     []string{"youtube.com", "youtu.be"},
		KeyCookie: "LOGIN_INFO",
		// YouTube serves a file carrying the __Secure-* family but not
		// LOGIN_INFO as a guest, which is what turns into "Sign in to
		// confirm you're not a bot" on videos that need an account.
		LoginCookies: []string{"LOGIN_INFO", "HSID", "SID", "__Secure-3PSID"},
	},
	{
		Key:       "bilibili",
		Name:      "Bilibili",
		Hosts:     []string{"bilibili.com", "b23.tv"},
		KeyCookie: "SESSDATA",
	},
	{
		Key:       "douyin",
		Name:      "Douyin",
		Hosts:     []string{"douyin.com", "iesdouyin.com"},
		KeyCookie: "sessionid",
	},
	{
		Key:       "tiktok",
		Name:      "TikTok",
		Hosts:     []string{"tiktok.com"},
		KeyCookie: "sessionid",
	},
	{
		Key:       "xiaohongshu",
		Name:      "Xiaohongshu",
		Hosts:     []string{"xiaohongshu.com", "xhslink.com"},
		KeyCookie: "web_session",
	},
	{
		Key:  "weibo",
		Name: "Weibo",
		// "t.cn" is the shortener the app hands out, and a link pasted from
		// it is a Weibo link.
		Hosts:     []string{"weibo.com", "weibo.cn", "t.cn"},
		KeyCookie: "SUB",
	},
	{
		Key:       "x",
		Name:      "X",
		Hosts:     []string{"x.com", "twitter.com"},
		KeyCookie: "auth_token",
	},
	{
		Key:       "instagram",
		Name:      "Instagram",
		Hosts:     []string{"instagram.com"},
		KeyCookie: "sessionid",
	},
	{
		Key:  "reddit",
		Name: "Reddit",
		// Reddit serves its video from a delivery host rather than from
		// reddit.com, so a file downloaded from it is a Reddit file.
		Hosts:     []string{"reddit.com", "redd.it"},
		KeyCookie: "reddit_session",
	},
	{
		Key:       "vimeo",
		Name:      "Vimeo",
		Hosts:     []string{"vimeo.com"},
		KeyCookie: "vimeo",
	},
	{
		Key:       "twitch",
		Name:      "Twitch",
		Hosts:     []string{"twitch.tv"},
		KeyCookie: "auth-token",
	},
	{
		Key:       "niconico",
		Name:      "Niconico",
		Hosts:     []string{"nicovideo.jp"},
		KeyCookie: "user_session",
	},
	{
		Key:       "facebook",
		Name:      "Facebook",
		Hosts:     []string{"facebook.com", "fb.watch"},
		KeyCookie: "c_user",
	},
}

// All returns every site, in display order.
func All() []Site {
	out := make([]Site, len(catalogue))
	copy(out, catalogue)
	return out
}

// ByKey finds a site by its key.
func ByKey(key string) (Site, bool) {
	for _, s := range catalogue {
		if s.Key == key {
			return s, true
		}
	}
	return Site{}, false
}

// ForHost finds the site a host belongs to.
//
// The match is by suffix, so a subdomain belongs to its site: "music.youtube.com"
// is YouTube and "packaged-media.redd.it" is Reddit. The longest match wins, so
// a host listed under two sites resolves to the more specific one.
func ForHost(rawHost string) (Site, bool) {
	host := strings.ToLower(strings.TrimSpace(rawHost))
	host = strings.TrimPrefix(host, "www.")
	host = strings.Trim(host, ".")

	if host == "" {
		return Site{}, false
	}

	best := Site{}
	longest := 0

	for _, s := range catalogue {
		for _, candidate := range s.Hosts {
			if host != candidate && !strings.HasSuffix(host, "."+candidate) {
				continue
			}

			if len(candidate) > longest {
				best = s
				longest = len(candidate)
			}
		}
	}

	if longest == 0 {
		return Site{}, false
	}

	return best, true
}

// NameFor is the human name of a site key, or the key itself when it is not
// one this application knows.
func NameFor(key string) string {
	if s, ok := ByKey(key); ok {
		return s.Name
	}
	return key
}
