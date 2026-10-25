package formats

import (
	"encoding/json"
	"log/slog"
	"sync"

	"github.com/LeisureLinux/happy-downloader/v4/server/internal/downloaders"
)

func ParseURL(url string) (*Metadata, error) {
	// Same builder as the metadata and download stages: the format probe has to
	// resolve under the same conditions, or a URL can list formats and then
	// fail to download them.
	args := downloaders.BuildArgs(url, "-J")
	downloaders.LogCommand("formats", url, args)

	stdout, err := downloaders.NewCommand(args).Output()
	if err != nil {
		slog.Error("failed to retrieve metadata", slog.String("err", err.Error()))
		return nil, err
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

	return info, nil
}
