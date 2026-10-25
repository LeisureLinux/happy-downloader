// Package profiles maps a video URL to the yt-dlp arguments that site needs.
//
// The host is a shared one: the same binary serves YouTube (which wants an
// account cookie, a PO Token provider and often a proxy), Bilibili (direct,
// no cookie needed for public videos) and x.com (cookie plus proxy) — and the
// operator adds sites over time. Two rules follow from that:
//
//   - The server cannot rely on yt-dlp's own config file. A profile that
//     depends on ~/.config/yt-dlp/config only works on the machine it was
//     authored on; every argument this server needs is passed explicitly and
//     --ignore-config keeps any host config out of the way.
//   - The argument set has to be data, not code. A new site is a new row in
//     a table the UI can edit, not a rebuild.
//
// A profile is matched by URL host. Suffix matching handles regional hosts
// (www.youtube.com, m.youtube.com, music.youtube.com) with one entry, and a
// fallback profile with an empty host pattern catches everything else.
package profiles

import (
	"net/url"
	"sort"
	"strings"

	"github.com/marcopiovanello/yt-dlp-web-ui/v4/server/internal/cookies"
)

// Profile is the argument set for one site.
type Profile struct {
	// Name is a short label for the UI, e.g. "YouTube".
	Name string `json:"name"`
	// HostSuffix matches the URL host by suffix, e.g. "youtube.com".
	// Empty matches every URL that no other profile matched.
	HostSuffix string `json:"hostSuffix"`
	// CookiePath points at a cookie export (JSON or Netscape) to convert.
	CookiePath string `json:"cookiePath,omitempty"`
	// PotBaseURL is the bgutil POT provider, e.g. http://127.0.0.1:4416.
	PotBaseURL string `json:"potBaseUrl,omitempty"`
	// Proxy is passed as --proxy.
	Proxy string `json:"proxy,omitempty"`
	// ExtraArgs are passed through verbatim, one per row in the UI.
	ExtraArgs []string `json:"extraArgs,omitempty"`
	// PreferFreeFormats sorts by free codec first, which is the right
	// default for archiving and the wrong one only rarely.
	PreferFreeFormats bool `json:"preferFreeFormats,omitempty"`
}

// table is the ordered profile list. Later entries win when several match,
// which keeps the fallback (empty HostSuffix) last regardless of storage
// order.
type table []Profile

// defaultTable is the baseline the server ships with. It is seeded into the
// store on first run and fully editable afterwards.
var defaultTable = table{
	{
		Name:       "YouTube",
		HostSuffix: "youtube.com",
		// Filled from the global config at seed time, see Seed().
	},
	{
		Name:       "Bilibili",
		HostSuffix: "bilibili.com",
	},
	{
		Name:       "x.com",
		HostSuffix: "x.com",
	},
	{
		Name:       "twitter.com",
		HostSuffix: "twitter.com",
	},
	{
		Name: "Fallback",
		// HostSuffix empty: matches everything else.
	},
}

// match returns the profile for a URL and the index it was found at.
//
// The longest matching host suffix wins, so "m.youtube.com" and
// "youtube.com" both resolve to the YouTube entry while a hypothetical
// "notyoutube.com" does not. The fallback is the last resort.
func (t table) match(rawURL string) (Profile, bool) {
	host := hostOf(rawURL)

	best := -1
	bestLen := -1

	for i, p := range t {
		if p.HostSuffix == "" {
			continue
		}
		if host == p.HostSuffix || strings.HasSuffix(host, "."+p.HostSuffix) {
			if len(p.HostSuffix) > bestLen {
				best, bestLen = i, len(p.HostSuffix)
			}
		}
	}

	if best >= 0 {
		return t[best], true
	}

	// Fallback: the entry with an empty suffix.
	for _, p := range t {
		if p.HostSuffix == "" {
			return p, true
		}
	}

	return Profile{}, false
}

// hostOf extracts the hostname, tolerating input that is not a full URL.
func hostOf(rawURL string) string {
	s := strings.TrimSpace(rawURL)
	if s == "" {
		return ""
	}

	u, err := url.Parse(s)
	if err != nil || u.Host == "" {
		// Maybe scheme-less, e.g. "youtu.be/xyz".
		u2, err2 := url.Parse("https://" + s)
		if err2 != nil {
			return ""
		}
		u = u2
	}

	h := strings.ToLower(u.Hostname())
	h = strings.TrimPrefix(h, "www.")
	return h
}

// Argv assembles the yt-dlp command line for a URL from the profile.
//
// The caller supplies the cookie staging path (a copy yt-dlp may rewrite)
// and the sanitized JS runtime; this function stays free of both the config
// singleton and the filesystem, which keeps it testable.
func (p Profile) Argv(cookieStaging, jsRuntime string) []string {
	args := []string{"--ignore-config"}

	if jsRuntime != "" {
		args = append(args, "--js-runtimes", jsRuntime)
	}

	if cookieStaging != "" {
		args = append(args, "--cookies", cookieStaging)
	}

	if p.PotBaseURL != "" {
		args = append(args,
			"--extractor-args", "youtubepot-bgutilhttp:base_url="+p.PotBaseURL)
	}

	if p.Proxy != "" {
		args = append(args, "--proxy", p.Proxy)
	}

	if p.PreferFreeFormats {
		args = append(args, "--prefer-free-formats")
	}

	for _, a := range p.ExtraArgs {
		if strings.TrimSpace(a) != "" {
			args = append(args, a)
		}
	}

	return args
}

// Summarize describes a cookie file without echoing values.
func Summarize(netscape string) cookies.Summary {
	return cookies.Summarize(netscape)
}

// sortedCopy returns the table with the fallback last, for display and
// storage. Sorting a copy keeps the in-memory order stable for match().
func (t table) sortedCopy() table {
	out := make(table, len(t))
	copy(out, t)

	sort.SliceStable(out, func(i, j int) bool {
		if (out[i].HostSuffix == "") != (out[j].HostSuffix == "") {
			return out[j].HostSuffix == ""
		}
		return out[i].Name < out[j].Name
	})

	return out
}
