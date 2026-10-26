package downloaders

import "testing"

func TestSanitizeProgressJSON(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{
			"audio download has no dimensions",
			`{"filepath":"/tmp/a.mp3","width":NA,"height":NA}`,
			`{"filepath":"/tmp/a.mp3","width":null,"height":null}`,
		},
		{
			"unknown speed and eta",
			`{"eta":NA,"percentage":"100.0%","speed":NA}`,
			`{"eta":null,"percentage":"100.0%","speed":null}`,
		},
		{
			"real numbers pass through",
			`{"eta":12,"percentage":"50.0%","speed":1048576}`,
			`{"eta":12,"percentage":"50.0%","speed":1048576}`,
		},
		{
			// "NA" inside a quoted string is data, not a missing number.
			"quoted NA untouched",
			`{"filepath":"/tmp/NA song.mp3","width":NA}`,
			`{"filepath":"/tmp/NA song.mp3","width":null}`,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := string(sanitizeProgressJSON([]byte(c.in))); got != c.want {
				t.Errorf("got  %s\nwant %s", got, c.want)
			}
		})
	}
}
