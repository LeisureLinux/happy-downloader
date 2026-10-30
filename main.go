package main

import (
	"context"
	"embed"
	"flag"
	"io/fs"
	"log/slog"
	"os"
	"os/signal"
	"runtime"
	"strings"
	"syscall"

	"github.com/LeisureLinux/happy-downloader/v4/server"
	"github.com/LeisureLinux/happy-downloader/v4/server/config"

	"github.com/spf13/viper"
)

//go:embed frontend/dist/index.html
//go:embed frontend/dist/assets/*
var frontend embed.FS

func main() {
	// The desktop build has no terminal, so what a terminal would have said
	// is said by other means.
	setupDesktop()

	// Parse optional config path from flag
	var configFile string
	flag.StringVar(&configFile, "conf", config.DefaultConfigPath(), "Config file path")
	flag.Parse()

	// The configuration is written back when the interface changes a setting
	// that lives in it, so the server has to know where it was read from.
	config.SetFilePath(configFile)

	v := viper.New()
	v.SetConfigFile(configFile)
	v.SetConfigType("yaml")

	// Defaults
	v.SetDefault("server.host", "0.0.0.0")
	v.SetDefault("server.port", 3033)
	v.SetDefault("server.queue_size", runtime.NumCPU())
	// The downloads go somewhere the person can find, not beside the
	// program: they are the point of the program, and the folder the
	// program was unpacked into is both hard to find and not theirs.
	v.SetDefault("paths.download_path", config.DefaultDownloadPath())
	v.SetDefault("paths.downloader_path", "yt-dlp")
	// The database is written to, so it goes where the application is
	// allowed to write rather than beside itself.
	if dir := config.DefaultDataDir(); dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			slog.Error("cannot create the data directory", "path", dir, "err", err)
		}
		v.SetDefault("paths.local_database_path", dir)
		v.SetDefault("logging.log_path", dir+string(os.PathSeparator)+"happy-downloader.log")
	} else {
		v.SetDefault("paths.local_database_path", ".")
		v.SetDefault("logging.log_path", "happy-downloader.log")
	}

	// File logging is on by default on Windows, because there is no console
	// to read: a windowed build that keeps its log only in memory has no log
	// to look at when something goes wrong.
	v.SetDefault("logging.enable_file_logging", config.DefaultDataDir() != "")

	// Env binding
	v.SetEnvPrefix("APP")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	if err := v.ReadInConfig(); err != nil {
		slog.Debug("using defaults")
	}

	cfg := config.Instance()
	if err := v.Unmarshal(&cfg); err != nil {
		slog.Error("failed to load config", "error", err)
	}

	if cfg.Server.QueueSize <= 0 || runtime.NumCPU() <= 2 {
		cfg.Server.QueueSize = 2
	}

	var appFS fs.FS
	if fp := v.GetString("frontend_path"); fp != "" {
		appFS = os.DirFS(fp)
	} else {
		sub, err := fs.Sub(frontend, "frontend/dist")
		if err != nil {
			slog.Error("failed to load embedded frontend", "error", err)
			os.Exit(1)
		}
		appFS = sub
	}

	// Graceful shutdown
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	slog.Info("starting server",
		"host", cfg.Server.Host,
		"port", cfg.Server.Port,
		"queue_size", cfg.Server.QueueSize,
		"downloader", cfg.Paths.DownloaderPath,
		"download_dir", cfg.Paths.DownloadPath,
	)

	if err := server.Run(ctx, &server.RunConfig{
		App:   appFS,
		Ready: whenReady,
	}); err != nil {
		slog.Error("server stopped with error", "error", err)
		whenFatal(err.Error())
		os.Exit(1)
	}

	slog.Info("server exited cleanly")
}
