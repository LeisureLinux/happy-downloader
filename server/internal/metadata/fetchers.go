package metadata

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"time"

	"github.com/LeisureLinux/happy-downloader/v4/server/common"
	"github.com/LeisureLinux/happy-downloader/v4/server/internal/downloaders"
	"github.com/LeisureLinux/happy-downloader/v4/server/internal/probeinfo"
)

func DefaultFetcher(url string) (*common.DownloadMetadata, error) {
	// The probe for this URL already ran a full extraction; reuse it instead of
	// making the user wait the same twenty seconds twice.
	if cached, ok := probeinfo.Recall(url); ok {
		slog.Info("metadata reused from the earlier probe", slog.String("url", url))
		return &cached, nil
	}

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
		// yt-dlp's "Unsupported URL" says which link it could not read
		// and nothing else. The rewritten message says whether the site is
		// one the application knows at all.
		return nil, errors.New(ExplainUnsupported(bufferedStderr.String()))
	}

	probeinfo.Remember(url, meta)

	return &meta, nil
}
