package downloaders

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"slices"
	"strings"
	"syscall"
	"time"

	"github.com/LeisureLinux/happy-downloader/v4/server/common"
	"github.com/LeisureLinux/happy-downloader/v4/server/config"
	"github.com/LeisureLinux/happy-downloader/v4/server/internal"
	"github.com/LeisureLinux/happy-downloader/v4/server/internal/pipes"
	"github.com/google/uuid"
)

type LiveStreamDownloader struct {
	progress internal.DownloadProgress

	proc *os.Process

	// stopped says the recording was stopped on purpose, so its exit code
	// means nothing.
	stopped bool

	logConsumer LogConsumer

	pipes []pipes.Pipe

	// Duration is how long to record, in minutes. Zero means until the
	// stream ends or the recording is stopped.
	//
	// A livestream does not end, so "download it" is a question without an
	// answer unless a length is given: the recording went on until somebody
	// noticed. yt-dlp has no option for it in this version, so the length
	// is kept here and the process is stopped when it has passed.
	Duration int

	// The file the recording is written to, and the name it is known by.
	outputPath string
	outputName string

	// startedAt is when the recording began. The name of the file carries
	// it, and it is settled when the recording ends — so it has to be
	// remembered from the beginning.
	startedAt time.Time

	// embedded
	DownloaderBase
}

func NewLiveStreamDownloader(url string, pipes []pipes.Pipe) Downloader {
	l := &LiveStreamDownloader{
		logConsumer: NewFFMpegLogConsumer(),
		pipes:       pipes,
	}
	// in base
	l.Id = uuid.NewString()
	l.URL = url
	l.pipes = pipes
	return l
}

