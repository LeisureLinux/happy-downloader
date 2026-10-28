package metadata

import "testing"

// yt-dlp says "Unsupported URL" and stops there. The person who pasted the
// link cannot tell whether the link is wrong, the site is unknown, or the
// application is at fault — so the message is rewritten into one that says.
func TestExplainUnsupported(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "a site the application does not know",
			in:   "ERROR: Unsupported URL: https://www.kuaishou.com/short-video/3xsvu5dtg9jqhzi",
			want: "there is no downloader for Kuaishou — the application cannot download from that site at all",
		},
		{
			name: "a known site, a link that is not a video",
			in:   "ERROR: Unsupported URL: https://www.douyin.com/jingxuan?modal_id=7700252144473034035",
			want: "the link is not one Douyin can be read from — it needs to be the address of a single video, not a page that shows one",
		},
		{
			name: "a subdomain still finds the site",
			in:   "ERROR: Unsupported URL: https://m.kuaishou.com/short-video/x",
			want: "there is no downloader for Kuaishou — the application cannot download from that site at all",
		},
		{
			// A message that is not about an unsupported URL is left alone.
			name: "another failure",
			in:   "ERROR: [youtube] dQw4w9WgXcQ: Requested format is not available",
			want: "ERROR: [youtube] dQw4w9WgXcQ: Requested format is not available",
		},
		{
			name: "no URL to name",
			in:   "ERROR: Unsupported URL",
			want: "ERROR: Unsupported URL",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := ExplainUnsupported(c.in); got != c.want {
				t.Errorf("ExplainUnsupported(%q) =\n  %q\nwant\n  %q", c.in, got, c.want)
			}
		})
	}
}

func TestSiteName(t *testing.T) {
	cases := map[string]string{
		"youtube.com":         "YouTube",
		"m.youtube.com":       "YouTube",
		"youtu.be":            "YouTube",
		"www.bilibili.com":    "Bilibili",
		"douyin.com":          "Douyin",
		"v.douyin.com":        "Douyin",
		"xiaohongshu.com":     "Xiaohongshu",
		"example.com":         "",
		"notyoutube.com":      "",
		"youtube.com.evil.io": "",
	}

	for host, want := range cases {
		got, _ := siteName(host)
		if got != want {
			t.Errorf("siteName(%q) = %q, want %q", host, got, want)
		}
	}
}
