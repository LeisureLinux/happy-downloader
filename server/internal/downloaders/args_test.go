package downloaders

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

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
func TestPotArgCarriesProxy(t *testing.T) {
	got := profiles.Profile{
		Name:       "YouTube",
		HostSuffix: "youtube.com",
		PotBaseURL: "http://127.0.0.1:4416",
		Proxy:      "http://127.0.0.1:8888",
	}.Argv("", "")

	joined := strings.Join(got, " ")
	want := "youtubepot-bgutilhttp:base_url=http://127.0.0.1:4416;proxy=http://127.0.0.1:8888"
	if !strings.Contains(joined, want) {
		t.Errorf("POT arg missing the provider proxy:\n got: %s\nwant: %s", joined, want)
	}
}
