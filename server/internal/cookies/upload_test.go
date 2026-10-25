package cookies

import (
	"strings"
	"testing"
)

// The scenario this exists for: a cookie file exported as JSON by a browser
// extension, uploaded to a headless host, converted to Netscape, and then
// passed to yt-dlp. Every step has to survive or the download fails with a
// misleading bot error.
func TestJSONUploadBecomesUsableCookieFile(t *testing.T) {
	upload := `[
	  {"domain":".youtube.com","expirationDate":1798761600,"httpOnly":true,
	   "name":"LOGIN_INFO","path":"/","secure":true,"value":"AF1QizMfR2xxx"},
	  {"domain":".youtube.com","expirationDate":1798761600,"httpOnly":true,
	   "name":"HSID","path":"/","secure":true,"value":"AbCdEf123"},
	  {"domain":".youtube.com","expirationDate":1798761600,"httpOnly":true,
	   "name":"SID","path":"/","secure":true,"value":"g.a000yyy"},
	  {"domain":".youtube.com","expirationDate":1798761600,"httpOnly":true,
	   "name":"__Secure-3PSID","path":"/","secure":true,"value":"g.a000xxx"}
	]`

	converted, format, err := Convert([]byte(upload))
	if err != nil {
		t.Fatalf("Convert: %v", err)
	}
	if format != FormatJSON {
		t.Fatalf("format = %q, want json", format)
	}

	// Every login cookie must survive, and each httpOnly entry must keep the
	// #HttpOnly_ prefix: dropping it makes curl skip the line, which silently
	// turns a logged-in session into a guest one.
	for _, name := range []string{"LOGIN_INFO", "HSID", "SID", "__Secure-3PSID"} {
		if !strings.Contains(converted, "\t"+name+"\t") {
			t.Errorf("%s missing from converted file:\n%s", name, converted)
		}
	}

	if got := strings.Count(converted, "#HttpOnly_"); got != 4 {
		t.Errorf("httpOnly prefix count = %d, want 4:\n%s", got, converted)
	}

	// Nothing may be reported missing once converted.
	s := Summarize(converted)
	if missing := Required(s.Names); len(missing) > 0 {
		t.Errorf("login cookies still missing after conversion: %v", missing)
	}

	// Values must be intact, since a mangled value is as good as absent.
	for _, v := range []string{"AF1QizMfR2xxx", "AbCdEf123", "g.a000yyy", "g.a000xxx"} {
		if !strings.Contains(converted, v) {
			t.Errorf("value %q lost in conversion", v)
		}
	}
}

// A JSON export that lacks LOGIN_INFO is exactly the file that produced the
// original failure. The converter must still produce a valid file, and the
// caller must be told what is missing rather than left to guess.
func TestJSONUploadMissingLoginInfoIsReported(t *testing.T) {
	upload := `[
	  {"domain":".youtube.com","expirationDate":1798761600,"httpOnly":true,
	   "name":"__Secure-3PSID","path":"/","secure":true,"value":"g.a000xxx"},
	  {"domain":".youtube.com","name":"PREF","path":"/","session":true,"value":"tz=UTC"}
	]`

	converted, _, err := Convert([]byte(upload))
	if err != nil {
		t.Fatalf("Convert: %v", err)
	}

	missing := Required(Summarize(converted).Names)
	if len(missing) == 0 {
		t.Fatal("expected missing login cookies to be reported")
	}

	found := false
	for _, m := range missing {
		if m == "LOGIN_INFO" {
			found = true
		}
	}
	if !found {
		t.Errorf("LOGIN_INFO not reported missing: %v", missing)
	}
}

// Netscape and header inputs must still be accepted, so an existing cookies.txt
// keeps working after the converter is added.
func TestExistingFormatsStillAccepted(t *testing.T) {
	netscape := "# Netscape HTTP Cookie File\n.youtube.com\tTRUE\t/\tTRUE\t1798761600\tSID\tx\n"
	header := "SID=x; HSID=y"

	for _, in := range []string{netscape, header} {
		out, _, err := Convert([]byte(in))
		if err != nil {
			t.Errorf("Convert(%q): %v", in, err)
			continue
		}
		if Summarize(out).Total == 0 {
			t.Errorf("Convert(%q) produced no cookies", in)
		}
	}
}
