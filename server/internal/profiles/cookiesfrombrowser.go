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
func BrowserForCookies() string {
	browser := strings.TrimSpace(config.Instance().Cookies.FromBrowser)

	switch strings.ToLower(browser) {
	case "", "none", "off", "no":
		return ""
	}

	return browser
}
