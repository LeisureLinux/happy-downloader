package downloaders

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LeisureLinux/happy-downloader/v4/server/config"
	"github.com/LeisureLinux/happy-downloader/v4/server/internal/profiles"
)

// yt-dlp's --cookies reads the file and writes its cookie jar back into it,
// so it must receive a copy. Handing over the converted file directly let one
// download replace it with a jar missing the login cookies, which then failed
// as "cookies are no longer valid" on the next run.
func TestStageCookieFileProtectsSourceFromRewrite(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "yt-cookie.json")

	const original = "# Netscape HTTP Cookie File\n.youtube.com\tTRUE\t/\tTRUE\t0\tLOGIN_INFO\tTESTVALUE-NOT-A-REAL-TOKEN\n"
	if err := os.WriteFile(src, []byte(original), 0o600); err != nil {
		t.Fatalf("setup source: %v", err)
	}

	staged := stageCookieFile(src)
	if staged == "" {
		t.Fatal("stageCookieFile returned empty")
	}
	if staged == src {
		t.Fatal("yt-dlp would receive the source file itself")
	}

	// The staging copy holds the same content.
	got, err := os.ReadFile(staged)
	if err != nil {
		t.Fatalf("staging file missing: %v", err)
	}
	if string(got) != original {
		t.Errorf("staging content differs:\ngot:  %q\nwant: %q", got, original)
	}

	// Simulate what yt-dlp does: rewrite the file it was given.
	if err := os.WriteFile(staged, []byte("yt-dlp jar"), 0o600); err != nil {
		t.Fatalf("simulate rewrite: %v", err)
	}

	after, err := os.ReadFile(src)
	if err != nil {
		t.Fatalf("source lost: %v", err)
	}
	if string(after) != original {
		t.Errorf("source was modified:\ngot:  %q\nwant: %q", after, original)
	}
}

// The POT provider fetches the BotGuard challenge from YouTube, so on a host
// that reaches YouTube through a proxy the provider needs it too. Its HTTP
// server has no --proxy flag; the proxy travels per request in the same
// extractor-args value as base_url.
//
// Both come from the configuration now: they are properties of the
// installation, not of a site.
func TestPotArgCarriesProxy(t *testing.T) {
	previous := config.Instance()
	t.Cleanup(func() { config.SetInstance(previous) })

	config.SetInstance(&config.Config{
		YtDlp: config.YtDlpConfig{
			PotBaseURL: "http://127.0.0.1:4416",
			Proxy:      "http://127.0.0.1:8888",
		},
	})

	got := profiles.Profile{
		Name:       "YouTube",
		HostSuffix: "youtube.com",
	}.Argv("https://www.youtube.com/watch?v=x", "", "")

	joined := strings.Join(got, " ")
	want := "youtubepot-bgutilhttp:base_url=http://127.0.0.1:4416;proxy=http://127.0.0.1:8888"
	if !strings.Contains(joined, want) {
		t.Errorf("POT arg missing the provider proxy:\n got: %s\nwant: %s", joined, want)
	}

	// The provider is asked for YouTube only. For another site the argument
	// is dead weight: the extractor ignores it and the provider is not
	// called at all.
	other := strings.Join(
		profiles.Profile{Name: "Bilibili", HostSuffix: "bilibili.com"}.
			Argv("https://www.bilibili.com/video/BV1xx", "", ""),
		" ")

	if strings.Contains(other, "youtubepot") {
		t.Errorf("the POT provider was asked for a site that is not YouTube:\n%s", other)
	}
}

// The queue shows the container of what is being downloaded, before the file
// exists to ask. The probe's answer is the source's *best* format — webm on
// YouTube, because the best video stream is VP9 or AV1 — so a download that
// asked for mp4 still said webm until it landed. The arguments say what will
// actually be produced.
func TestRequestedContainer(t *testing.T) {
	cases := []struct {
		name   string
		params []string
		want   string
	}{
		{"no container chosen", []string{"-f", "bestvideo[height<=1080]+bestaudio/best"}, ""},
		{"merged video", []string{"-f", "bestvideo+bestaudio", "--merge-output-format", "mp4"}, "mp4"},
		{"extracted audio", []string{"-f", "bestaudio/best", "--extract-audio", "--audio-format", "mp3"}, "mp3"},
		{"dot prefix", []string{"--merge-output-format", ".mkv"}, "mkv"},
		// yt-dlp takes a preference list; the first entry is what it tries.
		{"preference list", []string{"--merge-output-format", "mp4,mkv"}, "mp4"},
		// A format selector is a value, not a flag, and must not be read as one.
		{"selector is not a flag", []string{"-f", "bestvideo[height<=720]+bestaudio"}, ""},
		{"last one wins", []string{"--audio-format", "m4a", "--audio-format", "flac"}, "flac"},
		{"missing value", []string{"--merge-output-format"}, ""},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := requestedContainer(c.params); got != c.want {
				t.Errorf("requestedContainer(%v) = %q, want %q", c.params, got, c.want)
			}
		})
	}
}
