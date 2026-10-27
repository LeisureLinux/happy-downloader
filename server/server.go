// a stupid package name...
package server

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"net"
	"net/http"
	"net/rpc"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/LeisureLinux/happy-downloader/v4/server/config"
	"github.com/LeisureLinux/happy-downloader/v4/server/filebrowser"
	"github.com/LeisureLinux/happy-downloader/v4/server/internal/cookiewatch"
	"github.com/LeisureLinux/happy-downloader/v4/server/internal/downloaders"
	"github.com/LeisureLinux/happy-downloader/v4/server/internal/kv"
	"github.com/LeisureLinux/happy-downloader/v4/server/internal/livestream"
	"github.com/LeisureLinux/happy-downloader/v4/server/internal/pipeline"
	"github.com/LeisureLinux/happy-downloader/v4/server/internal/profiles"
	"github.com/LeisureLinux/happy-downloader/v4/server/internal/queue"
	"github.com/LeisureLinux/happy-downloader/v4/server/logging"
	middlewares "github.com/LeisureLinux/happy-downloader/v4/server/middleware"
	"github.com/LeisureLinux/happy-downloader/v4/server/rest"
	ytdlpRPC "github.com/LeisureLinux/happy-downloader/v4/server/rpc"
	"github.com/LeisureLinux/happy-downloader/v4/server/status"
	"github.com/LeisureLinux/happy-downloader/v4/server/subscription"
	"github.com/LeisureLinux/happy-downloader/v4/server/subscription/task"
	"github.com/LeisureLinux/happy-downloader/v4/server/twitch"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/cors"

	bolt "go.etcd.io/bbolt"
)

type RunConfig struct {
	App fs.FS
}

type serverConfig struct {
	frontend      fs.FS
	mdb           *kv.Store
	db            *bolt.DB
	mq            *queue.MessageQueue
	lm            *livestream.Monitor
	taskRunner    task.TaskRunner
	twitchMonitor *twitch.Monitor
	profileStore  *profiles.Store
	sourceIndex   *kv.SourceIndex
}

// TODO: change scope
var observableLogger = logging.NewObservableLogger()

func Run(ctx context.Context, rc *RunConfig) error {
	dbPath := filepath.Join(config.Instance().Paths.LocalDatabasePath, "bolt.db")

	boltdb, err := bolt.Open(dbPath, 0600, nil)
	if err != nil {
		return err
	}

	mdb, err := kv.NewStore(boltdb, time.Second*15)
	if err != nil {
		slog.Error("could not open the download store", slog.Any("err", err))
		os.Exit(1)
	}

	// The index of where downloaded files came from. It outlives the download
	// records, which are transient: a finished download leaves the queue and
	// its record goes with it, but the file stays and keeps needing to be
	// connected to its source.
	sourceIndex, err := kv.NewSourceIndex(boltdb)
	if err != nil {
		slog.Error("could not open the source index", slog.Any("err", err))
		os.Exit(1)
	}
	if err != nil {
		return err
	}

	logWriters := []io.Writer{
		os.Stdout,
		observableLogger, // for web-ui
	}

	conf := config.Instance()

	// file based logging
	if conf.Logging.EnableFileLogging {
		logger, err := logging.NewRotableLogger(conf.Logging.LogPath)
		if err != nil {
			return err
		}

		defer logger.Rotate()

		go func() {
			for {
				time.Sleep(time.Hour * 24)
				logger.Rotate()
			}
		}()

		logWriters = append(logWriters, logger)
	}

	logger := slog.New(slog.NewTextHandler(io.MultiWriter(logWriters...), &slog.HandlerOptions{
		Level: slog.LevelInfo, // TODO: detect when launched in debug mode -> slog.LevelDebug
	}))

	// make the new logger the default one with all the new writers
	slog.SetDefault(logger)

	mq, err := queue.NewMessageQueue()
	if err != nil {
		return err
	}
	mq.SetupConsumers()
	// The source index is filled from the postprocessing events, so the
	// recorder has to be installed before any download starts.
	recorder := &kv.SourceRecorder{Index: sourceIndex}
	downloaders.SetSourceRecorder(recorder.Record)
	filebrowser.SetSourceForgetter(sourceIndex.Forget)

	go mdb.Restore(mq)
	go mdb.EventListener()

	// Convert paths.cookie_path into the Netscape file yt-dlp reads, and keep
	// it in sync as the export is replaced. On a headless host this replaces
	// the browser re-export that is not available there.
	cookieSource := config.ResolvedCookieSource()
	cookieWatcher := cookiewatch.New(
		cookieSource,
		config.CookiesPath(cookieSource),
	)
	if cookieWatcher.Enabled() {
		if err := cookieWatcher.Start(); err != nil {
			slog.Warn("cookie watch failed to start", slog.String("err", err.Error()))
		}
		defer cookieWatcher.Stop()

		// Pre-download safety net: re-check the source mtime before each
		// argv build, which also covers hosts where inotify delivers nothing.
		downloaders.SetCookieSyncHook(func() {
			if err := cookieWatcher.Sync(); err != nil {
				slog.Debug("cookie sync skipped", slog.String("err", err.Error()))
			}
		})
		cookiewatch.SetGlobal(cookieWatcher)
	}

	// Site profiles decide which arguments a URL needs: cookie, POT
	// provider, proxy and extras. Stored in bolt and editable from the UI,
	// because adding a site must not mean rebuilding the binary.
	profileStore, err := profiles.NewStore(boltdb, profiles.Profile{
		CookiePath: config.ResolvedCookieSource(),
		PotBaseURL: config.Instance().YtDlp.PotBaseURL,
		Proxy:      config.Instance().YtDlp.Proxy,
	})
	if err != nil {
		return err
	}

	lm := livestream.NewMonitor(mq, mdb, boltdb)
	go lm.Schedule()
	go lm.Restore()

	tm := twitch.NewMonitor(
		twitch.NewAuthenticationManager(
			config.Instance().Twitch.ClientId,
			config.Instance().Twitch.ClientSecret,
		),
		boltdb,
	)
	go tm.Monitor(
		ctx,
		config.Instance().Twitch.CheckInterval,
		twitch.DEFAULT_DOWNLOAD_HANDLER(mdb, mq),
	)
	go tm.Restore()

	cronTaskRunner := task.NewCronTaskRunner(mq, mdb)
	go cronTaskRunner.Spawner(ctx)

	scfg := serverConfig{
		frontend:      rc.App,
		mdb:           mdb,
		sourceIndex:   sourceIndex,
		db:            boltdb,
		mq:            mq,
		lm:            lm,
		twitchMonitor: tm,
		taskRunner:    cronTaskRunner,
		profileStore:  profileStore,
	}

	srv := newServer(scfg)

	go gracefulShutdown(ctx, srv, &scfg)

	var (
		network = "tcp"
		address = fmt.Sprintf("%s:%d", conf.Server.Host, conf.Server.Port)
	)

	// support unix sockets
	if strings.HasPrefix(conf.Server.Host, "/") {
		network = "unix"
		address = conf.Server.Host
	}

	listener, err := net.Listen(network, address)
	if err != nil {
		slog.Error("failed to listen", slog.String("err", err.Error()))
		return err
	}

	slog.Info("happy-downloader started", slog.String("address", address))

	if err := srv.Serve(listener); err != nil {
		slog.Warn("http server stopped", slog.String("err", err.Error()))
	}

	return nil
}

