package downloaders

import "testing"

// The iQiyi short link lands on a share page that carries the video's id in a
// parameter rather than in the address.
//
// The extractor only knows the video's own page, so the id has to be turned
// into one. The page carries two ids that are the same number: plainly, as
// "positiveId", and base64-encoded, as "shareId".
func TestIqiyiURL(t *testing.T) {
	page := `{"tvid":5739997997499700,"videoInfo":{"vid":"4db500640f3f4227b424440e08ce3d63"}}`

	final := "https://m.iqiyi.com/m/playShare?shareId=NTczOTk5Nzk5NzQ5OTcwMA%3D%3D&positiveId=5739997997499700&type=0"

	got := iqiyiURL(page, final)

	want := "https://www.iqiyi.com/v_4db500640f3f4227b424440e08ce3d63.html"
	if got != want {
		t.Errorf("iqiyiURL = %q, want %q", got, want)
	}
}

// The encoded id alone is enough, for a page that does not carry the plain
// one.
func TestIqiyiURLFromTheEncodedID(t *testing.T) {
	final := "https://m.iqiyi.com/m/playShare?shareId=NTczOTk5Nzk5NzQ5OTcwMA%3D%3D"

	got := iqiyiURL("no page data at all", final)

	// Without the video's own id there is nothing to build an address from.
	if got != "" {
		t.Errorf("iqiyiURL = %q, want empty", got)
	}
}

// The hosts that are short links.
func TestShortLinkHosts(t *testing.T) {
	cases := map[string]string{
		"t.cn":          "weibo",
		"weibo.cn":      "weibo",
		"b23.tv":        "bilibili",
		"v.douyin.com":  "douyin",
		"iesdouyin.com": "douyin",
		"qy.net":        "iqiyi",
	}

	for host, site := range cases {
		got, ok := shortLinkHosts[host]
		if !ok {
			t.Errorf("%s is not a short link host", host)
			continue
		}
		if got != site {
			t.Errorf("%s is %q, want %q", host, got, site)
		}
	}
}

// A relative destination is made absolute against the page it was found on.
func TestAbsolute(t *testing.T) {
	cases := []struct {
		candidate string
		base      string
		want      string
	}{
		{
			candidate: "https://weibo.com/tv/show/1034:535",
			base:      "http://t.cn/abc",
			want:      "https://weibo.com/tv/show/1034:535",
		},
		{
			candidate: "/video/BV1xx",
			base:      "https://www.bilibili.com/",
			want:      "https://www.bilibili.com/video/BV1xx",
		},
		{
			candidate: "",
			base:      "https://example.com/",
			want:      "",
		},
	}

	for _, c := range cases {
		if got := absolute(c.candidate, c.base); got != c.want {
			t.Errorf("absolute(%q, %q) = %q, want %q", c.candidate, c.base, got, c.want)
		}
	}
}
