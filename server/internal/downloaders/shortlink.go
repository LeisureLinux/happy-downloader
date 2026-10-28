package downloaders

import (
	"context"
	"io"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"
)

// Resolving a short link into the address it points at.
//
// A link pasted from an app is a short link — "t.cn/abc" for Weibo,
// "b23.tv/abc" for Bilibili — and the extractors do not know them. The video is
// one page away, behind a link that has to be followed.
//
// The follow is not always a redirect. Weibo answers with a page that carries
// the destination in a script and sends the browser there with
// "location.replace", so the address has to be read out of the page.
//
// The answers are remembered, because the same link is often pasted twice —
// once to probe it and once to download it — and each resolution is a request
// to the site.

// shortLinkHosts are the hosts whose links are short links.
var shortLinkHosts = map[string]string{
	"t.cn":         "weibo",
	"weibo.cn":     "weibo",
	"b23.tv":       "bilibili",
	"v.douyin.com": "douyin",
}

// resolvedLink is what a short link pointed at.
type resolvedLink struct {
	// URL is the address the link pointed at.
	URL string
	// Site is the site it belongs to, which the caller already knows.
	Site string
	// Err is why it could not be resolved.
	Err error
}

// The cache entry: what the link resolved to, and when.
type shortLinkEntry struct {
	resolved resolvedLink
	at       time.Time
}

var (
	shortLinkMu    sync.Mutex
	shortLinkCache = map[string]shortLinkEntry{}
)

// shortLinkCacheTTL is how long a resolution is remembered.
const shortLinkCacheTTL = time.Hour

// ResolveShortLink follows a short link to the address it points at.
//
// An address that is not a short link is returned unchanged, with no request:
// most links are not short ones, and every one of them should not cost a
// network round trip.
func ResolveShortLink(ctx context.Context, raw string) (string, error) {
	raw = strings.TrimSpace(raw)

	host := hostOf(raw)
	site, isShort := shortLinkHosts[host]
	if !isShort {
		return raw, nil
	}

	shortLinkMu.Lock()
	cached, ok := shortLinkCache[raw]
	shortLinkMu.Unlock()

	if ok && time.Since(cached.at) < shortLinkCacheTTL {
		if cached.resolved.Err != nil {
			return raw, cached.resolved.Err
		}
		return cached.resolved.URL, nil
	}

	resolved := followShortLink(ctx, raw, site)

	shortLinkMu.Lock()
	shortLinkCache[raw] = shortLinkEntry{resolved: resolved, at: time.Now()}
	shortLinkMu.Unlock()

	if resolved.Err != nil {
		return raw, resolved.Err
	}

	return resolved.URL, nil
}

// followShortLink makes the request and works out where it leads.
func followShortLink(ctx context.Context, raw, site string) resolvedLink {
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, raw, nil)
	if err != nil {
		return resolvedLink{Err: err}
	}

	// A short link is followed by a browser, not by a downloader, and some of
	// them answer a browser differently.
	req.Header.Set("User-Agent", browserAgent)
	req.Header.Set("Accept", "text/html,application/xhtml+xml")

	client := &http.Client{
		// The redirect is followed, but the chain is short: a link that
		// redirects to another redirect is a link that is not going
		// anywhere.
		CheckRedirect: func(r *http.Request, via []*http.Request) error {
			if len(via) >= 5 {
				return http.ErrUseLastResponse
			}
			return nil
		},
	}

	res, err := client.Do(req)
	if err != nil {
		return resolvedLink{Err: err}
	}
	defer res.Body.Close()

	// A redirect that was not followed is the destination itself.
	if res.StatusCode >= 300 && res.StatusCode < 400 {
		if location := res.Header.Get("Location"); location != "" {
			return resolvedLink{URL: location, Site: site}
		}
	}

	body, err := io.ReadAll(io.LimitReader(res.Body, 512<<10))
	if err != nil {
		return resolvedLink{Err: err}
	}

	// The destination is in the page, in the script that sends the browser
	// there.
	if url := destinationIn(string(body)); url != "" {
		return resolvedLink{URL: url, Site: site}
	}

	return resolvedLink{Err: errNoDestination}
}

// browserAgent is what the request says it is.
//
// A short link that is asked for by something that is not a browser gets
// something different — an error page, or a visitor-verification page — and
// the destination is not in it.
const browserAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 " +
	"(KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"

// The destination in a page that redirects with a script.
//
// The shape is a line that assigns the address to a variable and then sends
// the browser there:
//
//	var url = "https://weibo.com/tv/show/1034:5358338104557613";
//	location.replace(url)
//
// The address is looked for in the places it is usually written rather than
// for anything that looks like a link: a page about the short link has links
// of its own, and taking the first one would give the wrong video.
var destinationPatterns = []*regexp.Regexp{
	// var url = "..."
	regexp.MustCompile(`(?i)\burl\s*=\s*["'](https?://[^"']+)["']`),
	// location.replace("...")
	regexp.MustCompile(`(?i)location\.replace\(\s*["'](https?://[^"']+)["']`),
	// window.location.href = "..."
	regexp.MustCompile(`(?i)location\.href\s*=\s*["'](https?://[^"']+)["']`),
	// <meta http-equiv="refresh" content="0;url=...">
	regexp.MustCompile(`(?i)url=([^"'\s>]+)`),
}

// destinationIn finds the address a page sends the browser to.
func destinationIn(page string) string {
	for _, pattern := range destinationPatterns {
		m := pattern.FindStringSubmatch(page)
		if len(m) < 2 {
			continue
		}

		candidate := strings.TrimSpace(m[1])
		candidate = strings.ReplaceAll(candidate, "&amp;", "&")

		if candidate == "" {
			continue
		}

		return candidate
	}

	return ""
}

// hostOf returns the host of a URL, without the scheme or the path.
//
// A string that is not a URL has no host: it is returned as it is, so it
// matches no short link and is left alone.
func hostOf(raw string) string {
	rest := raw

	if i := strings.Index(rest, "://"); i >= 0 {
		rest = rest[i+3:]
	} else if !strings.Contains(rest, ".") {
		return ""
	}

	if i := strings.IndexAny(rest, "/?#"); i >= 0 {
		rest = rest[:i]
	}

	rest = strings.ToLower(rest)
	rest = strings.TrimPrefix(rest, "www.")

	// A port is not part of the name.
	if colon := strings.LastIndex(rest, ":"); colon > 0 {
		rest = rest[:colon]
	}

	return rest
}

// errNoDestination is returned when a short link led nowhere usable.
var errNoDestination = errShortLink("the short link did not lead to a video")

type errShortLink string

func (e errShortLink) Error() string { return string(e) }
