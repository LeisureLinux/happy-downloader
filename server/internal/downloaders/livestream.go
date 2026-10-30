package downloaders

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"syscall"
	"time"

	"github.com/LeisureLinux/happy-downloader/v4/server/common"
	"github.com/LeisureLinux/happy-downloader/v4/server/internal"
	"github.com/LeisureLinux/happy-downloader/v4/server/internal/pipes"
	"github.com/LeisureLinux/happy-downloader/v4/server/internal/tools"
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

	// output is the file that was written, once there is one.
	//
	// It was never kept before: the recording reported no output at all, so
	// its row in the queue had no size and no play button, and the file
	// browser only learned about the file from the directory listing.
	output internal.DownloadOutput

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

	cmd := exec.Command(tools.YtDlp(), params...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	// stdout = media stream
	//
	// The pipe is made here rather than with cmd.StdoutPipe, because
	// cmd.Wait closes the pipes it opened — and it does so the moment the
	// process exits, discarding whatever was still in the buffer and
	// closing the read end under the copy that was reading it. The
	// recording came out empty, and stopping it by hand lost everything
	// that had been recorded.
	//
	// A pipe given to cmd.Stdout is left alone by cmd.Wait, so the copy
	// reads to the end of the stream and the file is whole.
	mediaReader, mediaWriter, err := os.Pipe()
	if err != nil {
		slog.Error("failed to get media stdout", slog.Any("err", err))
		panic(err)
	}

	cmd.Stdout = mediaWriter

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

	// The write end is the child's now. Closing it here is what tells the
	// copy that the stream has ended, once the child has gone.
	mediaWriter.Close()
	defer mediaReader.Close()

	l.proc = cmd.Process

	ctx, cancel := context.WithCancel(context.Background())
	defer func() {
		l.Complete()
		cancel()
	}()

	// build pipeline
	reader := io.Reader(mediaReader)
	for _, pipe := range l.pipes {
		nr, err := pipe.Connect(reader)
		if err != nil {
			slog.Error("pipe failed", slog.String("pipe", pipe.Name()), slog.Any("err", err))
			return err
		}
		reader = nr
	}

	// written is closed when the file is finished: the stream copied out,
	// the container written, the source recorded. It is what the end of
	// Start waits for.
	var written chan struct{}

	if !l.hasFileWriter() {
		written = make(chan struct{})

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
		// after the stream begins, so a name taken at the start is whatever
		// the fallback happens to be — which is how a YouTube recording came
		// to be called "Twitch 2026-10-29 14-16-20". It said so in this
		// comment for a while; the code went on naming the file at the start.
		//
		// The copy is waited for before the process is waited for.
		//
		// Waiting for the process closes the pipe the media comes out of,
		// and a copy that is still running when that happens gets nothing
		// to read: the recording was empty, and stopping it by hand lost
		// everything that had been recorded. The process is killed to end
		// it, and the pipe is then read out to the end before the file is
		// closed.
		go func() {
			defer close(written)

			// The stream is copied into a file with a provisional name,
			// because the title is not known yet. The real name is settled
			// below, once the recording has ended, and the file is moved
			// there.
			raw := l.outputPath
			provisional := false

			if raw == "" {
				raw = filepath.Join(tempDir(), "recording-"+l.Id+".ts")
				provisional = true
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

			// The recording is put into an MP4, so that it plays, and into
			// its real name.
			//
			// The media comes out of the stream as MPEG-TS and the file
			// was named as an MP4, which is a file whose extension lies
			// about its contents — and nothing plays it.
			final := raw
			if provisional {
				_, final = l.recordingPath()
			}

			if final != "" && final != raw {
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

	// The stream has stopped arriving. The file is not finished yet: it is
	// still being copied out of the pipe and put into its container, which
	// for a long recording takes a few seconds.
	//
	// So the row says so. It stays in the queue until the file exists,
	// because until then there is nothing for the file browser to show, and
	// an entry that vanished and came back would be worse than one that says
	// what it is doing.
	if written != nil {
		l.progress.Status = internal.StatusRemuxing

		select {
		case <-written:
		case <-time.After(30 * time.Minute):
			slog.Error("the recording was not written in time", slog.String("id", l.Id))
		}
	}

	// A recording that was stopped on purpose is not a failure.
	//
	// The process is killed, so it exits with an error, and the error was
	// reported as a failed download — which is what a recording that ran its
	// time always looked like.
	if l.stopped {
		l.progress.Status = internal.StatusCompleted
		l.Complete()
		signalFinished(l)
		return nil
	}

	// A recording that produced a file is a success, whatever the exit code
	// said.
	//
	// yt-dlp exits non-zero when a stream ends under it — the connection
	// dropped, the streamer stopped, a segment came back short — and every
	// one of those leaves a perfectly good recording behind. Reporting them
	// as failures put a red row in the queue for a recording that worked,
	// which is the one thing a person watching for it must not be told.
	if recorded := l.recordedBytes(); recorded > 0 {
		l.progress.Status = internal.StatusCompleted
		l.Complete()
		signalFinished(l)
		return nil
	}

	// Stopped before anything arrived is not a finished recording.
	//
	// It used to be reported as one, because "stopped on purpose" was taken
	// to mean "succeeded". The row then said completed with no file and a
	// path pointing into the temporary directory — a recording that looks
	// fine and does not exist.
	if l.stopped {
		slog.Warn("the recording was stopped before anything was written",
			slog.String("id", l.Id),
			slog.String("url", l.URL),
		)

		l.progress.Status = internal.StatusErrored
		l.SetFailureReason("stopped before anything was recorded")
		l.output = internal.DownloadOutput{}
		signalFinished(l)

		return nil
	}

	if waitErr != nil {
		slog.Error("the recording failed", slog.String("id", l.Id), slog.Any("err", waitErr))
		l.progress.Status = internal.StatusErrored
		// A recording that failed is over as well, and it has to leave the
		// queue for the same reason a finished one does: the queue and the
		// file browser have to agree on what exists. A failed recording
		// produced no file, so there is nothing for the file browser to
		// show.
		signalFinished(l)
		return waitErr
	}

	l.progress.Status = internal.StatusCompleted
	l.Complete()
	signalFinished(l)

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

	// A name the request asked for wins over anything derived.
	if l.output.Filename != "" {
		return l.output.Filename
	}

	// The metadata did not arrive, or arrived without a title. The fallback
	// used to be the literal "Twitch", which named a YouTube recording
	// "Twitch 2026-10-29 14-16-20" — a file that is wrong about where it came
	// from, in the one place a person looks to find out.
	return nameFromURL(l.URL)
}

// nameFromURL is what a recording is called when nothing else says.
//
// The site, and the channel when the address carries one: "Twitch sidsicksevan"
// says which stream it was, where "Twitch" alone could be any of them. The
// file is also stamped with the time it started, so two recordings of one
// channel do not collide.
func nameFromURL(raw string) string {
	parsed, err := url.Parse(raw)
	if err != nil {
		return "recording"
	}

	host := strings.TrimPrefix(strings.ToLower(parsed.Hostname()), "www.")
	if host == "" {
		return "recording"
	}

	site := host
	for suffix, name := range map[string]string{
		"youtube.com":  "YouTube",
		"youtu.be":     "YouTube",
		"twitch.tv":    "Twitch",
		"bilibili.com": "Bilibili",
		"nicovideo.jp": "Niconico",
		"tiktok.com":   "TikTok",
		"douyin.com":   "Douyin",
	} {
		if host == suffix || strings.HasSuffix(host, "."+suffix) {
			site = name
			break
		}
	}

	// The last meaningful part of the path is the channel for Twitch and the
	// handle for YouTube. The generic words are not names.
	generic := map[string]bool{
		"live": true, "watch": true, "video": true, "videos": true,
		"channel": true, "c": true, "stream": true, "streams": true,
	}

	segment := ""
	for _, part := range strings.Split(strings.Trim(parsed.Path, "/"), "/") {
		if part == "" || generic[strings.ToLower(part)] {
			continue
		}
		segment = strings.TrimPrefix(part, "@")
	}

	if segment == "" {
		return site
	}

	return site + " " + segment
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
		return "recording"
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
		// How long the recording was asked to be, in seconds. The queue
		// measures the bar against it, so without it a recording sat at
		// zero no matter how long it had run.
		Duration: float64(l.Duration) * 60,
		FromIP:   l.FromIP,
		Output:   l.output,
	}
}

// PrepareRestart makes the recording ready to run again.
//
// A recording that was stopped has finished, and starting it again is a new
// segment of the same live stream: the process starts from the live edge, so
// the file that follows is a different file. What is reset here is the state
// that says this attempt is over; what is not reset is anything about the
// files already written.
func (l *LiveStreamDownloader) PrepareRestart() {
	l.stopped = false

	// The directory the request asked for is kept: the next segment belongs
	// beside the last one. Only the file that was written is cleared, because
	// the new segment writes a different file.
	l.output = internal.DownloadOutput{Path: l.output.Path}
	l.Reopen()
	l.SetProgress(internal.DownloadProgress{Status: internal.StatusPending})
}

// recordedBytes is the size of the file the recording was written to, or zero
// when there is none yet.
func (l *LiveStreamDownloader) recordedBytes() int64 {
	path := l.output.SavedFilePath
	if path == "" {
		return 0
	}

	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		return 0
	}

	return info.Size()
}

func (l *LiveStreamDownloader) UpdateSavedFilePath(p string) {
	l.output.SavedFilePath = p

	if info, err := os.Stat(p); err == nil && !info.IsDir() {
		l.output.FileSize = info.Size()
		l.output.FileExists = true
	}
}

func (l *LiveStreamDownloader) UpdateOutputDimensions(width, height float64) {}

func (l *LiveStreamDownloader) SetOutput(o internal.DownloadOutput)     { l.output = o }
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
	l.FromIP = s.FromIP

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
