package downloaders

import (
	"context"
	"testing"
)

// The destination is found in a page that redirects with a script.
//
// Weibo answers a short link with a page that carries the address in a
// variable and sends the browser there, rather than with a redirect the
// client could follow.
func TestDestinationIn(t *testing.T) {
	cases := []struct {
		name string
		page string
		want string
	}{
		{
			name: "the shape Weibo uses",
			page: `var url = "https://weibo.com/tv/show/1034:5358338104557613?from=old_pc_videoshow";`,
			want: "https://weibo.com/tv/show/1034:5358338104557613?from=old_pc_videoshow",
		},
		{
			name: "a location.replace",
			page: `location.replace("https://www.bilibili.com/video/BV1xx");`,
			want: "https://www.bilibili.com/video/BV1xx",
		},
		{
			name: "a location.href",
			page: `window.location.href = "https://vimeo.com/12345";`,
			want: "https://vimeo.com/12345",
		},
		{
			name: "a meta refresh",
			page: `<meta http-equiv="refresh" content="0;url=https://vimeo.com/12345">`,
			want: "https://vimeo.com/12345",
		},
		{
			name: "an escaped ampersand",
			page: `url = "https://weibo.com/x?a=1&amp;b=2";`,
			want: "https://weibo.com/x?a=1&b=2",
		},
		{
			name: "a page with no destination",
			page: `<html><body>nothing here</body></html>`,
			want: "",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := destinationIn(c.page); got != c.want {
				t.Errorf("destinationIn = %q, want %q", got, c.want)
			}
		})
	}
}

// A link that is not a short one is left alone, without a request.
func TestResolveShortLinkLeavesOtherLinks(t *testing.T) {
	cases := []string{
		"https://www.youtube.com/watch?v=dQw4w9WgXcQ",
		"https://www.bilibili.com/video/BV1xx",
		"https://example.com/video/1",
	}

	for _, u := range cases {
		resolved, err := ResolveShortLink(context.Background(), u)
		if err != nil {
			t.Errorf("ResolveShortLink(%q) failed: %v", u, err)
		}
		if resolved != u {
			t.Errorf("ResolveShortLink(%q) = %q, want it unchanged", u, resolved)
		}
	}
}

// The host of a URL, for deciding whether it is a short link.
func TestHostOf(t *testing.T) {
	cases := map[string]string{
		"http://t.cn/abc":               "t.cn",
		"https://www.t.cn/abc":          "t.cn",
		"https://t.cn:443/abc":          "t.cn",
		"https://www.youtube.com/watch": "youtube.com",
		"not a url":                     "",
	}

	for in, want := range cases {
		if got := hostOf(in); got != want {
			t.Errorf("hostOf(%q) = %q, want %q", in, got, want)
		}
	}
}

// The lines that are the machinery talking are not errors.
func TestIsProgressNoise(t *testing.T) {
	noise := []string{
		"[hls @ 0xaaa] Skip ('#EXT-X-PROGRAM-DATE-TIME:2026-10-28T14:31:48.906Z')",
		"[https @ 0xaaa] Opening 'https://example.com/segment' for reading",
		"frame=  298 fps=7.3 q=-1.0 size=    5632KiB time=00:00:15.86 bitrate=2907.9kbits/s speed=0.387x",
		"[in#0/hls @ 0xaaa] corrupt input packet in stream 1",
	}

	for _, line := range noise {
		if !isProgressNoise(line) {
			t.Errorf("isProgressNoise(%q) = false, want true", line)
		}
	}

	errors := []string{
		"ERROR: [youtube] abc: Requested format is not available",
		"ERROR: [Douyin] 123: Unable to download webpage",
	}

	for _, line := range errors {
		if isProgressNoise(line) {
			t.Errorf("isProgressNoise(%q) = true, want false", line)
		}
	}
}

// The noise filter must not swallow a real error that happens to mention the
// machinery.
func TestIsProgressNoiseKeepsRealErrors(t *testing.T) {
	line := "ERROR: [hls @ 0xaaa] something actually went wrong"
	if isProgressNoise(line) {
		t.Error("a line that starts with ERROR was treated as noise")
	}
}
