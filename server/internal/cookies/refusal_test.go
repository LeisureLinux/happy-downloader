package cookies

import "testing"

// The wording is matched on the part that does not change.
//
// yt-dlp's exact text is "The provided YouTube account cookies are no
// longer valid" — and matching the whole sentence missed it, because the
// site name is in the middle of it.
func TestLooksRefused(t *testing.T) {
	cases := []struct {
		name string
		msg  string
		want bool
	}{
		{
			// The real message, from yt-dlp's YouTube extractor.
			name: "the rotated cookie warning",
			msg: "WARNING: [youtube] The provided YouTube account cookies are no longer valid. " +
				"They have likely been rotated in the browser as a security measure. " +
				"For tips on how to effectively export YouTube cookies, refer to  https://... .",
			want: true,
		},
		{
			// Another site says the same thing with its own name in it.
			name: "another site, same wording",
			msg:  "WARNING: [instagram] The provided Instagram account cookies are no longer valid",
			want: true,
		},
		{
			name: "the bot wall",
			msg:  "ERROR: [youtube] Sign in to confirm you're not a bot.",
			want: true,
		},
		{
			name: "the bot wall, curly apostrophe",
			msg:  "ERROR: [youtube] Sign in to confirm you’re not a bot.",
			want: true,
		},
		{
			name: "an unreadable cookie file",
			msg:  "ERROR: Cookies file must be Netscape formatted, not JSON.",
			want: true,
		},
		{
			name: "a forbidden response",
			msg:  "ERROR: unable to download webpage: HTTP Error 403: Forbidden",
			want: true,
		},
		{
			name: "an ordinary failure",
			msg:  "ERROR: [youtube] dQw4w9WgXcQ: Requested format is not available",
			want: false,
		},
		{
			name: "an ordinary warning",
			msg:  "WARNING: [youtube] n challenge solving failed: Some formats may be missing.",
			want: false,
		},
		{
			name: "a progress line",
			msg:  "[download]  23.2% of   29.01MiB at  208.65KiB/s ETA 01:49",
			want: false,
		},
		{
			name: "nothing at all",
			msg:  "",
			want: false,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := LooksRefused(c.msg); got != c.want {
				t.Errorf("LooksRefused(%q) = %v, want %v", c.msg, got, c.want)
			}
		})
	}
}
