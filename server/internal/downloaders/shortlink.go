package downloaders

import (
	"context"
	"encoding/base64"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"
)

// Resolving a short link into the address it points at.
//
// A link pasted from an app is a short link — "t.cn/abc" for Weibo,
// "b23.tv/abc" for Bilibili, "v.douyin.com/abc" for Douyin, "qy.net/abc"
// for iQiyi — and the extractors do not know them. The video is one page
// away, behind a link that has to be followed.
//
// The follow is not always a redirect. Weibo answers with a page that carries
// the destination in a script and sends the browser there with
// "location.replace", and iQiyi's short link lands on a page that carries
// the video's id in a parameter rather than in the address. So the answer is
// not always the address the link points at — sometimes it is a number that
// has to be turned into one.
//
// The answers are remembered, because the same link is often pasted twice —
// once to probe it and once to download it — and each resolution is a request
// to the site.

// shortLinkHosts are the hosts whose links are short links.
var shortLinkHosts = map[string]string{
	"t.cn":          "weibo",
	"weibo.cn":      "weibo",
	"b23.tv":        "bilibili",
	"v.douyin.com":  "douyin",
	"iesdouyin.com": "douyin",
	"qy.net":        "iqiyi",
	"iqiyi.com":     "iqiyi",
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

	// The address the link ended at. It is often the answer already: a
	// Bilibili short link redirects straight to the video.
	final := res.Request.URL.String()

	// A redirect that was not followed is the destination itself.
	if res.StatusCode >= 300 && res.StatusCode < 400 {
		if location := res.Header.Get("Location"); location != "" {
			return resolvedLink{URL: absolute(location, final), Site: site}
		}
	}

	body, err := io.ReadAll(io.LimitReader(res.Body, 512<<10))
	if err != nil {
		return resolvedLink{Err: err}
	}

	// iQiyi's short link lands on a page that carries the video's id in a
	// parameter rather than in the address. The id is all that is needed:
	// the video's page is built from it.
	if site == "iqiyi" {
		if url := iqiyiURL(string(body), final); url != "" {
			return resolvedLink{URL: url, Site: site}
		}
	}

	// The destination is in the page, in the script that sends the browser
	// there.
	if url := destinationIn(string(body)); url != "" {
		return resolvedLink{URL: absolute(url, final), Site: site}
	}

	// The address the link ended at is still better than the short link: the
	// extractor may know it even when the short link is not in its list.
	return resolvedLink{URL: final, Site: site}
}

// The iQiyi video's address, worked out from the page its short link leads to.
//
// The short link redirects to a share page whose parameters carry the video's
// id twice: once plainly, as "positiveId", and once base64-encoded, as
// "shareId". The plain one is there to be read; the encoded one is there to
// be put in a link.
//
// The page carries the same number as "tvid", which is what the extractor
// needs, and the video's own id, which it needs as well.
func iqiyiURL(page, final string) string {
	params := queryOf(final)

	tvid := params.Get("positiveId")
	if tvid == "" {
		tvid = params.Get("shareId")
		if decoded, err := base64.StdEncoding.DecodeString(tvid); err == nil {
			tvid = string(decoded)
		}
	}

	if tvid == "" {
		return ""
	}

	// The page has the video's own id, which is what the extractor pairs
	// with the tvid.
	videoID := iqiyiVideoID(page)

	if videoID == "" {
		// Without it there is nothing to build an address from, and an
		// address with only half of it would be worse than none.
		return ""
	}

	return "https://www.iqiyi.com/v_" + videoID + ".html"
}

// The iQiyi video's own id, read from the page.
//
// It is a run of hex characters, and it is the one that is paired with the
// tvid. The page has several: the cover images' ids are of the same shape,
// so the one that sits next to the tvid is the one that is wanted.
var iqiyiVideoIDPattern = regexp.MustCompile(
	`(?i)["']?vid["']?\s*[:=]\s*["']([a-f0-9]{32})["']`)

// A looser pattern, for a page that spells it differently.
var iqiyiLooseVideoIDPattern = regexp.MustCompile(`\b([a-f0-9]{32})\b`)

// iqiyiVideoID reads the video's id out of an iQiyi page.
func iqiyiVideoID(page string) string {
	if m := iqiyiVideoIDPattern.FindStringSubmatch(page); len(m) > 1 {
		return m[1]
	}

	// The id is also in the address the page ends at, when the short link
	// was followed all the way to the video.
	if m := iqiyiLooseVideoIDPattern.FindStringSubmatch(page); len(m) > 1 {
		return m[1]
	}

	return ""
}

// queryOf returns the parameters of an address.
func queryOf(raw string) url.Values {
	parsed, err := url.Parse(raw)
	if err != nil {
		return url.Values{}
	}

	return parsed.Query()
}

// absolute turns a relative address into one that can be fetched.
//
// A page that says where the browser should go may say it relatively, and
// the page it was found on is what makes it meaningful.
func absolute(candidate, base string) string {
	candidate = strings.TrimSpace(candidate)
	if candidate == "" {
		return ""
	}

	parsed, err := url.Parse(candidate)
	if err != nil {
		return candidate
	}

	if parsed.IsAbs() {
		return parsed.String()
	}

	baseURL, err := url.Parse(base)
	if err != nil {
		return candidate
	}

	return baseURL.ResolveReference(parsed).String()
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
