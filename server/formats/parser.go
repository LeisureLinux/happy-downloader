package formats

import (
	"bytes"
	"encoding/json"
	"errors"
	"log/slog"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/LeisureLinux/happy-downloader/v4/server/common"
	"github.com/LeisureLinux/happy-downloader/v4/server/internal"
	"github.com/LeisureLinux/happy-downloader/v4/server/internal/downloaders"
	"github.com/LeisureLinux/happy-downloader/v4/server/internal/probeinfo"
)

func ParseURL(url string) (*Metadata, error) {
	// Same builder as the metadata and download stages: the format probe has to
	// resolve under the same conditions, or a URL can list formats and then
	// fail to download them.
	args := downloaders.BuildArgs(url, "-J")
	downloaders.LogCommand("formats", url, args)

	// Capture stderr: "exit status 1" on its own says nothing about why, and
	// the reason is the whole point of a diagnostic.
	var stderr bytes.Buffer
	cmd := downloaders.NewCommand(args)
	cmd.Stderr = &stderr

	stdout, err := cmd.Output()
	if err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		slog.Error("failed to retrieve metadata",
			slog.String("url", url), slog.String("err", msg))
		return nil, errors.New(msg)
	}

	slog.Info(
		"retrieving metadata",
		slog.String("caller", "getFormats"),
		slog.String("url", url),
	)

	info := &Metadata{URL: url}
	best := &Format{}

	var (
		wg            sync.WaitGroup
		decodingError error
	)

	wg.Add(2)

	go func() {
		decodingError = json.Unmarshal(stdout, &info)
		wg.Done()
	}()

	go func() {
		decodingError = json.Unmarshal(stdout, &best)
		wg.Done()
	}()

	wg.Wait()

	if decodingError != nil {
		return nil, err
	}

	info.Best = *best

	// Hand the result to the download stage. Without this, pressing "start"
	// runs the same extraction again and the user waits twice for one answer.
	probeinfo.RememberFull(url, common.DownloadMetadata{
		URL:        url,
		Title:      info.Title,
		Thumbnail:  info.Thumbnail,
		Resolution: best.Resolution,
		Size:       int32(best.Size),
		VCodec:     best.VCodec,
		ACodec:     best.ACodec,
		Extension:  best.Ext,
		CreatedAt:  time.Now(),
	}, info.Duration, CaptionTracks(info.Subtitles, info.AutomaticCaptions))

	return info, nil
}

// CaptionTracks turns the probe's caption maps into a flat list.
//
// yt-dlp reports captions as a map from language code to the formats available
// for it. The file browser and the player want a list, and the human-readable
// name is useful: "Chinese (Simplified)" says more than "zh-Hans".
func CaptionTracks(subtitles, automatic map[string]any) []internal.CaptionTrack {
	var tracks []internal.CaptionTrack

	add := func(source map[string]any, auto bool) {
		for lang, value := range source {
			track := internal.CaptionTrack{Language: lang, Automatic: auto}

			// The value is a list of format objects; the first one carries the
			// name. It is not worth failing over: a track without a label is
			// still playable.
			if list, ok := value.([]any); ok && len(list) > 0 {
				if first, ok := list[0].(map[string]any); ok {
					if name, ok := first["name"].(string); ok {
						track.Name = name
					}
				}
			}

			tracks = append(tracks, track)
		}
	}

	add(subtitles, false)
	add(automatic, true)

	sort.Slice(tracks, func(i, j int) bool {
		if tracks[i].Automatic != tracks[j].Automatic {
			return !tracks[i].Automatic
		}
		return tracks[i].Language < tracks[j].Language
	})

	return tracks
}
