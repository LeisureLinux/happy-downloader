package probeinfo

import (
	"sync"
	"time"

	"github.com/LeisureLinux/happy-downloader/v4/server/common"
	"github.com/LeisureLinux/happy-downloader/v4/server/internal"
)

// A probe result is expensive — the metadata stage runs a full yt-dlp extraction
// and, on a host whose only route to the site is a slow proxy, that is twenty to
// thirty seconds. The download stage then ran the same extraction again before
// fetching a byte, so pressing "start" cost the wait a second time for a result
// the server had just produced.
//
// A short-lived cache keyed by URL removes that second wait. Short-lived because
// the useful fields are the title and the thumbnail; formats change when the
// source re-encodes, and serving a stale format list is worse than waiting.
const (
	cacheTTL     = 10 * time.Minute
	cacheMaxSize = 64
)

type cacheEntry struct {
	meta     common.DownloadMetadata
	storedAt time.Time

	// duration in seconds, and the caption tracks the source offered. Both
	// come from the same probe and are carried alongside so the download stage
	// can report them without running the extraction again.
	duration float64
	captions []internal.CaptionTrack
}

var (
	cacheMu sync.RWMutex
	cache   = make(map[string]cacheEntry, cacheMaxSize)
)

// Remember stores a freshly fetched result for the download stage to reuse.
func Remember(url string, meta common.DownloadMetadata) {
	RememberFull(url, meta, 0, nil)
}

// RememberFull stores a result together with what the probe learned about it.
func RememberFull(url string, meta common.DownloadMetadata, duration float64, captions []internal.CaptionTrack) {
	cacheMu.Lock()
	defer cacheMu.Unlock()

	if len(cache) >= cacheMaxSize {
		evictOldestLocked()
	}

	cache[url] = cacheEntry{
		meta:     meta,
		storedAt: time.Now(),
		duration: duration,
		captions: captions,
	}
}

// Recall returns a cached result when one is still fresh.
func Recall(url string) (common.DownloadMetadata, bool) {
	entry, ok := recallEntry(url)
	if !ok {
		return common.DownloadMetadata{}, false
	}

	return entry.meta, true
}

// RecallFull returns a cached result with the probe's duration and captions.
func RecallFull(url string) (common.DownloadMetadata, float64, []internal.CaptionTrack, bool) {
	entry, ok := recallEntry(url)
	if !ok {
		return common.DownloadMetadata{}, 0, nil, false
	}

	return entry.meta, entry.duration, entry.captions, true
}

func recallEntry(url string) (cacheEntry, bool) {
	cacheMu.RLock()
	entry, ok := cache[url]
	cacheMu.RUnlock()

	if !ok || time.Since(entry.storedAt) > cacheTTL {
		return cacheEntry{}, false
	}

	return entry, true
}

func evictOldestLocked() {
	var (
		oldestKey string
		oldest    time.Time
	)

	for k, v := range cache {
		if oldest.IsZero() || v.storedAt.Before(oldest) {
			oldestKey, oldest = k, v.storedAt
		}
	}

	if oldestKey != "" {
		delete(cache, oldestKey)
	}
}
