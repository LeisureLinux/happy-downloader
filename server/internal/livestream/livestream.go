package livestream

import (
	"bufio"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/LeisureLinux/happy-downloader/v4/server/config"
	"github.com/LeisureLinux/happy-downloader/v4/server/internal/downloaders"
	"github.com/LeisureLinux/happy-downloader/v4/server/internal/kv"
	"github.com/LeisureLinux/happy-downloader/v4/server/internal/pipes"
	"github.com/LeisureLinux/happy-downloader/v4/server/internal/queue"
)

const (
	waiting = iota
	inProgress
	completed
	errored
)

// Defines a generic livestream.
// A livestream is identified by its url.
type LiveStream struct {
	url          string
	proc         *os.Process        // used to manually kill the yt-dlp process
	status       int                // whether is monitoring or completed
	done         chan *LiveStream   // where to signal the completition
	waitTimeChan chan time.Duration // time to livestream start
	waitTime     time.Duration
	liveDate     time.Time

	mq    *queue.MessageQueue
	store *kv.Store

	// recordMinutes is how long the recording should run.
	recordMinutes int

	// fromIP is the address the stream was asked for from. It is handed to
	// the download the monitor publishes, which is the one that ends up in
	// the download log.
	fromIP string

	// recorder is the download that is doing the recording, once the stream
	// has been handed to the queue.
	//
	// It is what the stop button has to reach. The process this struct
	// holds is the one that watches for the stream to start, and that one
	// is finished long before the recording is: the recording is a
	// separate yt-dlp process, running for as long as the stream lasts.
	// Killing the watcher therefore stopped nothing at all — the button
	// said the recording had stopped while it carried on.
	recorder downloaders.Downloader
	mu       sync.Mutex

	// published says whether the stream has been handed to the queue, which
	// is the point where it stopped being watched for and started being
	// downloaded.
	//
	// The two are told apart when the stream is finished or killed: a stream
	// that is downloading is still known — it can be stopped and its state
	// reported — and one that was only being watched for is forgotten,
	// because there is nothing left of it.
	published bool
}

// SetFromIP records the address the stream was asked for from.
func (l *LiveStream) SetFromIP(ip string) {
	l.mu.Lock()
	l.fromIP = ip
	l.mu.Unlock()
}

// SetRecordMinutes says how long the recording should run.
//
// A livestream does not end, so the length is what makes "download it" a
// thing that finishes. It is in minutes, because that is how long a
// recording is thought about.
func (l *LiveStream) SetRecordMinutes(minutes int) {
	l.mu.Lock()
	l.recordMinutes = minutes
	l.mu.Unlock()
}

func New(url string, done chan *LiveStream, mq *queue.MessageQueue, store *kv.Store) *LiveStream {
	return &LiveStream{
		url:          url,
		done:         done,
		status:       waiting,
		waitTime:     time.Second * 0,
		waitTimeChan: make(chan time.Duration),
		mq:           mq,
		store:        store,
	}
}

// Start the livestream monitoring process, once completion signals on the done channel
func (l *LiveStream) Start() error {
	// The same arguments as any other request: the runtime, the cookie and
	// the proxy.
	//
	// They were missing here, which meant the stage that decides whether
	// the stream is live at all could not reach Twitch — a stream that was
	// live was reported as not live, and nothing was recorded.
	params := downloaders.BuildArgs(l.url,
		"--wait-for-video", "30", // wait for the stream to be live and recheck every 30 secs
		"--no-colors", // no ansi color fuzz
		"--simulate",
		"--newline",
		"--paths", config.Instance().Paths.DownloadPath,
	)

	cmd := exec.Command(config.Instance().Paths.DownloaderPath, params...)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		l.status = errored
		l.finish()
		return err
	}
	defer stdout.Close()

	if err := cmd.Start(); err != nil {
		l.status = errored
		l.finish()
		return err
	}

	l.proc = cmd.Process
	l.status = waiting

	// Start monitoring when the livestream is goin to be live.
	// If already live do nothing.
	go l.monitorStartTime(stdout)

	// Wait to the simulated download process to finish.
	cmd.Wait()

	// Set the job as completed and notify the parent the completion.
	l.status = completed

	// Send the started livestream to the message queue! :D

	//TODO: add pipes
	d := downloaders.NewLiveStreamDownloader(l.url, []pipes.Pipe{}).(*downloaders.LiveStreamDownloader)

	l.mu.Lock()
	duration := l.recordMinutes
	l.mu.Unlock()

	d.SetRecordMinutes(duration)

	l.mu.Lock()
	d.SetFromIP(l.fromIP)
	l.mu.Unlock()

	l.store.Set(d)
	l.mq.Publish(d)

	l.mu.Lock()
	l.published = true
	// The recorder is kept so that the stop button can reach it. It is set
	// before the stream is announced as published, so that a stop arriving
	// right away finds the recorder rather than nothing.
	l.recorder = d
	l.mu.Unlock()

	// The completion is signalled last, and only once the recording has
	// been handed to the queue.
	//
	// It used to be signalled first, which was a race: the monitor reads
	// `published` when it takes the stream off the channel, and it always
	// read it as false — so it forgot a stream that had just become a
	// recording. The recording then ran with nothing able to stop it, and
	// the footer kept saying the stream was live long after it was over.
	l.finish()

	return nil
}

