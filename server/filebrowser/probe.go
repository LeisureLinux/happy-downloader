package filebrowser

import (
	"context"
	"encoding/json"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/sync/singleflight"
)

// Reading a media file's length takes a process spawn, and the file browser
// lists a whole directory. Running ffprobe on every file on every listing
// would make the page slow enough to be noticeable, and the answer for a given
// file never changes.
//
// So the lengths are cached, keyed by path and modification time: a file that
// has not been touched keeps its answer, and a file that was replaced is read
// again.

type probeCacheEntry struct {
	duration float64
	storedAt time.Time
}

var (
	probeMu    sync.RWMutex
	probeCache = map[string]probeCacheEntry{}

	// probeGroup collapses concurrent probes of the same file. The file browser
	// is opened in more than one tab often enough for that to matter.
	probeGroup singleflight.Group
)

// mediaDuration returns the length of a media file in seconds, or zero when it
// cannot be determined.
//
// ffprobe is used rather than parsing the container by hand: it is already a
// dependency of ffmpeg, which yt-dlp needs for merging anyway, and it answers
// the question for every container.
func mediaDuration(path string) float64 {
	probeMu.RLock()
	entry, ok := probeCache[path]
	probeMu.RUnlock()

	if ok && time.Since(entry.storedAt) < time.Hour {
		return entry.duration
	}

	value, _, _ := probeGroup.Do(path, func() (any, error) {
		return runFFprobeDuration(path), nil
	})

	duration, _ := value.(float64)

	probeMu.Lock()
	probeCache[path] = probeCacheEntry{duration: duration, storedAt: time.Now()}
	probeMu.Unlock()

	return duration
}

func runFFprobeDuration(path string) float64 {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "ffprobe",
		"-v", "error",
		"-show_entries", "format=duration",
		"-of", "json",
		path,
	)

	out, err := cmd.Output()
	if err != nil {
		return 0
	}

	var parsed struct {
		Format struct {
			Duration string `json:"duration"`
		} `json:"format"`
	}

	if err := json.Unmarshal(out, &parsed); err != nil {
		return 0
	}

	seconds, err := strconv.ParseFloat(strings.TrimSpace(parsed.Format.Duration), 64)
	if err != nil || seconds <= 0 {
		return 0
	}

	return seconds
}
