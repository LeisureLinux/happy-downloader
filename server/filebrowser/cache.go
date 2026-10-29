package filebrowser

import (
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/LeisureLinux/happy-downloader/v4/server/config"
)

// The probe cache, on disk.
//
// Reading a media file's length and codecs means running ffprobe on it, which
// takes a couple of hundred milliseconds. The answers do not change while the
// file does not, so they are remembered.
//
// They were remembered in memory, which meant that every restart of the server
// forgot them: the first visit to the file browser after a restart took three
// seconds for thirteen files, and the person waiting for it had no idea why.
//
// The cache is written next to the application's other state, keyed by the
// file's path, its modification time and its size. A file that was replaced is
// read again, and a file that was deleted leaves an entry behind that is
// dropped when the cache is next written.

// probeCacheFile is the name of the cache inside the state directory.
const probeCacheFile = "media-probes.json"

// cachedProbe is what is remembered about one file.
type cachedProbe struct {
	// Duration in seconds.
	Duration float64 `json:"duration"`
	// VideoCodec and AudioCodec are ffprobe's names.
	VideoCodec string `json:"videoCodec,omitempty"`
	AudioCodec string `json:"audioCodec,omitempty"`
	// Source is the page the file came from, read out of the container's
	// tags. It is remembered with the rest, because reading it costs a
	// process spawn: the file browser listed a folder of seventeen files in
	// fifteen seconds because every one of them was read twice.
	Source string `json:"source,omitempty"`
	// ModTime and Size identify the file the answers belong to.
	ModTime time.Time `json:"modTime"`
	Size    int64     `json:"size"`
}

// probeDiskCache is the cache as it is written to disk.
type probeDiskCache struct {
	Entries map[string]cachedProbe `json:"entries"`
}

// probeCachePath returns where the cache is written.
//
// It is next to the application's other state, and it is not in the download
// directory: the file browser would list it.
func probeCachePath() string {
	dir := config.Instance().Paths.LocalDatabasePath
	if dir == "" {
		dir = os.TempDir()
	}

	return filepath.Join(dir, probeCacheFile)
}

// loadProbeCache reads the cache from disk.
//
// A cache that cannot be read is not a failure: the answers are a
// convenience, and the probe is run again for the files that are missing.
func loadProbeCache() map[string]cachedProbe {
	out := map[string]cachedProbe{}

	content, err := os.ReadFile(probeCachePath())
	if err != nil {
		return out
	}

	var disk probeDiskCache
	if err := json.Unmarshal(content, &disk); err != nil {
		return out
	}

	for path, entry := range disk.Entries {
		out[path] = entry
	}

	return out
}

// saveProbeCache writes the cache to disk.
//
// It is written on a timer rather than after every probe, because a listing
// of a directory that has just been filled would write it once per file.
func saveProbeCache(entries map[string]cachedProbe) {
	content, err := json.Marshal(probeDiskCache{Entries: entries})
	if err != nil {
		slog.Debug("could not encode the probe cache", slog.Any("err", err))
		return
	}

	path := probeCachePath()

	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		slog.Debug("could not create the probe cache directory", slog.Any("err", err))
		return
	}

	tmp := path + ".tmp"

	if err := os.WriteFile(tmp, content, 0o600); err != nil {
		slog.Debug("could not write the probe cache", slog.Any("err", err))
		return
	}

	if err := os.Rename(tmp, path); err != nil {
		os.Remove(tmp)
		slog.Debug("could not replace the probe cache", slog.Any("err", err))
	}
}

// the disk cache, and the timer that writes it.
var (
	diskCacheMu sync.Mutex
	diskCache   = map[string]cachedProbe{}
	cacheLoaded bool
)

// loadDiskCacheOnce reads the cache the first time it is needed.
func loadDiskCacheOnce() {
	diskCacheMu.Lock()
	defer diskCacheMu.Unlock()

	if cacheLoaded {
		return
	}

	diskCache = loadProbeCache()
	cacheLoaded = true

	if len(diskCache) > 0 {
		slog.Info("the media probe cache was loaded",
			slog.Int("entries", len(diskCache)),
		)
	}
}

// cachedProbeFor returns what is remembered about a file, if anything.
func cachedProbeFor(path string, modTime time.Time, size int64) (cachedProbe, bool) {
	loadDiskCacheOnce()

	diskCacheMu.Lock()
	defer diskCacheMu.Unlock()

	entry, ok := diskCache[path]
	if !ok {
		return cachedProbe{}, false
	}

	if !entry.ModTime.Equal(modTime) || entry.Size != size {
		return cachedProbe{}, false
	}

	return entry, true
}

// rememberProbe records what a probe found.
func rememberProbe(path string, modTime time.Time, size int64, info mediaInfo) {
	loadDiskCacheOnce()

	diskCacheMu.Lock()
	diskCache[path] = cachedProbe{
		Duration:   info.duration,
		VideoCodec: info.videoCodec,
		AudioCodec: info.audioCodec,
		Source:     info.source,
		ModTime:    modTime,
		Size:       size,
	}
	diskCacheMu.Unlock()
}

// StartProbeCacheWriter writes the cache to disk every so often.
//
// A minute is often enough: the cache is a convenience, and losing the last
// minute of it costs one probe per file.
func StartProbeCacheWriter() {
	go func() {
		ticker := time.NewTicker(time.Minute)
		defer ticker.Stop()

		for range ticker.C {
			loadDiskCacheOnce()

			diskCacheMu.Lock()
			snapshot := make(map[string]cachedProbe, len(diskCache))
			for path, entry := range diskCache {
				snapshot[path] = entry
			}
			diskCacheMu.Unlock()

			// The entries of files that are gone are dropped: the cache is
			// about the files that are there.
			saveProbeCache(snapshot)
		}
	}()
}
