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

	"github.com/LeisureLinux/happy-downloader/v4/server/internal/cookies"
)

// Profile is the argument set for one site.
type Profile struct {
	// Name is a short label for the UI, e.g. "YouTube".
	Name string `json:"name"`
	// HostSuffix matches the URL host by suffix, e.g. "youtube.com".
	// Several domains may share one profile, comma-separated, because a site
	// commonly serves the same content from more than one: YouTube answers on
	// youtube.com and youtu.be, and pasting a short link is the normal case.
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
		HostSuffix: "youtube.com,youtu.be",
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
		for _, suffix := range p.suffixes() {
			if host == suffix || strings.HasSuffix(host, "."+suffix) {
				// The longest matching suffix wins, so a more specific entry
				// beats a broader one even when both match.
				if len(suffix) > bestLen {
					best, bestLen = i, len(suffix)
				}
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

// suffixes splits a profile's host list, trimming blanks.
func (p Profile) suffixes() []string {
	if strings.TrimSpace(p.HostSuffix) == "" {
		return nil
	}

	parts := strings.Split(p.HostSuffix, ",")
	out := make([]string, 0, len(parts))

	for _, part := range parts {
		part = strings.TrimSpace(part)
		part = strings.TrimPrefix(part, "www.")
		if part != "" {
			out = append(out, part)
		}
	}

	return out
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

	if p.PotBaseURL != "" && p.isYouTube() {
		// The provider fetches the BotGuard challenge from YouTube, so it needs
		// the same egress as the download. Its HTTP server has no --proxy flag;
		// the proxy travels per request, in the same extractor-args value.
		//
		// It is only for YouTube. The provider is asked for a challenge that
		// only YouTube's extractor understands, so for any other site the
		// argument is dead weight: the extractor ignores it and the
		// provider is not called at all.
		//
		// The fallback profile carried one, because it was seeded with the
		// global setting — so every site without a profile of its own was
		// asked for a YouTube challenge it had no use for.
		potArg := "youtubepot-bgutilhttp:base_url=" + p.PotBaseURL
		if p.Proxy != "" {
			potArg += ";proxy=" + p.Proxy
		}
		args = append(args, "--extractor-args", potArg)
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

// isYouTube reports whether this profile is YouTube.
//
// It is what decides whether the PO Token provider is worth asking: the
// challenge it fetches is one only YouTube's extractor understands.
func (p Profile) isYouTube() bool {
	for _, suffix := range p.suffixes() {
		if suffix == "youtube.com" || suffix == "youtu.be" {
			return true
		}
	}

	return false
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
