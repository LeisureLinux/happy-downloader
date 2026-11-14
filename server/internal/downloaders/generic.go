package downloaders

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/LeisureLinux/happy-downloader/v4/server/common"
	"github.com/LeisureLinux/happy-downloader/v4/server/config"
	"github.com/LeisureLinux/happy-downloader/v4/server/internal"
	"github.com/LeisureLinux/happy-downloader/v4/server/internal/probeinfo"
	"github.com/LeisureLinux/happy-downloader/v4/server/internal/tools"
	"github.com/google/uuid"
)

const downloadTemplate = `download:
{
	"eta":%(progress.eta)s,
	"percentage":"%(progress._percent_str)s",
	"speed":%(progress.speed)s
}`

// The postprocess template reports where the file is *going*, not just where
// it is now. `filepath` is the intermediate in the temp directory; `__finaldir`
// is the directory MoveFiles will move it into. Keeping only `filepath` made
// every finished row point at a file that no longer existed.
//
// `status` is emitted at every postprocessor start/finish, so it also marks
// the stage after the last byte arrived: the download is no longer downloading.
const postprocessTemplate = `postprocess:
{
	"filepath":"%(info.filepath)s",
	"finaldir":"%(info.__finaldir)s",
	"status":"%(progress.status)s",
	"width":%(info.width)s,
	"height":%(info.height)s
}
`

type GenericDownloader struct {
	Params []string

	AutoRemove bool

	progress internal.DownloadProgress
	output   internal.DownloadOutput

	// duration in seconds, and the caption tracks the source offered.
	duration float64
	captions []internal.CaptionTrack

	proc *os.Process

	// stopped records that this download was stopped on purpose, so the exit
	// handler knows that an exit is not a completion.
	stopped bool

	logConsumer LogConsumer

	// embedded
	DownloaderBase
}

func NewGenericDownload(url string, params []string) Downloader {
	g := &GenericDownloader{
		logConsumer: NewJSONLogConsumer(),
	}
	// in base
	g.Id = uuid.NewString()
	g.URL = url
	g.Params = params
	g.Completed = false

	return g
}

