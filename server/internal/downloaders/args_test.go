package downloaders

import "testing"

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
