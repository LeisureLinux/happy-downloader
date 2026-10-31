package profiles

import (
	"os"
	"path/filepath"
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

	for _, c := range cases {
		if got := normalizeBrowser(c.set); got != c.want {
			t.Errorf("FromBrowser = %q: got %q, want %q", c.set, got, c.want)
		}
	}
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

// A browser that is not installed must not be passed to yt-dlp.
//
// yt-dlp exits at once when it cannot find the cookie database, before it has
// looked at the URL, so a name in a configuration file on the wrong machine
// stops every download — and the error says nothing about cookies.
func TestBrowserInstalledNeedsAProfile(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("APPDATA", home)
	t.Setenv("LOCALAPPDATA", home)

	for _, browser := range []string{"firefox", "chrome", "chromium", "edge", "brave", "opera", "vivaldi"} {
		if BrowserInstalled(browser) {
			t.Errorf("%s is reported as installed in an empty home directory", browser)
		}
	}

	// A profile after a colon is not part of the question.
	if BrowserInstalled("firefox:default-release") {
		t.Error("a named profile was reported as installed")
	}
}

// A browser this code does not know how to look for is tried anyway: yt-dlp may
// support it, and refusing would be a worse answer than attempting.
func TestBrowserInstalledTriesWhatItDoesNotKnow(t *testing.T) {
	if !BrowserInstalled("safari") {
		t.Error("a browser with no known location was refused")
	}
}

// And a profile that does exist is found.
func TestBrowserInstalledFindsAProfile(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	dir := filepath.Join(home, ".mozilla", "firefox", "abc.default")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	if !BrowserInstalled("firefox") {
		t.Error("firefox was not found where it keeps its profiles")
	}
}
