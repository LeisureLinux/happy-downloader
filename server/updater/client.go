package updater

import (
	"net/http"
	"net/url"
	"time"

	"github.com/LeisureLinux/happy-downloader/v4/server/config"
)

// The HTTP client the updater uses.
//
// It has to go through the same proxy as everything else on the host. The
// update downloads from GitHub, and on a host that reaches the internet
// through a proxy — which this one does — a direct connection times out.
//
// The proxy is the one in the configuration, the same value that is passed
// to yt-dlp as --proxy. Using it here is not a preference: without it the
// update simply fails, and the message says nothing about the network.
func httpClient() *http.Client {
	proxy := config.Instance().YtDlp.Proxy

	if proxy == "" {
		// No proxy configured. The environment is still consulted, so a host
		// that sets http_proxy gets it without having to be configured here.
		return &http.Client{Timeout: 60 * time.Second}
	}

	parsed, err := url.Parse(proxy)
	if err != nil {
		return &http.Client{Timeout: 60 * time.Second}
	}

	return &http.Client{
		Timeout: 60 * time.Second,
		Transport: &http.Transport{
			Proxy: http.ProxyURL(parsed),
		},
	}
}
