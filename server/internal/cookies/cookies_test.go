package cookies

import (
	"strings"
	"testing"
)

// The JSON shape browser extensions export: an array of objects with named
// fields, including the httpOnly flag that carries the session cookies.
const jsonExport = `[
  {
    "domain": ".youtube.com",
    "expirationDate": 1798761600,
    "hostOnly": false,
    "httpOnly": true,
    "name": "LOGIN_INFO",
    "path": "/",
    "sameSite": "unspecified",
    "secure": true,
    "session": false,
    "value": "AF1QizMfR2xxx"
  },
  {
    "domain": ".youtube.com",
    "expirationDate": 1798761600,
    "httpOnly": true,
    "name": "HSID",
    "path": "/",
    "secure": true,
    "value": "AbCdEf123"
  },
  {
    "domain": ".youtube.com",
    "expirationDate": 1798761600,
    "httpOnly": true,
    "name": "SID",
    "path": "/",
    "secure": true,
    "value": "g.a000yyy"
  },
  {
    "domain": ".youtube.com",
    "expirationDate": 1798761600,
    "httpOnly": true,
    "name": "__Secure-3PSID",
    "path": "/",
    "secure": true,
    "value": "g.a000xxx"
  },
  {
    "domain": ".youtube.com",
    "httpOnly": false,
    "name": "PREF",
    "path": "/",
    "secure": false,
    "session": true,
    "value": "tz=Asia.Shanghai"
  }
]`

func TestDetect(t *testing.T) {
	cases := []struct {
		name    string
		content string
		want    Format
	}{
		{"json array", `[{"name":"a","value":"b"}]`, FormatJSON},
		{"json single object", `{"name":"a","value":"b","domain":".x.com"}`, FormatJSON},
		{"netscape header comment", "# Netscape HTTP Cookie File\n", FormatNetscape},
		{"netscape httponly prefix", "#HttpOnly_.youtube.com\tTRUE\t/\tTRUE\t0\tSID\tx", FormatNetscape},
		{"netscape no comment", ".youtube.com\tTRUE\t/\tTRUE\t0\tSID\tx", FormatNetscape},
		{"header string", "SID=abc; HSID=def", FormatHeader},
		{"header with cookie prefix", "Cookie: SID=abc", FormatHeader},
		{"empty", "", FormatUnknown},
		{"garbage", "not a cookie file at all", FormatUnknown},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := Detect([]byte(c.content)); got != c.want {
				t.Errorf("Detect(%q) = %q, want %q", c.content, got, c.want)
			}
		})
	}
}

func TestConvertJSON(t *testing.T) {
	out, format, err := Convert([]byte(jsonExport))
	if err != nil {
		t.Fatalf("Convert: %v", err)
	}
	if format != FormatJSON {
		t.Errorf("format = %q, want json", format)
	}

	// httpOnly cookies must carry the #HttpOnly_ prefix, otherwise curl skips
	// the line and the session cookies that decide login state are lost.
	if !strings.Contains(out, "#HttpOnly_.youtube.com\tTRUE\t/\tTRUE\t1798761600\tLOGIN_INFO\tAF1QizMfR2xxx") {
		t.Errorf("LOGIN_INFO missing or malformed:\n%s", out)
	}

	if !strings.Contains(out, "#HttpOnly_.youtube.com\tTRUE\t/\tTRUE\t1798761600\t__Secure-3PSID\tg.a000xxx") {
		t.Errorf("__Secure-3PSID missing or malformed:\n%s", out)
	}

	// A session cookie has no expiry and is written as 0, not as a made-up
	// future timestamp that would make it look stale later.
	if !strings.Contains(out, ".youtube.com\tTRUE\t/\tFALSE\t0\tPREF\ttz=Asia.Shanghai") {
		t.Errorf("session cookie PREF missing or malformed:\n%s", out)
	}

	if !strings.HasPrefix(out, "# Netscape HTTP Cookie File\n") {
		t.Errorf("missing Netscape header:\n%s", out)
	}
}

// A JSON export that is already Netscape must round-trip untouched.
// The real export from a browser extension uses "expires" rather than
// "expirationDate". Treating that as an absent expiry would turn every cookie
// into a session cookie and lose the persistence the browser gave it.
func TestConvertAcceptsExpiresFieldName(t *testing.T) {
	upload := `[
	  {"domain":".youtube.com","expires":1823905798.159206,"httpOnly":true,
	   "name":"LOGIN_INFO","path":"/","secure":true,"value":"TESTVALUE-NOT-A-REAL-TOKEN"},
	  {"domain":".youtube.com","expires":1823905798.159206,"httpOnly":true,
	   "name":"SID","path":"/","secure":true,"value":"g.a000yyy"}
	]`

	out, _, err := Convert([]byte(upload))
	if err != nil {
		t.Fatalf("Convert: %v", err)
	}

	if !strings.Contains(out, "\t1823905798\tLOGIN_INFO\tTESTVALUE-NOT-A-REAL-TOKEN") {
		t.Errorf("expiry from \"expires\" field not preserved:\n%s", out)
	}

	s := Summarize(out)
	if s.Session != 0 {
		t.Errorf("Session = %d, want 0 (both cookies carry an expiry)", s.Session)
	}
	if s.Expired != 0 {
		t.Errorf("Expired = %d, want 0 (expiry is in the future)", s.Expired)
	}
}

