package cookies

import (
	"testing"
	"time"
)

func TestNormalizeExpiry(t *testing.T) {
	// 2027-12-03T00:55:09Z in four different units. The same moment, written
	// four ways, and only the number itself says which.
	const unixSeconds = 1827728109

	cases := []struct {
		name string
		in   float64
	}{
		{"unix seconds", unixSeconds},
		{"unix milliseconds", unixSeconds * 1e3},
		{"unix microseconds", unixSeconds * 1e6},
		{"chrome microseconds since 1601", (unixSeconds + chromeEpochOffset) * 1e6},
	}

	for _, c := range cases {
		got := normalizeExpiry(c.in)

		if int64(got) != unixSeconds {
			t.Errorf("%s: want %d, got %v", c.name, unixSeconds, got)
		}
	}
}

func TestNormalizeExpiryKeepsASessionCookie(t *testing.T) {
	for _, in := range []float64{0, -1} {
		if got := normalizeExpiry(in); got != 0 {
			t.Errorf("want 0 for %v, got %v", in, got)
		}
	}
}

func TestNormalizeExpiriesRewritesTheColumn(t *testing.T) {
	// A Chrome dump: the expiry is microseconds since 1601, and read as Unix
	// seconds it is the year 426 million.
	content := "# Netscape HTTP Cookie File\n" +
		".douyin.com\tTRUE\t/\tFALSE\t13472201709491024\tUIFID_TEMP\tvalue\n" +
		"#HttpOnly_.youtube.com\tTRUE\t/\tFALSE\t0\tPREF\tvalue\n" +
		".douyin.com\tTRUE\t/\tFALSE\t0\tis_support_rtm_web_ts\tvalue\n"

	got := normalizeExpiries(content)

	// The cookie with an expiry gets a real one, and the file still parses.
	summary := Summarize(got)
	if summary.Total != 3 {
		t.Fatalf("want three cookies, got %d\n%s", summary.Total, got)
	}

	// A session cookie keeps its zero, and the HttpOnly prefix survives —
	// without it the session cookies that matter most are skipped.
	if !containsLine(got, "#HttpOnly_.youtube.com\tTRUE\t/\tFALSE\t0\tPREF\tvalue") {
		t.Errorf("the HttpOnly line was mangled:\n%s", got)
	}

	// The header comment is left alone.
	if !containsLine(got, "# Netscape HTTP Cookie File") {
		t.Errorf("the header was mangled:\n%s", got)
	}

	// The rewritten date is one a person could read.
	domains := SummarizeDomains(got)

	var found bool
	for _, d := range domains {
		if d.Domain != "douyin.com" {
			continue
		}
		found = true

		if d.NewestExpiry.Year() != 2027 {
			t.Errorf("want a 2027 expiry, got %s\n%s", d.NewestExpiry, got)
		}
	}

	if !found {
		t.Errorf("douyin.com is missing:\n%s", got)
	}
}

func TestConvertNormalizesANetscapeFile(t *testing.T) {
	content := []byte("# Netscape HTTP Cookie File\n" +
		".example.com\tTRUE\t/\tFALSE\t13472201709491024\tname\tvalue\n")

	converted, format, err := Convert(content)
	if err != nil {
		t.Fatalf("converting: %v", err)
	}
	if format != FormatNetscape {
		t.Fatalf("want the format recognised as Netscape, got %v", format)
	}

	domains := SummarizeDomains(converted)
	if len(domains) != 1 {
		t.Fatalf("want one domain, got %d", len(domains))
	}

	if got := domains[0].NewestExpiry; got.Year() != 2027 {
		at := time.Time(got).UTC().Format(time.RFC3339)
		t.Errorf("want a 2027 expiry, got %s", at)
	}
}

func containsLine(haystack, needle string) bool {
	for _, line := range splitLines(haystack) {
		if line == needle {
			return true
		}
	}
	return false
}

func splitLines(s string) []string {
	var out []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			out = append(out, s[start:i])
			start = i + 1
		}
	}
	return append(out, s[start:])
}
