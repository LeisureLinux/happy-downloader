package filebrowser

import "testing"

// The site name is what the icon in the file browser is keyed by, so it has
// to be the same for every URL that means the same site.
func TestSiteOf(t *testing.T) {
	cases := []struct {
		url  string
		want string
	}{
		{"https://www.youtube.com/watch?v=abc", "youtube"},
		{"https://youtube.com/watch?v=abc", "youtube"},
		{"https://m.youtube.com/watch?v=abc", "youtube"},
		{"https://music.youtube.com/watch?v=abc", "youtube"},
		{"https://youtu.be/abc", "youtube"},
		{"https://www.bilibili.com/video/BV1", "bilibili"},
		{"https://b23.tv/abc", "bilibili"},
		{"https://x.com/user/status/1", "x"},
		{"https://twitter.com/user/status/1", "x"},
		{"https://www.xiaohongshu.com/explore/1", "xiaohongshu"},
		{"https://xhslink.com/abc", "xiaohongshu"},
		{"https://www.douyin.com/video/1", "douyin"},
		{"https://www.tiktok.com/@user/video/1", "tiktok"},
		{"https://www.reddit.com/r/x/comments/1", "reddit"},
		{"https://www.instagram.com/p/abc", "instagram"},
		{"https://vimeo.com/12345", "vimeo"},
		{"https://www.twitch.tv/videos/1", "twitch"},
		{"https://www.nicovideo.jp/watch/sm9", "niconico"},
		// A site nobody has an icon for still says where it came from.
		{"https://example.co.uk/watch/1", "example"},
		{"https://news.example.org/a", "news"},
		// Nothing to derive a site from.
		{"", ""},
		{"not a url", ""},
	}

	for _, c := range cases {
		t.Run(c.url, func(t *testing.T) {
			if got := SiteOf(c.url); got != c.want {
				t.Errorf("SiteOf(%q) = %q, want %q", c.url, got, c.want)
			}
		})
	}
}
