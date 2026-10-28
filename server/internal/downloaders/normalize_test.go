package downloaders

import "testing"

// People paste links from an app rather than from the address bar, and the
// two are different addresses for the same video.
//
// A link the extractor does not recognise is answered with "Unsupported URL",
// which says nothing about which part of it was wrong — so the link is put
// into the form the extractor wants before it is given to yt-dlp.
func TestNormalizeURL(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "a Douyin feed link",
			in:   "https://www.douyin.com/jingxuan?modal_id=7700252144473034035",
			want: "https://www.douyin.com/video/7700252144473034035",
		},
		{
			name: "a Douyin link with tracking parameters",
			in:   "https://www.douyin.com/jingxuan?modal_id=7700252144473034035&share_source=copy_web",
			want: "https://www.douyin.com/video/7700252144473034035",
		},
		{
			name: "a Douyin video link is already right",
			in:   "https://www.douyin.com/video/7700252144473034035",
			want: "https://www.douyin.com/video/7700252144473034035",
		},
		{
			name: "the other query parameter",
			in:   "https://www.douyin.com/share/video/7700252144473034035?aweme_id=7700252144473034035",
			want: "https://www.douyin.com/video/7700252144473034035",
		},
		{
			// A short link is followed by yt-dlp itself, so it is left
			// alone.
			name: "a short link",
			in:   "https://v.douyin.com/iRabc123/",
			want: "https://v.douyin.com/iRabc123/",
		},
		{
			// Anything that is not a plain number is not moved across: the
			// extractor would not accept it either, and guessing would
			// produce a URL that names a different video.
			name: "an id that is not a number",
			in:   "https://www.douyin.com/jingxuan?modal_id=abc123",
			want: "https://www.douyin.com/jingxuan?modal_id=abc123",
		},
		{
			// Other sites are left exactly as they are.
			name: "a YouTube link",
			in:   "https://www.youtube.com/watch?v=dQw4w9WgXcQ",
			want: "https://www.youtube.com/watch?v=dQw4w9WgXcQ",
		},
		{
			name: "a Bilibili link",
			in:   "https://www.bilibili.com/video/BV1gjxu6CED1/",
			want: "https://www.bilibili.com/video/BV1gjxu6CED1/",
		},
		{
			name: "a Kuaishou link is left alone",
			in:   "https://www.kuaishou.com/short-video/3xsvu5dtg9jqhzi?authorId=3xxx",
			want: "https://www.kuaishou.com/short-video/3xsvu5dtg9jqhzi?authorId=3xxx",
		},
		{
			name: "not a URL at all",
			in:   "hello",
			want: "hello",
		},
		{
			name: "empty",
			in:   "",
			want: "",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := NormalizeURL(c.in); got != c.want {
				t.Errorf("NormalizeURL(%q) = %q, want %q", c.in, got, c.want)
			}
		})
	}
}
