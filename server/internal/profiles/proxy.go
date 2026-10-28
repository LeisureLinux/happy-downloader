package profiles

import (
	"context"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/LeisureLinux/happy-downloader/v4/server/config"
)

// Choosing the route to a site.
//
// A proxy is there because the direct route does not work — either because
// the site is not reachable from here, or because it refuses a request from
// this country. Which of the two it is differs per site: YouTube is not
// blocked in China but refuses a Chinese address, while a site that is only
// reachable from abroad cannot be reached at all.
//
// So the route is decided by trying. A site that answers directly is reached
// directly, and a site that does not is reached through the proxy. The answer
// is remembered, because a request that times out costs twenty seconds and
// the answer does not change.
//
// The sites that are known to need the proxy are the exception: they are
// always reached through it, because the failure there is not a timeout but a
// refusal, and a refusal does not look like a failure.

// The sites that are always reached through the proxy.
//
// The proxy is there for them, and trying the direct route first would send a
// request that is going to be refused — which is the kind of refusal that
// looks like success until it does not.
var alwaysProxied = map[string]bool{
	"youtube.com": true,
	"youtu.be":    true,
	"x.com":       true,
	"twitter.com": true,
}

// ProxyFor returns the proxy a URL should be fetched through, or an empty
// string for the direct route.
//
// The decision is made per site and remembered.
func ProxyFor(rawURL string) string {
	proxy := strings.TrimSpace(config.Instance().YtDlp.Proxy)
	if proxy == "" {
		return ""
	}

	host := hostOf(rawURL)
	if host == "" {
		return proxy
	}

	// A site that is known to need the proxy gets it without asking.
	for suffix, needed := range alwaysProxied {
		if host == suffix || strings.HasSuffix(host, "."+suffix) {
			if needed {
				return proxy
			}
		}
	}

	// What worked last time is what works this time.
	routeMu.Lock()
	known, ok := routeCache[host]
	routeMu.Unlock()

	if ok {
		if known == routeDirect {
			return ""
		}
		return proxy
	}

	// Nobody has asked yet. Ask, once, and remember the answer.
	if siteAnswers(host) {
		rememberRoute(host, routeDirect)
		return ""
	}

	rememberRoute(host, routeProxy)
	return proxy
}

// The route that worked.
const (
	routeDirect = "direct"
	routeProxy  = "proxy"
)

var (
	routeMu    sync.Mutex
	routeCache = map[string]string{}
)

// rememberRoute records what worked for a site.
func rememberRoute(host, route string) {
	routeMu.Lock()
	routeCache[host] = route
	routeMu.Unlock()
}

// siteAnswers reports whether a site can be reached directly.
//
// It is a HEAD request to the site's own host, with a short timeout: the
// question is whether the connection works, not whether the site wants
// anything.
func siteAnswers(host string) bool {
	ctx, cancel := context.WithTimeout(context.Background(), 6*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodHead, "https://"+host+"/", nil)
	if err != nil {
		return false
	}

	req.Header.Set("User-Agent", browserAgent)

	client := &http.Client{
		Timeout: 8 * time.Second,
		// The redirect is not followed: the question is whether the host
		// answers at all.
		CheckRedirect: func(r *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	res, err := client.Do(req)
	if err != nil {
		return false
	}
	defer res.Body.Close()

	// Any answer means the route works. A site that refuses a HEAD request
	// with a 403 is still reachable, and so is one that answers 404.
	return true
}

// browserAgent is what the probe says it is.
const browserAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 " +
	"(KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"

// IsYouTubeHost reports whether a host is YouTube's.
//
// It is what decides whether the PO Token provider is worth asking: the
// challenge it fetches is one only YouTube's extractor understands.
func isYouTubeHost(host string) bool {
	return host == "youtube.com" || host == "youtu.be" ||
		strings.HasSuffix(host, ".youtube.com")
}

// LocalAddress returns the address of this host on the internet.
//
// It is what the country is worked out from, so it is asked once and
// remembered.
var (
	addressOnce sync.Once
	localAddr   string
)

// LocalAddress reports the public address of this host.
func LocalAddress() string {
	addressOnce.Do(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		req, err := http.NewRequestWithContext(ctx, http.MethodGet,
			"https://api.ipify.org", nil)
		if err != nil {
			return
		}

		res, err := (&http.Client{Timeout: 5 * time.Second}).Do(req)
		if err != nil {
			return
		}
		defer res.Body.Close()

		buf := make([]byte, 64)
		n, _ := res.Body.Read(buf)

		localAddr = strings.TrimSpace(string(buf[:n]))
	})

	return localAddr
}

// countryOf works out the country of an address.
//
// It is a lookup of the country a network is registered in, which is close
// enough to where it is for the purpose here: a site that refuses requests
// from one country and accepts them from another is refusing the network's
// country.
func countryOf(address string) string {
	if address == "" {
		return ""
	}

	// The address is not a name, so there is nothing to look up.
	if net.ParseIP(address) == nil {
		return ""
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		"https://ipapi.co/"+address+"/country/", nil)
	if err != nil {
		return ""
	}

	res, err := (&http.Client{Timeout: 5 * time.Second}).Do(req)
	if err != nil {
		return ""
	}
	defer res.Body.Close()

	buf := make([]byte, 8)
	n, _ := res.Body.Read(buf)

	return strings.ToUpper(strings.TrimSpace(string(buf[:n])))
}

// CountryOfHost returns the country a host is registered in.
//
// It is used to decide whether a site is likely to refuse a request from
// here, which is the difference between a site that needs the proxy and one
// that does not.
func CountryOfHost(host string) string {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		"https://ipapi.co/"+host+"/country/", nil)
	if err != nil {
		return ""
	}

	res, err := (&http.Client{Timeout: 5 * time.Second}).Do(req)
	if err != nil {
		return ""
	}
	defer res.Body.Close()

	buf := make([]byte, 8)
	n, _ := res.Body.Read(buf)

	return strings.ToUpper(strings.TrimSpace(string(buf[:n])))
}

// portOf returns the port a URL names, or the default for its scheme.
func portOf(rawURL string) int {
	if i := strings.Index(rawURL, ":"); i >= 0 {
		rest := rawURL[i+1:]
		if j := strings.IndexAny(rest, "/?#"); j >= 0 {
			rest = rest[:j]
		}
		if n, err := strconv.Atoi(rest); err == nil {
			return n
		}
	}

	if strings.HasPrefix(rawURL, "http://") {
		return 80
	}

	return 443
}
