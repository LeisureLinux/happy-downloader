package downloaders

import "testing"

// The progress of a livestream is read out of what ffmpeg writes.
//
// A recording has no percentage to speak of — it is as long as the stream
// lasts — so what is reported is how much has been recorded, against the
// length that was asked for.
func TestRecordedSeconds(t *testing.T) {
	cases := []struct {
		line string
		want float64
	}{
		{
			line: "frame=  3600 fps= 60 q=-1.0 size=  65536KiB time=00:01:00.00 bitrate=8192.0kbits/s speed=1.0x",
			want: 60,
		},
		{
			line: "frame=  100 fps= 30 q=-1.0 size=   1024KiB time=00:00:03.50 bitrate=2400.0kbits/s speed=1.0x",
			want: 3.5,
		},
		{
			line: "frame=    1 fps= 0.0 q=-1.0 size=       0KiB time=01:02:03.00 bitrate=  10.0kbits/s speed=0.0x",
			want: 3723,
		},
		{
			// Not a progress line.
			line: "[hls @ 0xaaa] Skip ('#EXT-X-PROGRAM-DATE-TIME:2026-10-28T14:31:48.906Z')",
			want: 0,
		},
	}

	for _, c := range cases {
		got, ok := recordedSeconds(c.line)
		if !ok {
			if c.want != 0 {
				t.Errorf("recordedSeconds(%q) found nothing, want %v", c.line, c.want)
			}
			continue
		}
		if got != c.want {
			t.Errorf("recordedSeconds(%q) = %v, want %v", c.line, got, c.want)
		}
	}
}

// The speed is shown in megabytes per second, and ffmpeg writes kilobits.
func TestBitrateOf(t *testing.T) {
	line := "frame=  3600 fps= 60 q=-1.0 size=  65536KiB time=00:01:00.00 bitrate=8192.0kbits/s speed=1.0x"

	got := bitrateOf(line)

	// 8192 kilobits is 1024000 bytes a second.
	if got < 1020000 || got > 1030000 {
		t.Errorf("bitrateOf = %v, want about 1024000", got)
	}
}