func (l *LiveStreamDownloader) Start() error {
	l.SetPending(true)
	l.startedAt = time.Now()

	// Same builder as the other stages, so the runtime, the cookie and the
	// proxy cannot drift apart between them.
	//
	// The proxy matters most here: a livestream is reached through it, and
	// the monitoring stage that decides whether the stream is live at all
	// needs it too.
	baseParams := BuildArgs(l.URL,
		"--newline",
		"--no-colors",
		"--no-playlist",
		"--no-exec",
	)

	params := append(baseParams, "-o", "-")

	LogCommand("livestream", l.URL, params)

	cmd := exec.Command(config.Instance().Paths.DownloaderPath, params...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	// stdout = media stream
	media, err := cmd.StdoutPipe()
	if err != nil {
		slog.Error("failed to get media stdout", slog.Any("err", err))
		panic(err)
	}

	// stderr = log/progress
	stderr, err := cmd.StderrPipe()
	if err != nil {
		slog.Error("failed to get stderr pipe", slog.Any("err", err))
		panic(err)
	}

	if err := cmd.Start(); err != nil {
		slog.Error("failed to start yt-dlp process", slog.Any("err", err))
		panic(err)
	}

	l.proc = cmd.Process

	ctx, cancel := context.WithCancel(context.Background())
	defer func() {
		l.Complete()
		cancel()
	}()

	// build pipeline
	reader := io.Reader(media)
	for _, pipe := range l.pipes {
		nr, err := pipe.Connect(reader)
		if err != nil {
			slog.Error("pipe failed", slog.String("pipe", pipe.Name()), slog.Any("err", err))
			return err
		}
		reader = nr
	}

	if !l.hasFileWriter() {
		// The recording is written to a file of its own, with the title and
		// the time in its name.
		//
		// It used to be written to the download directory itself — the
		// path was built and thrown away, and the file was created with an
		// empty name. Every recording landed on top of the last one, and
		// the file browser never saw them.
		//
		// The name is settled when the recording ends rather than when it
		// starts: the title arrives with the metadata, which is a moment
		// after the stream begins, so a file named at the start was called
		// "Twitch (live)" whatever the channel was.
		go func() {
			raw, final := l.outputPath, ""
			if raw == "" {
				raw, final = l.recordingPath()
			}

			f, err := os.Create(raw)
			if err != nil {
				slog.Error("failed to create the recording file", slog.Any("err", err))
				return
			}

			_, err = io.Copy(f, reader)
			f.Close()

			if err != nil {
				slog.Error("copy error", slog.Any("err", err))
			}

			// The recording is put into an MP4, so that it plays.
			//
			// The media comes out of the stream as MPEG-TS and the file
			// was named as an MP4, which is a file whose extension lies
			// about its contents — and nothing plays it.
			if final != "" {
				final = l.finishRecording(raw, final)
			}

			l.UpdateSavedFilePath(final)

			// Where the recording came from.
			//
			// Nothing else records it: the recording is written from the
			// media stream rather than by yt-dlp's own postprocessing, so
			// the hook that records a downloaded file's source is never
			// called for it. A recording with no source showed the globe
			// icon in the file browser rather than the site's.
			RememberSource(l, final)

			slog.Info("the recording was saved", slog.String("path", final))
		}()
	}

	logs := make(chan []byte)
	go produceLogs(stderr, logs)
	go consumeLogs(ctx, logs, l.logConsumer, l)

	l.progress.Status = internal.StatusLiveStream

	// The recording is stopped when the time is up.
	//
	// yt-dlp has no option for it in this version, so the length is kept
	// here: a livestream does not end, so "download it" means "record it for
	// this long" — otherwise the recording went on until somebody noticed.
	//
	// The recording is also stopped when the stream itself ends, which is
	// what yt-dlp does when the stream drops.
	stopped := make(chan struct{})

	if l.Duration > 0 {
		go func() {
			select {
			case <-stopped:
				return
			case <-time.After(time.Duration(l.Duration) * time.Minute):
			}

			slog.Info("the recording has run for its time",
				slog.String("id", l.Id),
				slog.Int("minutes", l.Duration),
			)

			if err := l.Stop(); err != nil {
				slog.Debug("could not stop the recording", slog.Any("err", err))
			}
		}()
	}

	waitErr := cmd.Wait()
	close(stopped)

	// A recording that was stopped on purpose is not a failure.
	//
	// The process is killed, so it exits with an error, and the error was
	// reported as a failed download — which is what a recording that ran its
	// time always looked like.
	if l.stopped {
		l.progress.Status = internal.StatusCompleted
		l.Complete()
		return nil
	}

	if waitErr != nil {
		slog.Error("the recording failed", slog.String("id", l.Id), slog.Any("err", waitErr))
		l.progress.Status = internal.StatusErrored
		return waitErr
	}

	l.progress.Status = internal.StatusCompleted
	l.Complete()

	return nil
}

// Title returns the name the recording is known by.
//
// It is the video's title, and it is what the file is named after — a
// recording called "download-1 (live)" is not something anybody can find
// again.
func (l *LiveStreamDownloader) Title() string {
	if l.Metadata.Title != "" {
		return l.Metadata.Title
	}

	return "Twitch"
}

// sanitizeFilename makes a string usable as a file name.
//
// The "(live)" the title of a Twitch stream carries is dropped: the file is
// a recording and the name says so, but a title that already ends with it
// would otherwise be given it twice.
func sanitizeFilename(name string) string {
	name = strings.TrimSpace(name)
	name = strings.TrimSuffix(name, " (live)")
	name = strings.TrimSuffix(name, "（直播）")
	replacer := strings.NewReplacer(
		"/", "_",
		"\\", "_",
		":", "_",
		"*", "_",
		"?", "_",
		"\"", "_",
		"<", "_",
		">", "_",
		"|", "_",
		"\n", " ",
		"\r", " ",
	)

	cleaned := strings.TrimSpace(replacer.Replace(name))
	if cleaned == "" {
		return "Twitch"
	}

	// A name that is too long is refused by the filesystem, and the
	// recording would fail at the last moment.
	if len(cleaned) > 120 {
		cleaned = cleaned[:120]
	}

	return cleaned
}

func (l *LiveStreamDownloader) Stop() error {
	l.stopped = true

	defer func() {
		l.progress.Status = internal.StatusCompleted
		l.Complete()
	}()
	// yt-dlp uses multiple child process the parent process
	// has been spawned with setPgid = true. To properly kill
	// all subprocesses a SIGTERM need to be sent to the correct
	// process group
	if l.proc == nil {
		return errors.New("*os.Process not set")
	}

	pgid, err := syscall.Getpgid(l.proc.Pid)
	if err != nil {
		return err
	}
	if err := syscall.Kill(-pgid, syscall.SIGTERM); err != nil {
		return err
	}

	return nil
}

func (l *LiveStreamDownloader) Status() internal.ProcessSnapshot {
	return internal.ProcessSnapshot{
		Id: l.Id,
		// The URL is what says where the recording came from, and the
		// file browser shows the site's icon because of it. It was left
		// out, so a recording had no source and showed the globe.
		URL:            l.URL,
		Info:           l.Metadata,
		Progress:       l.progress,
		DownloaderName: "livestream",
	}
}

func (l *LiveStreamDownloader) UpdateSavedFilePath(p string) {}

func (l *LiveStreamDownloader) UpdateOutputDimensions(width, height float64) {}

func (l *LiveStreamDownloader) SetOutput(o internal.DownloadOutput)     {}
func (l *LiveStreamDownloader) SetProgress(p internal.DownloadProgress) { l.progress = p }

func (l *LiveStreamDownloader) SetMetadata(fetcher func(url string) (*common.DownloadMetadata, error)) {
	l.FetchMetadata(fetcher)
}

func (l *LiveStreamDownloader) SetPending(p bool) {
	l.Pending = p
}

func (l *LiveStreamDownloader) GetId() string  { return l.Id }
func (l *LiveStreamDownloader) GetUrl() string { return l.URL }

// A livestream has no probe to learn a length or a caption list from.
// SetRecordMinutes says how long the recording should run.
//
// Zero means until the stream ends or the recording is stopped.
//
// It is not called SetDuration because that name is taken: it is the length
// of a video, which is a different thing — a recording has no length until
// it is over.
func (l *LiveStreamDownloader) SetRecordMinutes(minutes int) { l.Duration = minutes }

// SetDuration records the length of the video, in seconds.
//
// A recording has no length until it is over, so this is not what the
// recording limit is set with — that is SetRecordMinutes.
func (l *LiveStreamDownloader) SetDuration(float64)                 {}
func (l *LiveStreamDownloader) SetCaptions([]internal.CaptionTrack) {}

func (l *LiveStreamDownloader) RestoreFromSnapshot(snap *internal.ProcessSnapshot) error {
	if snap == nil {
		return errors.New("cannot restore nil snapshot")
	}

	s := *snap

	l.Id = s.Id
	l.Metadata = s.Info
	l.progress = s.Progress

	l.URL = s.URL
	if l.URL == "" {
		l.URL = s.Info.URL
	}

	return nil
}

func (l *LiveStreamDownloader) IsCompleted() bool { return l.Completed }

// IsPaused reports whether this download was stopped on purpose. A live
// stream cannot be resumed — it is gone by the time anyone asks — so it is
// never paused.
func (l *LiveStreamDownloader) IsPaused() bool { return false }

func (l *LiveStreamDownloader) hasFileWriter() bool {
	return slices.ContainsFunc(l.pipes, func(p pipes.Pipe) bool {
		return p.Name() == "file-writer"
	})
}
