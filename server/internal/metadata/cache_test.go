package metadata

import (
	"testing"
	"time"

	"github.com/LeisureLinux/happy-downloader/v4/server/common"
)

func TestRememberAndRecall(t *testing.T) {
	url := "https://example.com/a"
	Remember(url, common.DownloadMetadata{URL: url, Title: "A"})

	got, ok := Recall(url)
	if !ok {
		t.Fatal("expected a hit")
	}
	if got.Title != "A" {
		t.Errorf("Title = %q, want A", got.Title)
	}
}

func TestRecallMissesUnknownURL(t *testing.T) {
	if _, ok := Recall("https://example.com/never-seen"); ok {
		t.Error("unexpected hit")
	}
}

// A stale format list is worse than waiting: the source re-encodes and the
// cached one would no longer match what a download produces.
func TestRecallExpires(t *testing.T) {
	url := "https://example.com/old"
	Remember(url, common.DownloadMetadata{URL: url, Title: "old"})

	cacheMu.Lock()
	e := cache[url]
	e.storedAt = time.Now().Add(-cacheTTL - time.Minute)
	cache[url] = e
	cacheMu.Unlock()

	if _, ok := Recall(url); ok {
		t.Error("expected the entry to have expired")
	}
}

func TestCacheEvictsWhenFull(t *testing.T) {
	cacheMu.Lock()
	cache = make(map[string]cacheEntry, cacheMaxSize)
	cacheMu.Unlock()

	for i := 0; i < cacheMaxSize+5; i++ {
		u := "https://example.com/" + string(rune('a'+i%26)) + string(rune('0'+i/26))
		Remember(u, common.DownloadMetadata{URL: u})
	}

	cacheMu.RLock()
	size := len(cache)
	cacheMu.RUnlock()

	if size > cacheMaxSize {
		t.Errorf("cache grew to %d, want at most %d", size, cacheMaxSize)
	}
}
