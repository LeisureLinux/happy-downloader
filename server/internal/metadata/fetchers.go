package metadata

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"time"

	"github.com/marcopiovanello/yt-dlp-web-ui/v4/server/common"
	"github.com/marcopiovanello/yt-dlp-web-ui/v4/server/internal/downloaders"
)

func DefaultFetcher(url string) (*common.DownloadMetadata, error) {
	// The metadata stage used to run with a bare "<url> -J", which meant it
	// resolved without the JS runtime, without cookies and without any of the
	// options the download stage applies. A URL could therefore fail here while
	// an identical manual yt-dlp call succeeded, with nothing in the log to
	// explain the difference. It now shares the same argument builder.
	args := downloaders.BuildArgs(url, "-J")
	downloaders.LogCommand("metadata", url, args)

	cmd := downloaders.NewCommand(args)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, err
	}

	meta := common.DownloadMetadata{
		URL:       url,
		CreatedAt: time.Now(),
	}

	if err := cmd.Start(); err != nil {
		return nil, err
	}

	var bufferedStderr bytes.Buffer

	go func() {
		io.Copy(&bufferedStderr, stderr)
	}()

	slog.Info("retrieving metadata", slog.String("url", url))

	if err := json.NewDecoder(stdout).Decode(&meta); err != nil {
		return nil, err
	}

	if err := cmd.Wait(); err != nil {
		return nil, errors.New(bufferedStderr.String())
	}

	return &meta, nil
}
