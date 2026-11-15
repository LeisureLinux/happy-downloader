package profiles

import (
	"strings"

	"github.com/LeisureLinux/happy-downloader/v4/server/config"
)

// ProxyFor returns the configured proxy for a URL.
//
// Network policy belongs to the deployment's configuration, not to the code.
// The host list in a profile decides which extra yt-dlp arguments a site gets;
// it does not decide whether the application invents a route for that site.
// If ytdlp.proxy is empty, every URL is fetched directly. If it is set, the
// value is passed to yt-dlp unchanged for every URL.
//
// rawURL is accepted so callers and tests can ask the same question for any
// URL, but it has no effect on the result. This is deliberate: site-specific
// exceptions belong in explicit configuration, not in a hardcoded domain list
// or an automatic reachability probe.
func ProxyFor(rawURL string) string {
	return strings.TrimSpace(config.Instance().YtDlp.Proxy)
}

// isYouTubeHost reports whether a host is YouTube's.
//
// It is only used to decide whether the PO Token provider is worth asking:
// that provider solves a YouTube-specific challenge. It is not a proxy policy.
func isYouTubeHost(host string) bool {
	return host == "youtube.com" || host == "youtu.be" ||
		strings.HasSuffix(host, ".youtube.com")
}
