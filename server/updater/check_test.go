package updater

import "testing"

// The comparison decides whether the upgrade marker appears, so a wrong
// answer either hides an update or invents one.
func TestIsNewer(t *testing.T) {
	cases := []struct {
		latest  string
		current string
		want    bool
	}{
		{"2026.10.26", "2026.10.21", true},
		{"2026.10.21", "2026.10.21", false},
		{"2026.10.21", "2026.10.26", false},
		{"2026.11.01", "2026.10.26", true},
		{"2026.09.01", "2026.10.26", false},
		// A daily build carries a build suffix; the release it becomes is
		// the same day, which is not newer.
		{"2026.10.21", "2026.10.21.123456", false},
		{"2026.10.22", "2026.10.21.123456", true},
		// A version that is not a date cannot be compared, and guessing
		// would say "newer" about something that is not.
		{"nightly", "2026.10.21", false},
		{"2026.10.21", "dev", false},
		{"", "2026.10.21", false},
		{"2026.10.21", "", false},
	}

	for _, c := range cases {
		t.Run(c.latest+" vs "+c.current, func(t *testing.T) {
			if got := isNewer(c.latest, c.current); got != c.want {
				t.Errorf("isNewer(%q, %q) = %v, want %v", c.latest, c.current, got, c.want)
			}
		})
	}
}

// The build suffix is dropped before comparing.
func TestTrimBuild(t *testing.T) {
	cases := map[string]string{
		"2026.10.21":          "2026.10.21",
		"2026.10.21.123456":   "2026.10.21",
		"2026.10.21.123456.7": "2026.10.21",
		"nightly":             "nightly",
	}

	for in, want := range cases {
		if got := trimBuild(in); got != want {
			t.Errorf("trimBuild(%q) = %q, want %q", in, got, want)
		}
	}
}

// The asset name depends on the architecture, and the wrong one replaces a
// working downloader with one that cannot start.
func TestAssetName(t *testing.T) {
	cases := map[string]string{
		"amd64": "yt-dlp_linux",
		"arm64": "yt-dlp_linux_aarch64",
		"arm":   "yt-dlp_linux_armv7l",
	}

	for arch, want := range cases {
		got, ok := assetName(arch)
		if !ok {
			t.Errorf("assetName(%q) reported no asset", arch)
			continue
		}
		if got != want {
			t.Errorf("assetName(%q) = %q, want %q", arch, got, want)
		}
	}

	if _, ok := assetName("riscv64"); ok {
		t.Error("assetName found an asset for an architecture that has none")
	}
}
