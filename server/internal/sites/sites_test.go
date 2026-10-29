package sites

import "testing"

// TestForHostIsTheOnlyMatch is the point of the package: one answer to "which
// site is this host", used by the file browser, the cookie rules and the
// messages that name a site.
func TestForHostIsTheOnlyMatch(t *testing.T) {
	cases := map[string]string{
		"youtube.com":            "youtube",
		"www.youtube.com":        "youtube",
		"music.youtube.com":      "youtube",
		"youtu.be":               "youtube",
		"live.bilibili.com":      "bilibili",
		"b23.tv":                 "bilibili",
		"live.bilibili.com:443":  "", // a port is not part of a host here
		"packaged-media.redd.it": "reddit",
		"reddit.com":             "reddit",
		"t.cn":                   "weibo",
		"weibo.com":              "weibo",
		"www.douyin.com":         "douyin",
		"v.douyin.com":           "douyin",
		"x.com":                  "x",
		"twitter.com":            "x",
		"www.twitch.tv":          "twitch",
		"nicovideo.jp":           "niconico",
		"fb.watch":               "facebook",
		"example.test":           "",
		"not-a-site":             "",
		"":                       "",
	}

	for host, want := range cases {
		got := ""
		if site, ok := ForHost(host); ok {
			got = site.Key
		}

		if got != want {
			t.Errorf("%q: want %q, got %q", host, want, got)
		}
	}
}

// TestEverySiteHasWhatTheInterfaceNeeds guards the table itself.
//
// The interface draws every entry and looks its mark up by key, and the cookie
// dialog shows the session cookie's name. An entry missing either is a blank
// where a row should be.
func TestEverySiteHasWhatTheInterfaceNeeds(t *testing.T) {
	seen := map[string]bool{}

	for _, s := range All() {
		if s.Key == "" {
			t.Errorf("%q has no key", s.Name)
		}
		if seen[s.Key] {
			t.Errorf("%q appears twice", s.Key)
		}
		seen[s.Key] = true

		if s.Name == "" {
			t.Errorf("%q has no display name", s.Key)
		}
		if len(s.Hosts) == 0 {
			t.Errorf("%q has no hosts, so nothing can resolve to it", s.Key)
		}
		if s.KeyCookie == "" {
			t.Errorf("%q has no session cookie, so the dialog shows nothing for it", s.Key)
		}
	}
}

// TestKeyCookieBelongsToTheSiteThatOwnsIt is the bug that started this: two
// lists, two answers.
func TestKeyCookieBelongsToTheSiteThatOwnsIt(t *testing.T) {
	cases := map[string]string{
		"youtube.com":  "LOGIN_INFO",
		"bilibili.com": "SESSDATA",
		"douyin.com":   "sessionid",
		"twitch.tv":    "auth-token",
		"facebook.com": "c_user",
	}

	for host, want := range cases {
		site, ok := ForHost(host)
		if !ok {
			t.Fatalf("%q did not resolve", host)
		}
		if site.KeyCookie != want {
			t.Errorf("%q: want %q, got %q", host, want, site.KeyCookie)
		}
	}
}
