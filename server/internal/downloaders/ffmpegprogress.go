package downloaders

import (
	"fmt"
	"log/slog"
	"regexp"
	"strconv"
	"strings"

	"github.com/LeisureLinux/happy-downloader/v4/server/internal"
)

// The progress of a livestream.
//
// A livestream is not a file that gets downloaded: it is a stream that is
// watched, and it does not end. So there is no percentage to report — a
// recording of nine minutes has no "90% done" — and the row showed a bar
// that never moved.
//
// What there is to report is how much has been recorded, and how fast it is
// arriving. Both are in the output ffmpeg writes while it copies the stream
// out:
//
//	frame=  3600 fps= 60 q=-1.0 size=  65536KiB time=00:01:00.00 bitrate=8192.0kbits/s speed=1.0x
//
// The time is how much of the recording there is, and the bitrate is how fast
// it is arriving. The progress is reported against the length that was asked
// for, so the bar fills up as the recording approaches it.

// The ffmpeg progress line.
//
// The fields are in a fixed order and the line ends with a carriage return
// rather than a newline, because it is written over itself as it goes.
var ffmpegProgressPattern = regexp.MustCompile(
	`time=(\d+):(\d\d):(\d\d(?:\.\d+)?)`)

// The bitrate, in kilobits per second.
var ffmpegBitratePattern = regexp.MustCompile(`bitrate=\s*([\d.]+)`)

// The speed, as a multiple of real time.
var ffmpegSpeedPattern = regexp.MustCompile(`speed=\s*([\d.]+)`)

// ffmpegLogConsumer reads the progress out of what ffmpeg writes.
//
// It is used for a livestream, where there is no progress template to read:
// the media is copied out of the stream by ffmpeg, and ffmpeg reports what it
// has done in its own words.
type ffmpegLogConsumer struct{}

func NewFFMpegLogConsumer() LogConsumer {
	return &ffmpegLogConsumer{}
}

func (f *ffmpegLogConsumer) GetName() string { return "ffmpeg-log-consumer" }

func (f *ffmpegLogConsumer) ParseLogEntry(entry []byte, d Downloader) {
	line := strings.TrimSpace(string(entry))

	seconds, ok := recordedSeconds(line)
	if !ok {
		// Not a progress line. The output ffmpeg writes about the
		// streams themselves is worth having in the log, but it is not
		// progress and it is not an error either.
		slog.Debug("ffmpeg output",
			slog.String("id", d.GetId()),
			slog.String("url", d.GetUrl()),
			slog.String("output", line),
		)
		return
	}

	progress := internal.DownloadProgress{
		Status: internal.StatusLiveStream,
		Speed:  bitrateOf(line),
		// The time recorded is what the row says. A percentage would be
		// measured against the length that was asked for, which is a
		// thing the recording may not reach — the stream might end
		// first. A time says what exists.
		Percentage: formatRecorded(seconds),
	}

	d.SetProgress(progress)
}

// recordedSeconds reads how much has been recorded out of an ffmpeg line.
//
// The time in the line is the position in the recording, which is the time
// the stream has been watched for.
func recordedSeconds(line string) (float64, bool) {
	m := ffmpegProgressPattern.FindStringSubmatch(line)
	if len(m) < 4 {
		return 0, false
	}

	hours, _ := strconv.ParseFloat(m[1], 64)
	minutes, _ := strconv.ParseFloat(m[2], 64)
	seconds, _ := strconv.ParseFloat(m[3], 64)

	return hours*3600 + minutes*60 + seconds, true
}

// formatRecorded writes how much has been recorded as a time.
func formatRecorded(seconds float64) string {
	total := int(seconds)

	return fmt.Sprintf("%02d:%02d:%02d", total/3600, (total%3600)/60, total%60)
}

// bitrateOf reads how fast the recording is arriving.
//
// ffmpeg writes the bitrate in kilobits and the speed as a multiple of real
// time. The interface shows megabytes per second, so the kilobits are
// converted here rather than in the browser.
//
// The speed is a multiple of real time and the bitrate is a rate, so they
// are different things: the bitrate is what the interface wants, and it is
// what is read.
func bitrateOf(line string) float64 {
	m := ffmpegBitratePattern.FindStringSubmatch(line)
	if len(m) < 2 {
		return 0
	}

	kilobits, err := strconv.ParseFloat(m[1], 64)
	if err != nil {
		return 0
	}

	// Kilobits to bytes.
	return kilobits * 1000 / 8
}
