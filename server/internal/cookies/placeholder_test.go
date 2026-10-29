package cookies

import (
	"testing"
)

// A browser export holds an entry for every cookie of every site, and one of
// them is often a placeholder with no name.
//
// Throwing the whole export away for it meant that a file with seventy-two
// good cookies could not be imported at all — which is what happened with a
// Douyin export.
func TestJSONExportWithAPlaceholderEntry(t *testing.T) {
	export := `[
	  {"name":"enter_pc_once","value":"1","domain":".douyin.com","path":"/",
	   "expires":1827794460.02368,"httpOnly":false,"secure":true},
	  {"name":"","value":"douyin.com","domain":"www.douyin.com","path":"/",
	   "expires":-1,"httpOnly":false,"secure":false},
	  {"name":"ttwid","value":"abc","domain":".douyin.com","path":"/",
	   "expires":1827794460.02368,"httpOnly":true,"secure":true}
	]`

	converted, format, err := Convert([]byte(export))
	if err != nil {
		t.Fatalf("Convert: %v", err)
	}

	if format != FormatJSON {
		t.Errorf("format = %q, want json", format)
	}

	// The two good cookies are there.
	for _, want := range []string{"enter_pc_once", "ttwid"} {
		if !contains(converted, "\t"+want+"\t") {
			t.Errorf("%s is missing:\n%s", want, converted)
		}
	}

	// The placeholder is not.
	if contains(converted, "\t\t") {
		t.Errorf("the entry with no name was written:\n%s", converted)
	}
}

// An expiry in the past is written as "-1" by some browsers, and a negative
// timestamp in a cookie file means nothing to the reader.
func TestNegativeExpiryBecomesASessionCookie(t *testing.T) {
	export := `[
	  {"name":"gone","value":"x","domain":".douyin.com","path":"/",
	   "expires":-1,"httpOnly":false,"secure":true}
	]`

	converted, _, err := Convert([]byte(export))
	if err != nil {
		t.Fatalf("Convert: %v", err)
	}

	summary := Summarize(converted)
	if summary.Expired != 0 {
		t.Errorf("expired = %d, want 0", summary.Expired)
	}
}

func contains(haystack, needle string) bool {
	return len(haystack) >= len(needle) &&
		(func() bool {
			for i := 0; i+len(needle) <= len(haystack); i++ {
				if haystack[i:i+len(needle)] == needle {
					return true
				}
			}
			return false
		})()
}
