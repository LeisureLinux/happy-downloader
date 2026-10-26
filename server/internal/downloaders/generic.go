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
	"syscall"

	"github.com/LeisureLinux/happy-downloader/v4/server/common"
	"github.com/LeisureLinux/happy-downloader/v4/server/config"
	"github.com/LeisureLinux/happy-downloader/v4/server/internal"
	"github.com/google/uuid"
)

const downloadTemplate = `download:
{
	"eta":%(progress.eta)s,
	"percentage":"%(progress._percent_str)s",
	"speed":%(progress.speed)s
}`

// filename not returning the correct extension after postprocess
const postprocessTemplate = `postprocess:
{
	"filepath":"%(info.filepath)s",
	"width":%(info.width)s,
	"height":%(info.height)s
}
`

type GenericDownloader struct {
	Params []string

	AutoRemove bool

	progress internal.DownloadProgress
	output   internal.DownloadOutput

	proc *os.Process

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
		"--progress-template",
		templateReplacer.Replace(downloadTemplate),
		"--progress-template",
		templateReplacer.Replace(postprocessTemplate),
		"--remote-components",
		"ejs:github",
	)

	// if user asked to manually override the output path...
	if !(slices.Contains(g.Params, "-P") || slices.Contains(g.Params, "--paths")) {
		outputPath := filepath.Join(out.Path, out.Filename)

		rel, err := filepath.Rel(config.Instance().Paths.DownloadPath, outputPath)
		if err != nil {
			return err
		}
		if strings.HasPrefix(rel, "..") {
			return errors.New(ErrIsNotSubPath)
		}

		g.Params = append(g.Params, "-o")
		g.Params = append(g.Params, outputPath)
	}

	params := append(baseParams, g.Params...)
	params = append(params, "--no-exec")

	LogCommand("download", g.URL, params)

	ctx, cancel := context.WithCancel(context.Background())

	cmd := exec.CommandContext(ctx, config.Instance().Paths.DownloaderPath, params...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

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

	stop := func() {
		stdout.Close()
		g.Complete()
		g.progress.Status = internal.StatusCompleted
		cancel()
	}
	defer stop()

	logs := make(chan []byte, 2)
	go produceLogs(stdout, logs)
	go consumeLogs(ctx, logs, g.logConsumer, g)

	go func() {
		err := printYtDlpErrors(stderr, g.Id, g.URL)
		if err != nil {
			stop()
		}
	}()

	g.SetPending(false)
	return cmd.Wait()
}

func (g *GenericDownloader) Stop() error {
	defer func() {
		g.progress.Status = internal.StatusCompleted
		g.Complete()
	}()
	// yt-dlp uses multiple child process the parent process
	// has been spawned with setPgid = true. To properly kill
	// all subprocesses a SIGTERM need to be sent to the correct
	// process group
	if g.proc == nil {
		return errors.New("*os.Process not set")
	}

	pgid, err := syscall.Getpgid(g.proc.Pid)
	if err != nil {
		return err
	}
	if err := syscall.Kill(-pgid, syscall.SIGTERM); err != nil {
		return err
	}

	return nil
}

func (g *GenericDownloader) Status() internal.ProcessSnapshot {
	return internal.ProcessSnapshot{
		Id:             g.Id,
		Info:           g.Metadata,
		Progress:       g.progress,
		Output:         g.output,
		Params:         g.Params,
		DownloaderName: "generic",
	}
}

func (g *GenericDownloader) UpdateSavedFilePath(p string) { g.output.SavedFilePath = p }

func (g *GenericDownloader) UpdateOutputDimensions(width, height float64) {
	g.output.Width = width
	g.output.Height = height
}

func (g *GenericDownloader) SetOutput(o internal.DownloadOutput)     { g.output = o }
func (g *GenericDownloader) SetProgress(p internal.DownloadProgress) { g.progress = p }

func (g *GenericDownloader) SetMetadata(fetcher func(url string) (*common.DownloadMetadata, error)) {
	g.FetchMetadata(fetcher)
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
	g.URL = s.Info.URL
	g.Metadata = s.Info
	g.progress = s.Progress
	g.output = s.Output
	g.Params = s.Params

	return nil
}

func (g *GenericDownloader) IsCompleted() bool { return g.Completed }
