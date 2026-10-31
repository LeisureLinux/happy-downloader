package rest

import (
	"strings"
	"testing"
)

// The three answers yt-dlp gives, as they appear in its output.
func TestProbeReadsYtDlpsOutput(t *testing.T) {
	cases := []struct {
		name     string
		output   string
		wantOK   bool
		wantProb string
	}{
		{
			name:   "cookies were read",
			output: "[Cookies] Extracting cookies from firefox\nExtracted 42 cookies from firefox\n",
			wantOK: true,
		},
		{
			name:   "loading progress means the database was read",
			output: "[Cookies] Extracting cookies from chrome\n[Cookies] Loading cookie 12/30\n",
			wantOK: true,
		},
		{
			name:     "app-bound encryption",
			output:   "[Cookies] Extracting cookies from chrome\nWARNING: failed to decrypt with DPAPI\n",
			wantProb: "encrypted",
		},
		{
			name:     "the database is locked by a running browser",
			output:   "ERROR: could not copy Chrome cookie database\n",
			wantProb: "locked",
		},
		{
			name:     "nothing readable at all",
			output:   "ERROR: no cookies found\n",
			wantProb: "unreadable",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			// The parsing is what is under test, so the same decisions are
			// taken on the text alone.
			got := classifyProbeOutput(c.output)

			if got.OK != c.wantOK {
				t.Errorf("OK = %v, want %v", got.OK, c.wantOK)
			}

			if got.Problem != c.wantProb {
				t.Errorf("Problem = %q, want %q", got.Problem, c.wantProb)
			}
		})
	}
}

func TestLastLines(t *testing.T) {
	got := lastLines("1\n2\n3\n4\n5\n", 2)
	if strings.TrimSpace(got) != "4\n5" {
		t.Fatalf("lastLines = %q", got)
	}
}
