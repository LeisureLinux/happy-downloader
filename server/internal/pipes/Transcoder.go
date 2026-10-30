package pipes

import (
	"bufio"
	"errors"
	"io"
	"log/slog"
	"os/exec"
	"strings"

	"github.com/LeisureLinux/happy-downloader/v4/server/internal/tools"
)

type Transcoder struct {
	Args []string
}

func (t *Transcoder) Name() string { return "ffmpeg-transcoder" }

func (t *Transcoder) Connect(r io.Reader) (io.Reader, error) {
	// ffmpeg is looked up rather than named, so that a failure says "ffmpeg
	// is not installed" instead of the process failing to start with no
	// explanation. The same lookup is used when a recording is put into its
	// container, and the two should not differ on where ffmpeg comes from.
	ffmpeg := tools.FFmpeg()
	if ffmpeg == "" {
		return nil, errors.New("ffmpeg was not found")
	}

	cmd := exec.Command(ffmpeg,
		append([]string{"-i", "pipe:0"}, append(t.Args, "-f", "webm", "pipe:1")...)...,
	)
	tools.Quiet(cmd)

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, err
	}

	go func() {
		reader := bufio.NewReader(stderr)
		var line string

		for {
			part, err := reader.ReadString('\r')
			line += part
			if err != nil {
				break
			}

			line = strings.TrimRight(line, "\r\n")
			slog.Info("ffmpeg transcoder", slog.String("log", line))
			line = ""
		}
	}()

	go func() {
		defer stdin.Close()
		_, err := io.Copy(stdin, r)
		if err != nil && !errors.Is(err, io.EOF) {
			slog.Error("transcoder stdin error", slog.Any("err", err))
		}
	}()

	if err := cmd.Start(); err != nil {
		return nil, err
	}

	return stdout, nil
}
