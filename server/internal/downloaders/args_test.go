package downloaders

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/marcopiovanello/yt-dlp-web-ui/v4/server/config"
)

// yt-dlp's --cookies reads the file and writes its cookie jar back into
// it. Passing the upload straight through lets a download replace it, which
// drops the login cookies the upload was made for and shows up on the next run
// as "cookies are no longer valid".
func TestResolveCookiePathProtectsUploadFromRewrite(t *testing.T) {
	dir := t.TempDir()
	upload := filepath.Join(dir, "cookies.txt")

	const original = "# Netscape HTTP Cookie File\n.youtube.com\tTRUE\t/\tTRUE\t0\tLOGIN_INFO\tTESTVALUE-NOT-A-REAL-TOKEN\n"
	if err := os.WriteFile(upload, []byte(original), 0o600); err != nil {
		t.Fatalf("setup: %v", err)
	}

	// Point the config at this directory the way the service does.
	config.Instance().Paths.LocalDatabasePath = dir
	config.Instance().Paths.CookiePath = ""

	got := resolveCookiePath()
	if got == "" {
		t.Fatal("resolveCookiePath returned empty with a cookie file present")
	}
	if got == upload {
		c, _ := os.ReadFile(upload)
		t.Fatalf("uploaded file passed to yt-dlp directly; yt-dlp would rewrite it:\n%s", c)
	}

	// The staging copy must hold the same content, so yt-dlp sees the login
	// cookies rather than an empty jar.
	staged, err := os.ReadFile(got)
	if err != nil {
		t.Fatalf("staging file missing: %v", err)
	}
	if string(staged) != original {
		t.Errorf("staging content differs from upload:\ngot:  %q\nwant: %q", staged, original)
	}

	// Simulate what yt-dlp does: rewrite the file it was given.
	if err := os.WriteFile(got, []byte("yt-dlp jar"), 0o600); err != nil {
		t.Fatalf("simulate rewrite: %v", err)
	}

	// The upload must be untouched.
	after, err := os.ReadFile(upload)
	if err != nil {
		t.Fatalf("upload lost: %v", err)
	}
	if string(after) != original {
		t.Errorf("upload was modified:\ngot:  %q\nwant: %q", after, original)
	}
}

func TestSanitizeRuntime(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  string
	}{
		{"bare path gets node prefix", "/usr/bin/node", "node:/usr/bin/node"},
		{"already qualified node", "node:/usr/bin/node", "node:/usr/bin/node"},
		{"deno qualified", "deno:/opt/deno", "deno:/opt/deno"},
		{"runtime name only", "node", "node"},
		{"quickjs", "quickjs", "quickjs"},
		{"empty stays empty", "", ""},
		{"whitespace trimmed", "  /usr/bin/node  ", "node:/usr/bin/node"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := SanitizeRuntime(c.input); got != c.want {
				t.Errorf("SanitizeRuntime(%q) = %q, want %q", c.input, got, c.want)
			}
		})
	}
}

// A bare path must not reach yt-dlp: it is rejected outright, which silently
// disables the JS challenge solver and only shows up later as a bot check.
func TestSanitizeRuntimeNeverReturnsBarePath(t *testing.T) {
	for _, in := range []string{"/usr/bin/node", "/home/u/.nvm/versions/node/v24/bin/node", "node"} {
		got := SanitizeRuntime(in)
		if got == "" {
			continue
		}
		if len(got) > 0 && got[0] == '/' {
			t.Errorf("SanitizeRuntime(%q) returned a bare path %q", in, got)
		}
	}
}

func TestBuildArgsOmitsRuntimeWhenUnset(t *testing.T) {
	// config.Instance() has no js_runtime_path by default
	args := BuildArgs("https://example.com", "-J")

	for _, a := range args {
		if a == "--js-runtimes" {
			t.Fatalf("--js-runtimes present with no configured runtime: %v", args)
		}
	}

	if len(args) < 2 || args[0] != "https://example.com" || args[len(args)-1] != "-J" {
		t.Errorf("unexpected argv: %v", args)
	}
}

// The URL must survive verbatim. generic.go used to cut everything from "?list"
// onwards, which also removed ordinary query parameters.
func TestBuildArgsPreservesURLQuery(t *testing.T) {
	urls := []string{
		"https://youtu.be/abc123?si=ZRUDnXwuGPKFT5X7",
		"https://www.youtube.com/watch?v=abc123&pp=ugUEEgJ6aA%3D%3D",
		"https://example.com/video?list=PL123&index=4",
	}

	for _, u := range urls {
		args := BuildArgs(u, "-J")
		if args[0] != u {
			t.Errorf("URL mangled: got %q, want %q", args[0], u)
		}
		if len(args) < 2 || args[len(args)-1] != "-J" {
			t.Errorf("extra args lost for %q: %v", u, args)
		}
	}
}
