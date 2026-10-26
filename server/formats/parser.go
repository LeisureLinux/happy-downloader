package formats

import (
	"bytes"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"
	"sync"

	"github.com/LeisureLinux/happy-downloader/v4/server/internal/downloaders"
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

	return info, nil
}
