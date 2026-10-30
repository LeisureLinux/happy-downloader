package server

import (
	"context"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/LeisureLinux/happy-downloader/v4/server/config"
	"github.com/LeisureLinux/happy-downloader/v4/server/filebrowser"
	"github.com/LeisureLinux/happy-downloader/v4/server/internal/cookies"
	"github.com/LeisureLinux/happy-downloader/v4/server/internal/cookiewatch"
	"github.com/LeisureLinux/happy-downloader/v4/server/internal/downloaders"
	"github.com/LeisureLinux/happy-downloader/v4/server/internal/kv"
	"github.com/LeisureLinux/happy-downloader/v4/server/internal/livestream"
	"github.com/LeisureLinux/happy-downloader/v4/server/internal/profiles"
	"github.com/LeisureLinux/happy-downloader/v4/server/internal/queue"
	"github.com/LeisureLinux/happy-downloader/v4/server/logging"
	"github.com/LeisureLinux/happy-downloader/v4/server/rest"
	"github.com/LeisureLinux/happy-downloader/v4/server/subscription/task"
	"github.com/LeisureLinux/happy-downloader/v4/server/twitch"

	bolt "go.etcd.io/bbolt"
)

// Core is the application without an interface.
//
// Everything the web interface does happens here: the queue, the progress,
// the recordings, the cookie handling, the site profiles. The web interface
// is one way of driving it — over HTTP — and the desktop application would be
// another, in the same process, with no port and no network.
//
// It exists because the alternative is a second implementation of the queue
// and the progress reporting, and those are the parts that are easy to get
// subtly wrong: a download that is reported as finished before its file is
// written, a recording that is forgotten while it is still running.
//
// A Core owns a database and a queue. It is not safe for concurrent use by
// several programs, and it does not listen on anything: a caller that wants
// to reach it over the network has to put an HTTP server in front of it,
// which is what Run does.
type Core struct {
	// DB is the state store. The download records, the source index and the
	// site profiles all live in it.
	DB *bolt.DB

	// MDB is the in-memory view of what is downloading.
	MDB *kv.Store

	// MQ is the queue the downloads are published to.
	MQ *queue.MessageQueue

	// LM watches for livestreams and records them.
	LM *livestream.Monitor

	// Profiles are the per-site argument sets.
	Profiles *profiles.Store

	// SourceIndex remembers where a downloaded file came from.
	SourceIndex *kv.SourceIndex

	// TaskRunner runs the scheduled downloads.
	TaskRunner task.TaskRunner

	// TwitchMonitor watches the channels that are followed.
	TwitchMonitor *twitch.Monitor

	// ImportedCookies is the export the person using the interface
	// uploaded.
	ImportedCookies *cookies.Store

	// observableLogger carries the log to whoever is watching it. The web
	// interface reads it over SSE; a desktop application would read it from
	// memory.
	observableLogger *logging.ObservableLogger
}

// CoreOptions is what the caller wants from a Core.
type CoreOptions struct {
	// LogWriters are where the log goes besides the standard logger. The
	// observable logger is added to them, so a caller that wants to watch the
	// log gets it without having to know about the logger itself.
	LogWriters []io.Writer

	// FileLogging writes the log to a file as well, and rotates it. It is
	// read from the configuration.
	FileLogging bool
}

