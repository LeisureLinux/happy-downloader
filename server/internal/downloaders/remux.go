package downloaders

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/LeisureLinux/happy-downloader/v4/server/internal/tools"

	"github.com/LeisureLinux/happy-downloader/v4/server/config"
)

// Putting a recording into a container that plays.
//
// A Twitch stream arrives as MPEG-TS — that is what an HLS stream is — and
// the recording is written out as it comes.
//
// A file called ".mp4" that holds MPEG-TS is a file that will not play:
// the container says one thing and the bytes are another, and the player
// believes the extension. So the recording is put into an MP4 after it is
// finished.
//
// Nothing is re-encoded. The streams are copied across as they are, so it
// takes a second rather than as long as the recording, and the quality is
// exactly what was streamed.
//
// If ffmpeg is not there, the recording is left as it is, with the extension
// that says what it is. A file that plays in a player that knows MPEG-TS is
// better than no file.
func remuxRecording(from, to string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	ffmpeg := tools.FFmpeg()
	if ffmpeg == "" {
		return errors.New("ffmpeg was not found")
	}

	cmd := exec.CommandContext(ctx, ffmpeg,
		"-hide_banner", "-loglevel", "error",
		"-i", from,
		"-c", "copy",
		"-movflags", "+faststart",
		to,
	)

	if err := cmd.Run(); err != nil {
		return err
	}

	return nil
}

// The name the recording is known by, and where it is written.
//
// It ends in ".mp4" when the recording could be put into an MP4, and in
// ".ts" when it could not — a file whose extension lies about its contents
// is worse than one whose extension does not.
func (l *LiveStreamDownloader) recordingPath() (string, string) {
	name := sanitizeFilename(l.Title())
	when := l.startedAt.Format("2006-01-02 15-04-05")

	// A recording goes where the request asked for, like any other download.
	dir := config.Instance().Paths.DownloadPath
	if l.output.Path != "" {
		dir = l.output.Path
	}

	// The stream is written as it arrives, into the partial directory.
	//
	// It used to be written into the download directory, which meant the
	// recording appeared in the file browser as soon as stop was pressed —
	// as a ".ts" that became a ".mp4" a few seconds later. The file was
	// there, listed, and not what it said it was; and a person watching for
	// their recording saw a file that looked finished and was.
	//
	// Nothing half-made belongs in the directory the browser lists.
	raw := filepath.Join(
		tempDir(),
		fmt.Sprintf("%s %s.ts", name, when),
	)

	// And the one that is kept is the same thing in an MP4, if ffmpeg can
	// put it there.
	final := filepath.Join(
		dir,
		fmt.Sprintf("%s %s.mp4", name, when),
	)

	return raw, final
}

// finishRecording puts the recording into its final file.
//
// The raw file is removed afterwards: it is the same content, and leaving it
// behind would put two copies of every recording in the download directory.
func (l *LiveStreamDownloader) finishRecording(raw, final string) string {
	// A file with nothing in it is not a recording.
	//
	// It happens when the stream never delivered anything: the connection
	// never got going, or the recording was stopped while yt-dlp was still
	// negotiating one. Keeping it puts a 0-byte file in the browser, named
	// after a recording that does not exist.
	if info, err := os.Stat(raw); err != nil || info.Size() == 0 {
		os.Remove(raw)
		return ""
	}

	if err := remuxRecording(raw, final); err != nil {
		// No ffmpeg, or it failed. The raw file is still a recording, and
		// its extension says what it is.
		//
		// The failure is logged rather than swallowed: the file left behind
		// is not the one that was expected, and without this line there was
		// nothing anywhere saying why the recording is a ".ts".
		slog.Error("could not put the recording into its container",
			slog.String("id", l.Id),
			slog.String("raw", raw),
			slog.String("err", err.Error()),
		)

		// The raw file is moved next to where the finished one would have
		// been, out of the partial directory: that is where the temporary
		// files live and they are cleaned up, and a recording left there is
		// one that exists and then does not.
		//
		// It takes the name it was going to have. While the recording runs
		// its file is called "recording-<id>", because the title is not known
		// yet; a kept recording is finished, so it gets the real name.
		kept := strings.TrimSuffix(final, ".mp4") + ".ts"

		if err := os.Rename(raw, kept); err != nil {
			slog.Error("could not keep the recording",
				slog.String("id", l.Id),
				slog.String("raw", raw),
				slog.String("err", err.Error()),
			)
			return raw
		}

		return kept
	}

	os.Remove(raw)

	return final
}
