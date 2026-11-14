package queue

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"time"

	"github.com/LeisureLinux/happy-downloader/v4/server/config"
	"github.com/LeisureLinux/happy-downloader/v4/server/internal"
	"github.com/LeisureLinux/happy-downloader/v4/server/internal/downloaders"
	"github.com/LeisureLinux/happy-downloader/v4/server/internal/downloadlog"
	"github.com/LeisureLinux/happy-downloader/v4/server/internal/metadata"
)

type MessageQueue struct {
	concurrency   int
	downloadQueue chan downloaders.Downloader
	metadataQueue chan downloaders.Downloader
	ctx           context.Context
	cancel        context.CancelFunc
}

func NewMessageQueue() (*MessageQueue, error) {
	qs := config.Instance().Server.QueueSize
	if qs <= 0 {
		return nil, errors.New("invalid queue size")
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &MessageQueue{
		concurrency:   qs,
		downloadQueue: make(chan downloaders.Downloader, qs*2),
		metadataQueue: make(chan downloaders.Downloader, qs*4),
		ctx:           ctx,
		cancel:        cancel,
	}, nil
}

// Publish download job
func (m *MessageQueue) Publish(d downloaders.Downloader) {
	d.SetPending(true)

	select {
	case m.downloadQueue <- d:
		slog.Info("published download", slog.String("id", d.GetId()))
	case <-m.ctx.Done():
		slog.Warn("queue stopped, dropping download", slog.String("id", d.GetId()))
	}
}

// Workers: download + metadata
func (m *MessageQueue) SetupConsumers() {
	// N parallel workers for downloadQueue
	for i := 1; i < m.concurrency+1; i++ {
		go m.downloadWorker(i)
	}

	// 1 serial worker for metadata
	go m.metadataWorker()
}

// Worker dei download
func (m *MessageQueue) downloadWorker(workerId int) {
	slog.Info("download worker spawned", slog.Int("worker", workerId))

	for {
		select {
		case <-m.ctx.Done():
			return
		case p := <-m.downloadQueue:
			if p == nil {
				continue
			}
			if p.IsCompleted() {
				continue
			}

			slog.Info("download worker starting download",
				slog.Int("worker", workerId),
				slog.String("id", p.GetId()),
			)

			m.metadataQueue <- p
			slog.Info("queued for metadata", slog.String("id", p.GetId()))

			// A rejected argument used to vanish here: the download sat at
			// "preparing" forever with nothing in the log to explain it.
			//
			// A download that was stopped on purpose also ends with an error
			// — the process was killed — and that is not a failure. Reporting
			// it as one turned a paused download into a failed one, and the
			// button became "download again" instead of "continue".
			//
			// This is also the one place a download is over, for both kinds:
			// Start blocks until the file is written or the recording has
			// ended. So it is where the download log is written. A download
			// that was paused is not an outcome — it is waiting to be
			// resumed — and it is not recorded.
			started := time.Now()
			err := p.Start()

			if err != nil {
				if p.IsPaused() {
					slog.Info("download paused",
						slog.String("id", p.GetId()),
					)
					continue
				}

				// Start() already decided whether a non-zero exit is a real
				// failure: a postprocessor may complain after the file is
				// complete. Overwriting the status here undid that decision
				// and turned a finished video into a retryable failure.
				slog.Warn("yt-dlp exited with an error",
					slog.String("id", p.GetId()),
					slog.String("err", err.Error()),
				)
			}

			recordOutcome(p, err, started)
		}
	}
}

// recordOutcome writes the download log line for a download that has finished.
//
// The status is read from the download rather than from the error, because the
// two do not always agree: stopping a recording on purpose kills the process,
// so it exits with an error, and the download is nevertheless a success.
func recordOutcome(d downloaders.Downloader, startErr error, started time.Time) {
	snapshot := d.Status()
	status := snapshot.Progress.Status

	// Paused and still-running are not outcomes.
	if status == internal.StatusPaused || status == internal.StatusLiveStream {
		return
	}

	path := snapshot.Output.SavedFilePath

	// Status is Start()'s decision, not ours. It already checked whether the
	// finished file exists when yt-dlp exited non-zero. Guessing "success"
	// here from a file on disk would be wrong for a retry: the file left by
	// the *first* attempt is still there while the second one fails, and the
	// log would record a success nobody earned.
	_, size, _ := statFile(path)

	outcome := downloadlog.Outcome{
		FromIP:     snapshot.FromIP,
		URL:        snapshot.URL,
		Path:       path,
		Elapsed:    time.Since(started),
		Downloader: snapshot.DownloaderName,
		Size:       size,
	}

	if status == internal.StatusCompleted {
		outcome.Status = downloadlog.StatusSuccess
	} else {
		outcome.Status = downloadlog.StatusFailed

		// What yt-dlp said beats the exit code it left behind: "exit
		// status 1" is what the process reports, and it explains nothing.
		switch reason := d.GetFailureReason(); {
		case reason != "":
			outcome.Reason = reason
		case startErr != nil:
			outcome.Reason = startErr.Error()
		default:
			outcome.Reason = "download failed"
		}
	}

	downloadlog.Record(outcome)
}

// statFile is the file check shared by the outcome record. A file must be
// non-empty: a zero-byte artefact is not a video.
func statFile(path string) (os.FileInfo, int64, bool) {
	if path == "" {
		return nil, 0, false
	}

	info, err := os.Stat(path)
	if err != nil || info.IsDir() || info.Size() == 0 {
		return nil, 0, false
	}

	return info, info.Size(), true
}

func (m *MessageQueue) metadataWorker() {
	slog.Info("metadata worker spawned", slog.Int("worker", 1))

	for {
		select {
		case <-m.ctx.Done():
			return
		case p := <-m.metadataQueue:
			if p == nil {
				continue
			}

			slog.Info("metadata worker started",
				slog.String("id", p.GetId()),
			)

			if p.IsCompleted() {
				slog.Warn("metadata skipped, illegal state",
					slog.String("id", p.GetId()),
				)
				continue
			}

			slog.Info("metadata worker started", slog.String("id", p.GetId()))
			p.SetMetadata(metadata.DefaultFetcher)
		}
	}
}

func (m *MessageQueue) Stop() {
	m.cancel()
	close(m.downloadQueue)
	close(m.metadataQueue)
}
