package profiles

import (
	"testing"

	"github.com/LeisureLinux/happy-downloader/v4/server/config"
)

// The browser is only used when one is named; otherwise the export file is
// what yt-dlp is given.
func TestBrowserForCookies(t *testing.T) {
	cases := []struct {
		set  string
		want string
	}{
		{"", ""},
		{"none", ""},
		{"off", ""},
		{"No", ""},
		{"chrome", "chrome"},
		{"Edge", "Edge"},
		{"  firefox  ", "firefox"},
		// A profile is part of the value and is passed through: rewriting it
		// here would only mean translating it back.
		{"chrome:Profile 1", "chrome:Profile 1"},
	}

	instance := config.Instance()

	for _, c := range cases {
		instance.Cookies.FromBrowser = c.set

		if got := BrowserForCookies(); got != c.want {
			t.Errorf("FromBrowser = %q: got %q, want %q", c.set, got, c.want)
		}
	}

	instance.Cookies.FromBrowser = ""
}

// The two ways of supplying credentials must not both be passed: yt-dlp
// refuses --cookies and --cookies-from-browser together.
func TestArgvUsesOneCookieSource(t *testing.T) {
	instance := config.Instance()
	previous := instance.Cookies.FromBrowser
	defer func() { instance.Cookies.FromBrowser = previous }()

	var profile Profile

	instance.Cookies.FromBrowser = "chrome"

	withBrowser := profile.Argv("https://example.com/v", "/tmp/cookies.txt", "")

	if !contains(withBrowser, "--cookies-from-browser") {
		t.Fatalf("no --cookies-from-browser in %v", withBrowser)
	}

	if contains(withBrowser, "--cookies") {
		t.Fatalf("the file was passed as well: %v", withBrowser)
	}

	instance.Cookies.FromBrowser = ""

	withFile := profile.Argv("https://example.com/v", "/tmp/cookies.txt", "")

	if !contains(withFile, "--cookies") || contains(withFile, "--cookies-from-browser") {
		t.Fatalf("the file was not used on its own: %v", withFile)
	}
}

func contains(args []string, flag string) bool {
	for _, a := range args {
		if a == flag {
			return true
		}
	}

	return false
}