func (g *GenericDownloader) Start() error {
	g.SetPending(true)

	// Older builds recorded the resolved -o in Params. The whitelist rejects
	// it, so those entries failed on every restart; the output path is
	// re-derived below anyway, so drop it before validating.
	g.Params = StripOutputArgs(g.Params)

	whiltelistedParams, err := argsSanitizer(g.Params)
	if err != nil {
		return err
	}

	g.Params = whiltelistedParams

	out := internal.DownloadOutput{
		Path:     config.Instance().Paths.DownloadPath,
		Filename: "%(title)s.%(ext)s",
	}

	if g.output.Path != "" {
		out.Path = g.output.Path
	}

	if g.output.Filename != "" {
		out.Filename = g.output.Filename
	}

	buildFilename(&g.output)

	templateReplacer := strings.NewReplacer("\n", "", "\t", "", " ", "")

	// XXX: the URL used to be truncated at "?list" to force --no-playlist. That
	// also dropped every legitimate query parameter whenever the video id was
	// not the first one, so the parameter is handled with --no-playlist alone.
	//
	// The runtime and cookie flags come from BuildArgs, so this stage resolves
	// under exactly the same conditions as metadata: when they disagreed, a URL
	// could pass metadata and then fail the download.
	baseParams := BuildArgs(g.URL,
		"--newline",
		"--no-colors",
		"--no-playlist",
		// Keep going when a secondary step fails. The video is the artifact the
		// user asked for; a throttled caption request (HTTP 429 from YouTube's
		// subtitle endpoint is routine) or a failed thumbnail embed should not
		// discard a download that already succeeded. A genuinely failed video
		// still leaves no output path, which the UI reports as a missing file
		// rather than a success.
		"--ignore-errors",
		// Subtitles are disabled on purpose. A host-level yt-dlp config often
		// turns on --write-subs/--write-auto-subs, and YouTube rate-limits the
		// caption endpoint hard (HTTP 429). Without these overrides a throttled
		// subtitle request aborts the whole job before a single byte of video
		// is fetched, which reads as "the download just does nothing". Captions
		// are not worth failing a video over.
		"--no-write-subs",
		"--no-write-auto-subs",
		// Throttle progress output. Several lines per fragment on a slow HLS
		// download filled the log with identical percentages and pushed the
		// real errors out of the in-memory history.
		"--progress-delta", "0.5",
		// A transfer that stalls on one fragment used to burn ten retries and
		// then fail a download whose bytes were almost all on disk. Give each
		// fragment a longer allowance and more attempts before giving up.
		"--socket-timeout", "30",
		"--retries", "20",
		"--fragment-retries", "20",
		"--progress-template",
		templateReplacer.Replace(downloadTemplate),
		"--progress-template",
		templateReplacer.Replace(postprocessTemplate),
		"--remote-components",
		"ejs:github",
	)

	// The output flag goes onto a copy. Appending it to g.Params made the field
	// record the resolved command instead of the request, so a second Start()
	// — or a retry rebuilt from Status() — carried a -o that the argument
	// whitelist rejects and the job could never be restarted.
	userParams := g.Params

	// if user asked to manually override the output path...
	if !(slices.Contains(g.Params, "-P") || slices.Contains(g.Params, "--paths")) {
		outputPath := filepath.Join(out.Path, out.Filename)

		home, temp, err := outputPaths()
		if err != nil {
			return err
		}

		// The template is relative to the home path, because "--paths temp:"
		// is ignored when the output template is absolute. The file still
		// ends up in the download directory, and the partial file is kept
		// out of it until the download is finished.
		rel, err := relativeOutputName(outputPath, out.Filename)
		if err != nil {
			return err
		}

		userParams = append(userParams, pathArgs(home, temp, rel)...)
	}

	params := append(baseParams, userParams...)
	params = append(params, "--no-exec")

	LogCommand("download", g.URL, params)

	started := time.Now()
	ctx, cancel := context.WithCancel(context.Background())

	cmd := exec.CommandContext(ctx, tools.YtDlp(), params...)
	prepareProcess(cmd)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		slog.Error("failed to get a stdout pipe", slog.String("err", err.Error()))
		panic(err)
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		slog.Error("failed to get a stderr pipe", slog.String("err", err.Error()))
		panic(err)
	}

	if err := cmd.Start(); err != nil {
		slog.Error("failed to start yt-dlp process", slog.String("err", err.Error()))
		panic(err)
	}

	g.proc = cmd.Process

	// The cleanup that runs when yt-dlp exits, whether it finished, was
	// killed or failed. It runs exactly once: the stderr reader used to call
	// it too, which closed stdout and marked the download complete while
	// yt-dlp was still writing the file.
	//
	failed := false
	var once sync.Once
	stop := func() {
		once.Do(func() {
			switch {
			case g.stopped:
				g.progress.Status = internal.StatusPaused
			case failed:
				g.progress.Status = internal.StatusErrored
			default:
				g.Complete()
				g.progress.Status = internal.StatusCompleted
			}
		})
	}
	defer stop()

	// stdout and stderr are drained to completion before the outcome is
	// decided. cmd.Wait() returns as soon as the process is gone, but the
	// pipes still hold the last postprocess lines — the ones that name the
	// final path. The consumer is joined below, not abandoned to the
	// deferred cleanup.
	logs := make(chan []byte, 2)
	var readers sync.WaitGroup
	readers.Add(2)

	go func() {
		defer readers.Done()
		defer close(logs)
		produceLogs(stdout, logs)
	}()
	go func() {
		defer readers.Done()
		if err := printYtDlpErrors(stderr, g.Id, g.URL, g); err != nil {
			slog.Warn("could not read the rest of yt-dlp's stderr",
				slog.String("id", g.Id), slog.String("err", err.Error()))
		}
	}()

	consumerDone := make(chan struct{})
	go func() {
		defer close(consumerDone)
		consumeLogs(ctx, logs, g.logConsumer, g)
	}()

	g.SetPending(false)

	err = cmd.Wait()
	readers.Wait()
	// The writer closed the channel after EOF; the consumer drains the rest
	// before leaving. Join it so the last postprocess line — the one naming
	// the final path — is applied before the outcome is decided.
	<-consumerDone
	cancel()

	// A download that failed is not a finished one — but a non-zero exit is
	// not proof of failure either. `--ignore-errors` applies to
	// postprocessing: yt-dlp reports the postprocessor's error and exits
	// non-zero even when the video is already on disk. Judging that a
	// failure discarded a finished file and sent people back to press
	// download a second time. Only the absence of a playable file is a
	// failure here.
	if err != nil && !g.stopped {
		// When a postprocessor dies before the last progress line, no
		// postprocess event ever names the finished file. Look for it by the
		// output template before concluding that nothing was produced.
		if !fileExists(g.output.SavedFilePath) {
			if found := findFinishedFile(g, started); found != "" {
				g.output.SavedFilePath = found
			}
		}

		failed = !fileExists(g.output.SavedFilePath)
		if !failed {
			slog.Warn("yt-dlp reported a postprocessing error but the file is complete",
				slog.String("id", g.Id),
				slog.String("url", g.URL),
				slog.String("path", g.output.SavedFilePath),
				slog.String("err", err.Error()))
		}
	}

	// Now the file is at its final path. Record the source once, for the
	// file browser's captions and the archive's dedupe index.
	if fileExists(g.output.SavedFilePath) {
		RememberSource(g, g.output.SavedFilePath)
	}

	return err
}