// finish tells the monitor that this stream is over.
//
// It is safe to call more than once, and it never blocks: the monitor reads
// the channel in a loop, but a stream that failed to start has to be
// signalled from a path that cannot wait for it.
func (l *LiveStream) finish() {
	select {
	case l.done <- l:
	default:
		// The monitor is busy with another stream. The signal would be
		// dropped, and the stream would stay in the list — which is what
		// this is meant to prevent.
		go func() { l.done <- l }()
	}
}

func (l *LiveStream) monitorStartTime(r io.Reader) {
	// yt-dlp shows the time in the stdout
	scanner := bufio.NewScanner(r)

	defer func() {
		l.status = inProgress
		close(l.waitTimeChan)
	}()

	// however the time to live is not shown in a new line (and atm there's nothing to do about)
	// use a custom split funciton to set the line separator to \r instead of \r\n or \n
	scanner.Split(stdoutSplitFunc)

	waitTimeScanner := func() {
		for scanner.Scan() {
			// l.log <- scanner.Bytes()

			// if this substring is in the current line the download is starting,
			// no need to monitor the time to live.
			//TODO: silly
			if !strings.Contains(scanner.Text(), "Remaining time until next attempt") {
				return
			}

			parts := strings.Split(scanner.Text(), ": ")
			if len(parts) < 2 {
				continue
			}

			startsIn := parts[1]
			parsed, err := parseTimeSpan(startsIn)
			if err != nil {
				continue
			}

			l.liveDate = parsed

			//TODO: check if using channels is stupid or not
			// l.waitTimeChan <- time.Until(start)
			l.waitTime = time.Until(parsed)
		}
	}

	scanner.Scan()

	for !strings.Contains(scanner.Text(), "Waiting for") {
		scanner.Scan()
	}

	waitTimeScanner()
}

func (l *LiveStream) WaitTime() <-chan time.Duration {
	return l.waitTimeChan
}

// Kills a livestream process and signal its completition
func (l *LiveStream) Kill() error {
	// The recording is what has to stop.
	//
	// The process this struct holds is the watcher — the one that waits for
	// the stream to start — and it is finished long before the recording
	// is. Killing it therefore stopped nothing: the button said the
	// recording had stopped while it carried on, and the file kept growing.
	l.mu.Lock()
	recorder := l.recorder
	l.mu.Unlock()

	if recorder != nil {
		if err := recorder.Stop(); err != nil {
			slog.Debug("could not stop the recording",
				slog.String("url", l.url),
				slog.Any("err", err),
			)
		}
	}

	// The watcher is killed too, for the case where the stream has not
	// started yet and there is no recording to stop.
	if l.proc != nil {
		l.proc.Kill()
	}

	// The signal is sent after the process is killed, so that whoever
	// receives it sees a stream that is really over. The channel is
	// unbuffered, so sending first would block until the monitor took it —
	// and the monitor would then look at a stream that was still running.
	l.finish()

	return nil
}

// Parse the timespan returned from yt-dlp (time to live)
//
//	parsed := parseTimeSpan("76:12:15")
//	fmt.Println(parsed) // 2024-07-21 13:59:59.634781 +0200 CEST
func parseTimeSpan(timeStr string) (time.Time, error) {
	parts := strings.Split(timeStr, ":")

	hh, err := strconv.Atoi(parts[0])
	if err != nil {
		return time.Time{}, err
	}
	mm, err := strconv.Atoi(parts[1])
	if err != nil {
		return time.Time{}, err
	}
	ss, err := strconv.Atoi(parts[2])
	if err != nil {
		return time.Time{}, err
	}

	dd := 0

	if hh > 24 {
		dd = hh / 24
		hh = hh % 24
	}

	start := time.Now()
	start = start.AddDate(0, 0, dd)
	start = start.Add(time.Duration(hh) * time.Hour)
	start = start.Add(time.Duration(mm) * time.Minute)
	start = start.Add(time.Duration(ss) * time.Second)

	return start, nil
}