// NewCore wires the application together.
//
// The order matters in a few places and the comments say where: the source
// recorder has to be installed before any download starts, or the files of
// the first download are recorded as having come from nowhere.
func NewCore(ctx context.Context, opts CoreOptions) (*Core, error) {
	dbPath := filepath.Join(config.Instance().Paths.LocalDatabasePath, "bolt.db")

	boltdb, err := bolt.Open(dbPath, 0600, nil)
	if err != nil {
		return nil, err
	}

	mdb, err := kv.NewStore(boltdb, time.Second*15)
	if err != nil {
		boltdb.Close()
		return nil, err
	}

	// The index of where downloaded files came from. It outlives the download
	// records, which are transient: a finished download leaves the queue and
	// its record goes with it, but the file stays and keeps needing to be
	// connected to its source.
	sourceIndex, err := kv.NewSourceIndex(boltdb)
	if err != nil {
		boltdb.Close()
		return nil, err
	}

	conf := config.Instance()

	// The download directory is made here, because every download assumes it
	// is there. It may not be: the setting may have just been changed to a
	// folder that does not exist yet, and the default is a folder of our own
	// inside "Downloads" rather than one the system made.
	//
	// Failing here is not fatal. A directory that cannot be created is a
	// real problem, but it is one the person can fix in the settings, and
	// the interface is how they found out.
	if err := os.MkdirAll(conf.Paths.DownloadPath, 0o755); err != nil {
		slog.Error("cannot create the download directory",
			slog.String("path", conf.Paths.DownloadPath),
			slog.String("err", err.Error()))
	}

	observableLogger := logging.NewObservableLogger()

	logWriters := append([]io.Writer{os.Stdout}, opts.LogWriters...)
	logWriters = append(logWriters, observableLogger)

	if opts.FileLogging && conf.Logging.EnableFileLogging {
		logger, err := logging.NewRotableLogger(conf.Logging.LogPath)
		if err != nil {
			boltdb.Close()
			return nil, err
		}

		go func() {
			for {
				time.Sleep(time.Hour * 24)
				logger.Rotate()
			}
		}()

		logWriters = append(logWriters, logger)
	}

	// Fanout, not io.MultiWriter. MultiWriter stops at the first destination
	// that returns an error, so a machine with no console — the Windows build
	// is one — logged to the terminal it did not have and nothing else, and
	// the log panel in the interface stayed empty.
	slog.SetDefault(slog.New(slog.NewTextHandler(
		logging.FanoutWriter(logWriters...),
		&slog.HandlerOptions{Level: slog.LevelInfo},
	)))

	mq, err := queue.NewMessageQueue()
	if err != nil {
		boltdb.Close()
		return nil, err
	}
	mq.SetupConsumers()

	// The source index is filled from the postprocessing events, so the
	// recorder has to be installed before any download starts.
	recorder := &kv.SourceRecorder{Index: sourceIndex}
	downloaders.SetSourceRecorder(recorder.Record)
	filebrowser.SetSourceForgetter(sourceIndex.Forget)

	// A recording left in the partial directory is one that was interrupted:
	// the process stopped between writing it and putting it into its
	// container. It is finished here rather than swept up with the rest of
	// the temporary files — nothing is using the directory yet, so anything
	// in it is somebody's recording.
	go downloaders.RecoverPartialRecordings()

	filebrowser.StartProbeCacheWriter()
	filebrowser.SetSourceLookup(func(path string) (string, string, bool) {
		rec, ok := sourceIndex.Lookup(path)
		if !ok {
			// The file was moved before the index learned to move, so its
			// record names a path that is no longer there. The name is the
			// same, so it is looked for by name.
			rec, ok = sourceIndex.LookupByName(path)
		}
		if !ok {
			return "", "", false
		}
		return rec.URL, rec.Site, true
	})
	filebrowser.SetSourceMover(sourceIndex.Move)

	go mdb.Restore(mq)
	go mdb.EventListener()

	// The cookie export the person using the interface uploaded.
	//
	// It takes precedence over the automatic export below: it is the
	// credential the person chose. When it has been refused twice it is
	// removed, and the automatic export takes over again.
	importedCookies := cookies.NewStore(rest.ImportedCookieDir(conf.Paths.LocalDatabasePath))
	importedCookies.Restore()
	cookies.SetGlobal(importedCookies)

	cookies.SetFailureHook(func() {
		if importedCookies.Failed() == nil {
			slog.Info("the imported cookie export was refused twice and has been removed")
		}
	})

	// Convert paths.cookie_path into the Netscape file yt-dlp reads, and keep
	// it in sync as the export is replaced. On a headless host this replaces
	// the browser re-export that is not available there.
	cookieSource := config.ResolvedCookieSource()
	cookieWatcher := cookiewatch.New(cookieSource, config.CookiesPath(cookieSource))
	if cookieWatcher.Enabled() {
		if err := cookieWatcher.Start(); err != nil {
			slog.Warn("cookie watch failed to start", slog.String("err", err.Error()))
		} else {
			defer cookieWatcher.Stop()

			downloaders.SetCookieSyncHook(func() {
				if err := cookieWatcher.Sync(); err != nil {
					slog.Debug("cookie sync skipped", slog.String("err", err.Error()))
				}
			})
			cookiewatch.SetGlobal(cookieWatcher)
		}
	}

	profileStore, err := profiles.NewStore(boltdb, profiles.Profile{})
	if err != nil {
		boltdb.Close()
		return nil, err
	}

	lm := livestream.NewMonitor(mq, mdb, boltdb)
	go lm.Schedule()
	go lm.Restore()

	// A finished recording is done as far as the monitor is concerned: it
	// cannot be stopped any more, and the stream it came from is over.
	downloaders.SetFinishedRecorder(func(d downloaders.Downloader) {
		lm.Forget(d.GetUrl())
	})

	tm := twitch.NewMonitor(
		twitch.NewAuthenticationManager(
			conf.Twitch.ClientId,
			conf.Twitch.ClientSecret,
		),
		boltdb,
	)
	go tm.Monitor(
		ctx,
		conf.Twitch.CheckInterval,
		twitch.DEFAULT_DOWNLOAD_HANDLER(mdb, mq),
	)
	go tm.Restore()

	cronTaskRunner := task.NewCronTaskRunner(mq, mdb)
	go cronTaskRunner.Spawner(ctx)

	return &Core{
		DB:               boltdb,
		MDB:              mdb,
		MQ:               mq,
		LM:               lm,
		Profiles:         profileStore,
		SourceIndex:      sourceIndex,
		TaskRunner:       cronTaskRunner,
		TwitchMonitor:    tm,
		ImportedCookies:  importedCookies,
		observableLogger: observableLogger,
	}, nil
}

// Close releases what the Core holds.
//
// The database is closed and the cookie watcher is stopped; the rest is
// goroutines that end with the process.
func (c *Core) Close() error {
	if c.DB != nil {
		return c.DB.Close()
	}
	return nil
}

// ObservableLogger is the log the caller can watch.
//
// The web interface streams it over SSE. A desktop application would read the
// same lines from it and put them in a window.
func (c *Core) ObservableLogger() *logging.ObservableLogger {
	return c.observableLogger
}