// "expirationDate" is the other spelling in the wild and must keep working.
func TestConvertAcceptsExpirationDateFieldName(t *testing.T) {
	upload := `[
	  {"domain":".youtube.com","expirationDate":1823905798,"httpOnly":true,
	   "name":"SID","path":"/","secure":true,"value":"g.a000yyy"}
	]`

	out, _, err := Convert([]byte(upload))
	if err != nil {
		t.Fatalf("Convert: %v", err)
	}

	if !strings.Contains(out, "\t1823905798\tSID\t") {
		t.Errorf("expiry from \"expirationDate\" field not preserved:\n%s", out)
	}
}

func TestConvertNetscapeIsPassThrough(t *testing.T) {
	netscape := "# Netscape HTTP Cookie File\n.youtube.com\tTRUE\t/\tTRUE\t0\tSID\tx\n"

	out, format, err := Convert([]byte(netscape))
	if err != nil {
		t.Fatalf("Convert: %v", err)
	}
	if format != FormatNetscape {
		t.Errorf("format = %q, want netscape", format)
	}
	if out != netscape {
		t.Errorf("netscape content was rewritten:\ngot:  %q\nwant: %q", out, netscape)
	}
}

func TestConvertHeader(t *testing.T) {
	out, format, err := Convert([]byte("SID=abc; HSID=def"))
	if err != nil {
		t.Fatalf("Convert: %v", err)
	}
	if format != FormatHeader {
		t.Errorf("format = %q, want header", format)
	}
	if !strings.Contains(out, "\tSID\tabc") || !strings.Contains(out, "\tHSID\tdef") {
		t.Errorf("header pairs not converted:\n%s", out)
	}
}

func TestConvertRejectsGarbage(t *testing.T) {
	if _, _, err := Convert([]byte("not a cookie file at all")); err == nil {
		t.Error("expected an error for unrecognised content")
	}
}

// Wrapper shapes differ between extensions: {"cookies": [...]}, a bare array,
// and a single object must all work, since the file arrives on a headless host
// with no way to ask for a re-export.
func TestExtractListShapes(t *testing.T) {
	cases := []struct {
		name    string
		content string
		want    int
	}{
		{"array", `[{"name":"a","value":"1","domain":".x.com"}]`, 1},
		{"wrapped cookies key", `{"cookies":[{"name":"a","value":"1","domain":".x.com"}]}`, 1},
		{"wrapped Cookies key", `{"Cookies":[{"name":"a","value":"1","domain":".x.com"}]}`, 1},
		{"single object", `{"name":"a","value":"1","domain":".x.com"}`, 1},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			out, _, err := Convert([]byte(c.content))
			if err != nil {
				t.Fatalf("Convert: %v", err)
			}
			if got := Summarize(out).Total; got != c.want {
				t.Errorf("parsed %d cookies, want %d\n%s", got, c.want, out)
			}
		})
	}
}

// The export that started this: a JSON file with the httpOnly session cookies.
// If these are dropped, YouTube serves the request as a guest and the failure
// only shows up later as "Sign in to confirm you're not a bot".
func TestSummarizeCountsHttpOnlyAndMissingLoginCookies(t *testing.T) {
	out, _, err := Convert([]byte(jsonExport))
	if err != nil {
		t.Fatalf("Convert: %v", err)
	}

	s := Summarize(out)
	if s.Total != 5 {
		t.Errorf("Total = %d, want 5", s.Total)
	}
	if s.HttpOnly != 4 {
		t.Errorf("HttpOnly = %d, want 4", s.HttpOnly)
	}
	if s.Secure != 4 {
		t.Errorf("Secure = %d, want 4", s.Secure)
	}
	if s.Session != 1 {
		t.Errorf("Session = %d, want 1", s.Session)
	}

	missing := Required(s.Names)
	if len(missing) != 0 {
		t.Errorf("login cookies reported missing but present: %v", missing)
	}

	// The names list must not contain values.
	for _, n := range s.Names {
		if strings.Contains(n, "AF1QizMfR2xxx") || strings.Contains(n, "g.a000xxx") {
			t.Errorf("cookie value leaked into summary: %q", n)
		}
	}
}

func TestRequiredDetectsMissingLoginCookies(t *testing.T) {
	missing := Required([]string{"PREF", "VISITOR_INFO1_LIVE", "YSC"})

	want := map[string]bool{"LOGIN_INFO": true, "HSID": true, "SID": true, "__Secure-3PSID": true}
	if len(missing) != len(want) {
		t.Fatalf("missing = %v, want %d entries", missing, len(want))
	}
	for _, m := range missing {
		if !want[m] {
			t.Errorf("unexpected missing cookie: %q", m)
		}
	}
}

func TestSummarizeReportsExpired(t *testing.T) {
	netscape := "# Netscape HTTP Cookie File\n.youtube.com\tTRUE\t/\tTRUE\t1000000000\tOLD\tv\n"

	s := Summarize(netscape)
	if s.Expired != 1 {
		t.Errorf("Expired = %d, want 1", s.Expired)
	}
}
