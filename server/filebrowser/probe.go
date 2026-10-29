package filebrowser

import (
	"context"
	"encoding/json"
	"os"
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
// has not been touched keeps its answer, and a file that is replaced is read
// again.

// mediaInfo is what the browser needs to know about a file before it tries to
// play it.
type mediaInfo struct {
	// duration in seconds; zero when it cannot be determined
	duration float64
	// videoCodec is ffprobe's name for the video codec, e.g. "av1" or "h264"
	videoCodec string
	// audioCodec is ffprobe's name for the audio codec, e.g. "opus" or "aac"
	audioCodec string
	// source is the page the file came from, read out of the container's
	// tags. It is read by the same ffprobe call as the length and the
	// codecs, because a second call costs another process spawn — and
	// spawning ffprobe twice for every file is what made the file browser
	// take fifteen seconds to list a folder of seventeen files.
	source string
}

type probeCacheEntry struct {
	info mediaInfo
	// modTime and size are the file's own. A file that was replaced — a
	// download that finished over an older file of the same name — has the
	// same path, so the path alone is not an identity: the cache has to
	// notice that the file behind it changed.
	modTime time.Time
	size    int64
}

var (
	probeMu    sync.RWMutex
	probeCache = map[string]probeCacheEntry{}

	// probeGroup collapses concurrent probes of the same file. The file browser
	// is opened in more than one tab often enough for that to matter.
	probeGroup singleflight.Group
)

// mediaInfoFor returns the length and the codecs of a media file.
//
// The codecs matter as much as the length: the browser asks whether it can
// decode the file before playing it, and "av1" on a phone without an AV1
// decoder is a black screen with no message. The browser cannot find that out
// from the file name, so it has to be told.
//
// ffprobe is used rather than parsing the container by hand: it is already a
// dependency of ffmpeg, which yt-dlp needs for merging anyway, and it answers
// the question for every container.
func mediaInfoFor(path string) mediaInfo {
	// The file's own identity is read first, so the cache can be checked
	// against it rather than against the clock. A clock-only cache would
	// report a stale length for an hour after the file was replaced.
	info, err := os.Stat(path)
	if err != nil {
		return mediaInfo{}
	}

	// The memory cache first: it is free, and it is what a listing that
	// runs twice in a row reads.
	probeMu.RLock()
	entry, ok := probeCache[path]
	probeMu.RUnlock()

	if ok && entry.modTime.Equal(info.ModTime()) && entry.size == info.Size() {
		return entry.info
	}

	// Then the one on disk. It is what makes the first visit to the file
	// browser fast after a restart: the answers are still there from the
	// last time, and running ffprobe on every file again took three seconds
	// for thirteen files.
	if cached, ok := cachedProbeFor(path, info.ModTime(), info.Size()); ok {
		probed := mediaInfo{
			duration:   cached.Duration,
			videoCodec: cached.VideoCodec,
			audioCodec: cached.AudioCodec,
			source:     cached.Source,
		}

		probeMu.Lock()
		probeCache[path] = probeCacheEntry{
			info:    probed,
			modTime: info.ModTime(),
			size:    info.Size(),
		}
		probeMu.Unlock()

		return probed
	}

	value, _, _ := probeGroup.Do(path, func() (any, error) {
		return runFFprobe(path), nil
	})

	probed, _ := value.(mediaInfo)

	probeMu.Lock()
	probeCache[path] = probeCacheEntry{
		info:    probed,
		modTime: info.ModTime(),
		size:    info.Size(),
	}
	probeMu.Unlock()

	rememberProbe(path, info.ModTime(), info.Size(), probed)

	return probed
}

func runFFprobe(path string) mediaInfo {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "ffprobe",
		"-v", "error",
		// The disposition is what tells an attached picture — the cover art
		// some containers carry — from a real video stream. A file with a
		// cover image reports two video streams, and the first one is the
		// artwork.
		//
		// The tags are read in the same call. They are where the source
		// URL is, and asking for them in a second ffprobe cost another
		// process spawn for every file in the listing.
		"-show_entries", "stream=codec_type,codec_name,disposition:format=duration:format_tags=comment,description,title",
		"-of", "json",
		path,
	)

	out, err := cmd.Output()
	if err != nil {
		return mediaInfo{}
	}

	var parsed struct {
		Streams []struct {
			CodecType   string `json:"codec_type"`
			CodecName   string `json:"codec_name"`
			Disposition struct {
				AttachedPic int `json:"attached_pic"`
			} `json:"disposition"`
		} `json:"streams"`
		Format struct {
			Duration string `json:"duration"`
			Tags     map[string]string `json:"tags"`
		} `json:"format"`
	}

	if err := json.Unmarshal(out, &parsed); err != nil {
		return mediaInfo{}
	}

	info := mediaInfo{}

	for _, s := range parsed.Streams {
		switch s.CodecType {
		case "video":
			// Skip the artwork.
			if s.Disposition.AttachedPic == 1 || info.videoCodec != "" {
				continue
			}
			info.videoCodec = strings.ToLower(s.CodecName)
		case "audio":
			if info.audioCodec != "" {
				continue
			}
			info.audioCodec = strings.ToLower(s.CodecName)
		}
	}

	seconds, _ := strconv.ParseFloat(strings.TrimSpace(parsed.Format.Duration), 64)
	if seconds > 0 {
		info.duration = seconds
	}

	// The source is in the tags, and it is read here rather than by a
	// second ffprobe.
	if url, ok := sourceFromTags(parsed.Format.Tags); ok {
		info.source = url
	}

	return info
}
