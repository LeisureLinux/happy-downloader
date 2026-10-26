package middlewares

import (
	"log/slog"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5/middleware"
)

// AccessLog records one line per request, with the client address.
//
// The point is answering "which machine on the LAN did that": the log is read
// from a browser elsewhere on the network, and a bare "download started" says
// nothing about who started it. Behind a reverse proxy RemoteAddr is always
// the proxy, so the forwarded headers win when present.
func AccessLog(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Skip what carries no information: the log stream is long-lived and
		// its connect/disconnect lines would drown everything else, and the
		// frontend's static assets are noise on every page load.
		if shouldSkipLog(r.URL.Path) {
			next.ServeHTTP(w, r)
			return
		}

		start := time.Now()
		ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)

		next.ServeHTTP(ww, r)

		slog.Info("request",
			slog.String("client", ClientIP(r)),
			slog.String("method", r.Method),
			slog.String("path", r.URL.Path),
			slog.Int("status", ww.Status()),
			slog.String("dur", time.Since(start).Round(time.Millisecond).String()),
		)
	})
}

// shouldSkipLog reports whether a request is noise for this log.
func shouldSkipLog(path string) bool {
	switch {
	case strings.HasSuffix(path, "/log/sse"), strings.HasSuffix(path, "/log/ws"):
		return true
	case strings.HasPrefix(path, "/assets/"):
		return true
	case path == "/", path == "/favicon.ico":
		return true
	}
	return false
}

// ClientIP resolves the address of the machine that made the request,
// preferring the headers a reverse proxy sets over the socket address.
func ClientIP(r *http.Request) string {
	// X-Forwarded-For is a chain: the first entry is the original client.
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		if first, _, ok := strings.Cut(xff, ","); ok {
			return strings.TrimSpace(first)
		}
		return strings.TrimSpace(xff)
	}

	if xr := r.Header.Get("X-Real-IP"); xr != "" {
		return strings.TrimSpace(xr)
	}

	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
