package profiles

import (
	"strings"

	"github.com/LeisureLinux/happy-downloader/v4/server/config"
)

// BrowserForCookies is the browser yt-dlp is told to read cookies from, or an
// empty string when it should be given a file instead.
//
// The accepted values are yt-dlp's own: chrome, chromium, edge, firefox,
// brave, opera, vivaldi, safari — optionally with a profile, written
// "chrome:Profile 1" or "chrome::/path/to/profile". The value is passed
// through unchanged, because renaming a browser here would only mean having
// to translate it back, and a name yt-dlp does not know produces an error
// that says which names it does.
//
// "none" and an empty value both mean "use the export file", so that a
// configuration file can say so out loud.
//
// A browser that is not on this machine is treated the same way, and that is
// the difference between a setting and an outage. yt-dlp exits with status 1
// the moment it cannot find the cookie database, before it has looked at the
// URL, so a name typed into a configuration file on the wrong machine stops
// every download — with an error that says nothing about cookies.
//
// The check is a stat of the profile directory, not a run of yt-dlp: it has to
// be cheap enough to do for every command line, and its job is only to catch the
// case where the browser is simply not installed. Whether the cookies can be
// decrypted is a different question, answered by the probe in the interface.
func normalizeBrowser(setting string) string {
	browser := strings.TrimSpace(setting)

	switch strings.ToLower(browser) {
	case "", "none", "off", "no":
		return ""
	}

	return browser
}

func BrowserForCookies() string {
	browser := normalizeBrowser(config.Instance().Cookies.FromBrowser)
	if browser == "" {
		return ""
	}

	if !BrowserInstalled(browser) {
		warnOnce(browser,
			"the configured browser has no cookie database on this machine; "+
				"downloads use the cookie file instead")

		return ""
	}

	return browser
}