func newServer(c serverConfig) *http.Server {
	// archiver.Register(c.db)
	service := ytdlpRPC.Container(c.mdb, c.mq, c.lm)
	rpc.Register(service)

	r := chi.NewRouter()

	corsMiddleware := cors.New(cors.Options{
		AllowedOrigins: []string{"*"},
		AllowedMethods: []string{
			http.MethodHead,
			http.MethodGet,
			http.MethodPost,
			http.MethodPut,
			http.MethodPatch,
			http.MethodDelete,
		},
		AllowedHeaders:   []string{"*"},
		AllowCredentials: true,
	})

	r.Use(corsMiddleware.Handler)

	// One line per request, with the client address: the log is read from a
	// browser on another machine, and "download started" without knowing which
	// machine is half an answer.
	r.Use(middlewares.AccessLog)

	baseUrl := config.Instance().Server.BaseURL
	r.Mount(baseUrl+"/", http.StripPrefix(baseUrl, http.FileServerFS(c.frontend)))

	// Filebrowser routes
	r.Route("/filebrowser", func(r chi.Router) {
		r.Post("/downloaded", filebrowser.ListDownloaded)
		r.Post("/delete", filebrowser.DeleteFile)
		r.Get("/d/{id}", filebrowser.DownloadFile)
		r.Get("/v/{id}", filebrowser.SendFile)
		r.Get("/s/{id}", filebrowser.SendSubtitle)
		r.Get("/bulk", filebrowser.BulkDownload(c.mdb))
	})

	// Archive routes
	// r.Route("/archive", archive.ApplyRouter(c.db))

	// RPC handlers
	r.Route("/rpc", ytdlpRPC.ApplyRouter())

	// REST API handlers
	r.Route("/api/v1/profiles", profiles.NewRestHandler(c.profileStore, downloaders.Preview))

	r.Route("/api/v1", rest.ApplyRouter(&rest.ContainerArgs{
		DB:          c.db,
		MDB:         c.mdb,
		MQ:          c.mq,
		LM:          c.lm,
		SourceIndex: c.sourceIndex,
	}))

	// Logging
	r.Route("/log", logging.ApplyRouter(observableLogger))

	// Status
	r.Route("/status", status.ApplyRouter(c.mdb))

	// Subscriptions
	r.Route("/subscriptions", subscription.Container(c.db, c.taskRunner).ApplyRouter())

	// Twitch
	r.Route("/twitch", func(r chi.Router) {
		r.Get("/users", twitch.GetMonitoredUsers(c.twitchMonitor))
		r.Post("/user", twitch.MonitorUserHandler(c.twitchMonitor))
		r.Delete("/user/{user}", twitch.DeleteUser(c.twitchMonitor))
	})

	// Pipelines
	r.Route("/pipelines", func(r chi.Router) {
		h := pipeline.NewRestHandler(c.db)
		r.Get("/id/{id}", h.GetPipeline)
		r.Get("/all", h.GetAllPipelines)
		r.Post("/", h.SavePipeline)
		r.Delete("/id/{id}", h.DeletePipeline)
	})

	return &http.Server{Handler: r}
}

func gracefulShutdown(ctx context.Context, srv *http.Server, cfg *serverConfig) {
	<-ctx.Done()
	slog.Info("shutdown signal received")

	defer func() {
		cfg.mdb.Snapshot()
		cfg.db.Close()
		srv.Shutdown(context.Background())
	}()
}
