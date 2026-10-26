package profiles

import (
	"strings"
	"testing"
)

func TestHostOf(t *testing.T) {
	cases := []struct{ in, want string }{
		{"https://www.youtube.com/watch?v=x", "youtube.com"},
		{"https://m.youtube.com/watch?v=x", "m.youtube.com"},
		{"https://youtu.be/x", "youtu.be"},
		{"https://www.bilibili.com/video/BV1xx", "bilibili.com"},
		{"https://x.com/user/status/1", "x.com"},
		{"https://twitter.com/user/status/1", "twitter.com"},
		{"youtu.be/x", "youtu.be"}, // scheme-less
		{"", ""},
		{"   ", ""},
		{"not a url at all", ""}, // junk parses to no host
	}

	for _, c := range cases {
		if got := hostOf(c.in); got != c.want {
			t.Errorf("hostOf(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestMatchLongestSuffixWins(t *testing.T) {
	tab := table{
		{Name: "YouTube", HostSuffix: "youtube.com"},
		{Name: "Music", HostSuffix: "music.youtube.com"},
		{Name: "Fallback"},
	}

	// music.youtube.com must match the more specific entry.
	if p, _ := tab.match("https://music.youtube.com/watch?v=x"); p.Name != "Music" {
		t.Errorf("music.youtube.com matched %q, want Music", p.Name)
	}

	// A plain youtube URL must not match the music entry.
	if p, _ := tab.match("https://www.youtube.com/watch?v=x"); p.Name != "YouTube" {
		t.Errorf("youtube.com matched %q, want YouTube", p.Name)
	}

	// notyoutube.com shares the suffix string but not the host, and must
	// fall through to the fallback.
	if p, _ := tab.match("https://notyoutube.com/watch?v=x"); p.Name != "Fallback" {
		t.Errorf("notyoutube.com matched %q, want Fallback", p.Name)
	}

	// Unrecognised host goes to the fallback.
	if p, _ := tab.match("https://vimeo.com/12345"); p.Name != "Fallback" {
		t.Errorf("vimeo.com matched %q, want Fallback", p.Name)
	}

	// youtu.be is a different host from youtube.com: it also falls back,
	// which is deliberate — short-link domains get their own entry.
	if p, _ := tab.match("https://youtu.be/x"); p.Name != "Fallback" {
		t.Errorf("youtu.be matched %q, want Fallback", p.Name)
	}
}

// The argv must be complete: --ignore-config means nothing outside this list
// reaches yt-dlp, so a missing flag is a behavioural change, not a fallback.
func TestArgvIsSelfContained(t *testing.T) {
	p := Profile{
		Name:              "YouTube",
		HostSuffix:        "youtube.com",
		CookiePath:        "/data/yt-cookie.json",
		PotBaseURL:        "http://127.0.0.1:4416",
		Proxy:             "http://wpad.lan:8888",
		ExtraArgs:         []string{"--sub-langs", "en,zh-Hans"},
		PreferFreeFormats: true,
	}

	got := p.Argv("/tmp/cookies.txt.ytdlp-jar", "node:/usr/bin/node")
	gotStr := strings.Join(got, " ")

	want := []string{
		"--ignore-config",
		"--js-runtimes", "node:/usr/bin/node",
		"--cookies", "/tmp/cookies.txt.ytdlp-jar",
		"--extractor-args", "youtubepot-bgutilhttp:base_url=http://127.0.0.1:4416",
		"--proxy", "http://wpad.lan:8888",
		"--prefer-free-formats",
		"--sub-langs", "en,zh-Hans",
	}

	for _, w := range want {
		if !strings.Contains(gotStr, w) {
			t.Errorf("argv missing %q:\n%s", w, gotStr)
		}
	}

	if got[0] != "--ignore-config" {
		t.Errorf("argv[0] = %q, want --ignore-config (must come before the URL is appended by the caller)", got[0])
	}
}

// Empty fields must not produce empty arguments: yt-dlp warns or errors on
// those instead of ignoring them.
func TestArgvOmitsEmptyValues(t *testing.T) {
	p := Profile{Name: "Bilibili", HostSuffix: "bilibili.com"}

	got := p.Argv("", "")

	gotStr := strings.Join(got, " ")

	for _, bad := range []string{"--cookies", "--proxy", "--js-runtimes", "--extractor-args"} {
		if strings.Contains(gotStr, bad) {
			t.Errorf("argv for an empty profile contains %q:\n%s", bad, gotStr)
		}
	}

	if len(got) != 1 || got[0] != "--ignore-config" {
		t.Errorf("argv = %v, want just [--ignore-config]", got)
	}
}

// Blank extra-arg rows (an artefact of the UI table) must not reach yt-dlp.
func TestArgvSkipsBlankExtraArgs(t *testing.T) {
	p := Profile{ExtraArgs: []string{"", "   ", "--yes-video"}}

	got := p.Argv("", "")

	if len(got) != 2 || got[1] != "--yes-video" {
		t.Errorf("argv = %v, want [--ignore-config --yes-video]", got)
	}
}

func TestSortedCopyKeepsFallbackLast(t *testing.T) {
	tab := table{
		{Name: "Fallback"},
		{Name: "Bilibili", HostSuffix: "bilibili.com"},
		{Name: "x.com", HostSuffix: "x.com"},
	}

	out := tab.sortedCopy()

	if out[len(out)-1].HostSuffix != "" {
		t.Errorf("fallback not last: %+v", out)
	}
}

// A site commonly serves the same content from several domains. YouTube
// answers on youtube.com and youtu.be, and pasting a short link is the normal
// case, so one profile has to cover both.
func TestMatchCoversSeveralDomainsPerProfile(t *testing.T) {
	tab := table{
		{Name: "YouTube", HostSuffix: "youtube.com,youtu.be"},
		{Name: "Fallback"},
	}

	for _, u := range []string{
		"https://www.youtube.com/watch?v=x",
		"https://youtube.com/watch?v=x",
		"https://m.youtube.com/watch?v=x",
		"https://youtu.be/x",
		"https://youtu.be/x?si=abc",
	} {
		if p, _ := tab.match(u); p.Name != "YouTube" {
			t.Errorf("%s matched %q, want YouTube", u, p.Name)
		}
	}

	// A host that merely ends in the same letters must not match.
	if p, _ := tab.match("https://notyoutube.com/x"); p.Name != "Fallback" {
		t.Errorf("notyoutube.com matched %q, want Fallback", p.Name)
	}
}

func TestSuffixesTrimsAndSplits(t *testing.T) {
	p := Profile{HostSuffix: " youtube.com , youtu.be , , www.m.youtube.com "}

	got := p.suffixes()
	want := []string{"youtube.com", "youtu.be", "m.youtube.com"}

	if len(got) != len(want) {
		t.Fatalf("suffixes = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("suffixes[%d] = %q, want %q", i, got[i], want[i])
		}
	}

	if (Profile{}).suffixes() != nil {
		t.Error("empty HostSuffix should yield no suffixes")
	}
}
