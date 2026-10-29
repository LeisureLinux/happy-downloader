package downloaders

import "testing"

func TestNameFromURL(t *testing.T) {
	cases := map[string]string{
		"https://www.youtube.com/live/vookt4cfgzI?si=abc": "YouTube vookt4cfgzI",
		"https://www.twitch.tv/sidsicksevan":              "Twitch sidsicksevan",
		"https://live.bilibili.com/1741634790":            "Bilibili 1741634790",
		"https://www.youtube.com/@SomeChannel/live":       "YouTube SomeChannel",
		"https://example.test/whatever":                   "example.test whatever",
		"https://www.youtube.com/watch?v=dQw4w9WgXcQ":     "YouTube",
		"not a url at all":                                "recording",
	}
	for in, want := range cases {
		if got := nameFromURL(in); got != want {
			t.Errorf("%s\n  want %q\n  got  %q", in, want, got)
		}
	}
}
