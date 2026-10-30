package rest

import (
	"context"
	"log/slog"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/LeisureLinux/happy-downloader/v4/server/internal/tools"
)

// The yt-dlp version, and how long it is worth remembering.
//
// Asking yt-dlp for its version is not free. The standalone binary takes
// four seconds to start — it is a self-contained PyInstaller bundle that
// unpacks itself — and the pip-installed one takes nearly two. The version
// endpoint is called on every page load, because the footer shows it, so the
// page waited four seconds for it.
//
// The version does not change between updates, so it is remembered: the first
// call asks, and the rest are answered from memory. The cache is dropped
// when yt-dlp is updated, because that is the only moment the answer can
// change.
var (
	versionMu    sync.RWMutex
	versionCache string
	versionAt    time.Time
)

// The version is remembered for an hour. It cannot change without an update,
// and an update drops the cache by itself, so this is only a bound on how
// long a stale answer survives a restart that happened to miss the drop.
const versionCacheTTL = time.Hour

// A known version is never held up by asking again.
//
// The request that arrives when the answer is stale gets the remembered one,
// and the answer is refreshed in the background. Blocking that request is how
// the version endpoint came to take twenty seconds on a single-board computer
// that was busy converting a cookie file: the footer waited, the page waited,
// and a health check that used the same endpoint decided the program was not
// running at all.
//
// The first call has nothing to answer with, so it does wait. That wait is
// kept short by asking once at startup: on a single-board computer the answer
// can take fifteen seconds, which is longer than a page should be, so it is
// asked for in the background and the page finds it already known.
//
// A failed read is not remembered, so the next call tries again rather than
// caching the failure.
func ytDlpVersion(ctx context.Context) string {
	versionMu.RLock()
	cached := versionCache
	fresh := time.Since(versionAt) < versionCacheTTL
	versionMu.RUnlock()

	if cached != "" {
		if !fresh {
			refreshInBackground()
		}

		return cached
	}

	return readVersion(ctx)
}

// WarmYtdlpVersion asks for the version without anybody waiting for it.
//
// It is called once at startup, so that the first page load finds the answer
// already known instead of waiting for yt-dlp to start.
func WarmYtdlpVersion() {
	refreshInBackground()
}

// refreshing stops a burst of requests from each starting a yt-dlp.
var refreshing sync.Mutex

func refreshInBackground() {
	if !refreshing.TryLock() {
		return
	}

	go func() {
		defer refreshing.Unlock()

		// Detached from the request that asked: the request is already
		// answered, so cancelling it must not cancel this.
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		readVersion(ctx)
	}()
}

func readVersion(ctx context.Context) string {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, tools.YtDlp(), "--version")
	tools.Quiet(cmd)

	out, err := cmd.Output()
	if err != nil {
		// A yt-dlp that will not report its version is a yt-dlp that will not
		// download either. Nothing is remembered, so the next call tries
		// again; saying so once is enough to explain an empty footer.
		slog.Warn("could not read the yt-dlp version", slog.Any("err", err))

		return ""
	}

	// yt-dlp prints a bare date; some channels prefix a "v". Strip it so the
	// footer does not render "yt-dlp vv2026.10.21".
	version := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(string(out)), "v"))

	versionMu.Lock()
	versionCache = version
	versionAt = time.Now()
	versionMu.Unlock()

	return version
}

// ForgetYtdlpVersion drops the remembered version.
//
// It is called after an update, which is the only moment the answer can
// change. The RPC package calls it, so it is exported.
func ForgetYtdlpVersion() {
	versionMu.Lock()
	versionCache = ""
	versionAt = time.Time{}
	versionMu.Unlock()
}
