package updater

import (
	"context"
	"log/slog"
	"sync"
	"time"
)

// The cached answer to "is there a newer yt-dlp?".
//
// The check is a network call, and the footer asks for it every time the page
// loads. A page that is left open would otherwise ask on every reload, and
// the GitHub API rate-limits anonymous callers.
//
// A day is long enough: yt-dlp breaks YouTube every few weeks, and a release
// that came out an hour ago can wait until tomorrow.

var (
	cacheMu    sync.RWMutex
	cached     *Status
	cachedAt   time.Time
	cacheTTL   = 24 * time.Hour
	checkMutex sync.Mutex
)

// CheckOnce returns the cached answer, or makes the check.
//
// The result is cached whether the check succeeded or failed: a network that
// is down should not mean every page load waits for a timeout.
func CheckOnce(ctx context.Context, out *Status) {
	cacheMu.RLock()
	if cached != nil && time.Since(cachedAt) < cacheTTL {
		cachedCopy := *cached
		cacheMu.RUnlock()
		*out = cachedCopy
		return
	}
	cacheMu.RUnlock()

	// One check at a time. Several tabs loading at once would otherwise each
	// make the request, which is exactly what the cache is there to avoid.
	checkMutex.Lock()
	defer checkMutex.Unlock()

	cacheMu.RLock()
	if cached != nil && time.Since(cachedAt) < cacheTTL {
		cachedCopy := *cached
		cacheMu.RUnlock()
		*out = cachedCopy
		return
	}
	cacheMu.RUnlock()

	status, err := Check(ctx)

	cacheMu.Lock()
	cached = status
	cachedAt = time.Now()
	cacheMu.Unlock()

	if err != nil {
		slog.Debug("the yt-dlp update check failed", slog.Any("err", err))
	}

	*out = *status
}

// Invalidate forgets the cached answer.
//
// It is called after an update, so the next check reports the version that
// was just installed rather than the one that was available before it.
func Invalidate() {
	cacheMu.Lock()
	cached = nil
	cachedAt = time.Time{}
	cacheMu.Unlock()
}