// Stop halts the download without ending it.
//
// It used to mark the download as completed, which was wrong twice over: the
// file was never finished, and the entry left the queue and appeared in the
// file browser as a file that did not exist. A stopped download is now
// paused — it stays in the queue, and resuming it picks up where it left off
// because yt-dlp keeps the partial file.
//
// The status is only set when the process was actually running. A download
// that was stopped before it started, or stopped twice, keeps whatever status
// it had.
func (g *GenericDownloader) Stop() error {
	// The status is set before the process is killed, not after. yt-dlp
	// prints a last progress line as it dies, and that line says
	// "downloading" — which turned the pause back into a running download,
	// so the row showed a moving bar for something that was stopped.
	if g.progress.Status != internal.StatusCompleted {
		g.progress.Status = internal.StatusPaused
	}

	defer func() {
		// Only a download that was actually running becomes paused. One that
		// was already stopped stays paused, and one that had already
		// finished stays finished.
		if g.progress.Status != internal.StatusCompleted {
			g.progress.Status = internal.StatusPaused
		}
	}()
	// yt-dlp uses multiple child process the parent process
	// has been spawned with setPgid = true. To properly kill
	// all subprocesses a SIGTERM need to be sent to the correct
	// process group
	g.stopped = true

	// A download that has not started yet has no process to kill. That is not
	// a failure to pause it: the status was already set to paused by the
	// deferred function above, and the entry stays in the queue. Returning an
	// error here made the button report a failure for a download that was
	// stopped successfully.
	if g.proc == nil {
		return nil
	}

	return killProcessGroup(&exec.Cmd{Process: g.proc})
}

func (g *GenericDownloader) Status() internal.ProcessSnapshot {
	// The container the arguments asked for, not the one the probe reported.
	// Recomputed from Params rather than cached so a restored snapshot, whose
	// Params came back with it, shows the same thing.
	info := g.Metadata
	if container := requestedContainer(g.Params); container != "" {
		info.Extension = container
	}

	return internal.ProcessSnapshot{
		Id:             g.Id,
		URL:            g.URL,
		Info:           info,
		Progress:       g.progress,
		Output:         g.output,
		Params:         g.Params,
		DownloaderName: "generic",
		Duration:       g.duration,
		FromIP:         g.FromIP,
		Captions:       g.captions,
	}
}

// UpdateSavedFilePath records the finished file.
//
// The postprocess event names the temp file; MoveFiles has not run yet when
// it is emitted. Translating here as well as in the log consumer means every
// writer — including a restored snapshot or a build whose template did not
// report `finaldir` — lands on the final path. The translation is idempotent:
// an absolute path outside the temp directory is returned unchanged.
func (g *GenericDownloader) UpdateSavedFilePath(p string) {
	if strings.TrimSpace(p) == "" {
		return
	}
	g.output.SavedFilePath = finalOutputPath(p, "")
}

// SetDuration records the video length, in seconds.
//
// It comes from the probe, which runs before anything is transferred, so the
// queue can show a length for work that has not started. Zero means unknown.
func (g *GenericDownloader) SetDuration(d float64) { g.duration = d }

// SetCaptions records the caption tracks the source offered.
func (g *GenericDownloader) SetCaptions(c []internal.CaptionTrack) { g.captions = c }

func (g *GenericDownloader) UpdateOutputDimensions(width, height float64) {
	g.output.Width = width
	g.output.Height = height
}

func (g *GenericDownloader) SetOutput(o internal.DownloadOutput)     { g.output = o }
func (g *GenericDownloader) SetProgress(p internal.DownloadProgress) { g.progress = p }

func (g *GenericDownloader) SetMetadata(fetcher func(url string) (*common.DownloadMetadata, error)) {
	g.FetchMetadata(fetcher)

	// The probe already knew the length and which caption languages the source
	// offers. Carry them over so the queue can show a duration and the player
	// can offer the right tracks, without a second extraction.
	if meta, duration, captions, ok := probeinfo.RecallFull(g.URL); ok {
		g.duration = duration
		g.captions = captions
		_ = meta
	}
}

func (g *GenericDownloader) SetPending(p bool) {
	g.Pending = p
}

func (g *GenericDownloader) GetId() string  { return g.Id }
func (g *GenericDownloader) GetUrl() string { return g.URL }

func (g *GenericDownloader) RestoreFromSnapshot(snap *internal.ProcessSnapshot) error {
	if snap == nil {
		return errors.New("cannot restore nil snapshot")
	}

	s := *snap

	g.Id = s.Id
	g.Metadata = s.Info
	g.progress = s.Progress
	g.output = s.Output
	g.Params = s.Params
	g.FromIP = s.FromIP

	// Prefer the requested URL; older snapshots only recorded the metadata one.
	g.URL = s.URL
	if g.URL == "" {
		g.URL = s.Info.URL
	}

	// Without this every restart re-ran every download that had ever finished,
	// because Completed defaulted to false and the store publishes anything
	// that is not completed.
	g.Completed = s.Progress.Status == internal.StatusCompleted

	return nil
}

func (g *GenericDownloader) IsCompleted() bool { return g.Completed }

// IsPaused reports whether this download was stopped on purpose.
func (g *GenericDownloader) IsPaused() bool { return g.stopped }
