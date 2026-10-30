// a stupid package name...
package server

import (
	"context"
	"fmt"
	"io/fs"
	"log/slog"
	"net"
	"net/http"
	"net/rpc"
	"strings"

	"github.com/LeisureLinux/happy-downloader/v4/server/config"
	"github.com/LeisureLinux/happy-downloader/v4/server/filebrowser"
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
	conf := config.Instance()

	// The wiring lives in the Core, so that a desktop application can have
	// the same application without a web interface in front of it. There is
	// one way the parts are connected, and it is not the one that happens to
	// serve HTTP.
	core, err := NewCore(ctx, CoreOptions{
		FileLogging: true,
	})
	if err != nil {
		slog.Error("could not start the application", slog.Any("err", err))
		return err
	}
	defer core.Close()

	mdb := core.MDB
	mq := core.MQ
	lm := core.LM
	sourceIndex := core.SourceIndex
	boltdb := core.DB

	scfg := serverConfig{
		frontend:      rc.App,
		mdb:           mdb,
		sourceIndex:   sourceIndex,
		db:            boltdb,
		mq:            mq,
		lm:            lm,
		twitchMonitor: core.TwitchMonitor,
		taskRunner:    core.TaskRunner,
		profileStore:  core.Profiles,
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
		// What can be downloaded for one file: the video and each of its
		// captions, so the interface can offer them one by one.
		r.Get("/downloadable", filebrowser.ListDownloadable)

		// Folders.
		r.Post("/folder", filebrowser.CreateFolder)
		r.Patch("/folder", filebrowser.RenameFolder)
		r.Post("/move", filebrowser.MoveFiles)
		r.Delete("/folder", filebrowser.DeleteFolder)
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
