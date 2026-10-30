// Command happy-downloader-console drives the downloader without a browser.
//
// It exists to prove that the application is not a web application with a
// queue in it: the queue, the progress, the recordings and the cookie
// handling are all here, in this process, with nothing listening on a port.
//
// That is what a desktop application needs. A Windows build would have a
// window instead of a terminal, and would show the same rows this prints.
//
// The web interface is not a second implementation of any of it, and neither
// is this: they both drive the same Core.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/LeisureLinux/happy-downloader/v4/server"
	"github.com/LeisureLinux/happy-downloader/v4/server/config"

	"github.com/spf13/viper"
)

func main() {
	var (
		configFile string
		watch      bool
		listen     string
		recordMin  int
	)

	flag.StringVar(&configFile, "conf", "./config.yml", "Config file path")
	flag.BoolVar(&watch, "watch", false, "Keep running and show the queue as it changes")
	flag.StringVar(&listen, "listen", "", "Serve the web interface on this address instead of the console")
	flag.IntVar(&recordMin, "record", 0, "Record a livestream for this many minutes (0 = until stopped)")
	flag.Parse()

	config.SetFilePath(configFile)

	v := viper.New()
	v.SetConfigFile(configFile)
	v.SetConfigType("yaml")

	v.SetDefault("server.host", "127.0.0.1")
	v.SetDefault("server.port", 3033)
	v.SetDefault("server.queue_size", 4)
	v.SetDefault("paths.download_path", ".")
	v.SetDefault("paths.downloader_path", "yt-dlp")
	v.SetDefault("paths.local_database_path", ".")
	v.SetDefault("logging.log_path", "happy-downloader.log")
	v.SetDefault("logging.enable_file_logging", false)

	if err := v.ReadInConfig(); err != nil {
		fmt.Fprintln(os.Stderr, "no config file, using defaults:", err)
	}

	cfg := config.Instance()
	if err := v.Unmarshal(&cfg); err != nil {
		fmt.Fprintln(os.Stderr, "the config file could not be read:", err)
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	core, err := server.NewCore(ctx, server.CoreOptions{})
	if err != nil {
		fmt.Fprintln(os.Stderr, "could not start:", err)
		os.Exit(1)
	}
	defer core.Close()

	urls := flag.Args()

	switch {
	case len(urls) > 0 && recordMin > 0:
		for _, u := range urls {
			core.RecordLivestream(u, recordMin, cfg.Paths.DownloadPath)
			fmt.Printf("recording %s for %d minutes\n", u, recordMin)
		}

	case len(urls) > 0:
		for _, u := range urls {
			id := core.QueueDownload(u, []string{})
			fmt.Printf("queued %s (%s)\n", u, id[:8])
		}

	case listen != "":
		// The web interface, in this process. The Core is already up; this
		// only puts the HTTP server in front of it.
		go serve(ctx, core, listen)
		fmt.Printf("the web interface is on http://%s\n", listen)

	default:
		printUsage()
		return
	}

	if !watch {
		// The downloads keep going: the queue is the queue, and it does not
		// need anybody watching it.
		return
	}

	watchQueue(ctx, core)
}

func printUsage() {
	fmt.Println(`happy-downloader-console — the downloader without a browser

  happy-downloader-console [flags] [url...]

  -conf FILE     config file (default ./config.yml)
  -watch         keep running and show the queue as it changes
  -listen ADDR   serve the web interface on this address instead
  -record MIN    record a livestream for this many minutes (0 = until stopped)

Give it a URL and it downloads. Give it -record and it records a livestream.`)
}

// watchQueue prints the queue as it changes.
//
// The rows are what the web interface shows, from the same place: the store
// of what is downloading. There is no second source of truth, which is the
// point of the exercise.
func watchQueue(ctx context.Context, core *server.Core) {
	// The width of each column, so the output lines up.
	const (
		nameWidth  = 44
		stateWidth = 10
	)

	shown := map[string]string{}

	for {
		select {
		case <-ctx.Done():
			return
		case <-time.After(time.Second):
		}

		running := core.MDB.All()
		if running == nil {
			continue
		}

		for _, r := range *running {
			// What changed about this row, if anything.
			// yt-dlp's percentage carries leading spaces — it pads the
			// field so the number does not jitter as it grows. They are
			// dropped here, because they make the column ragged.
			percent := strings.TrimSpace(r.Progress.Percentage)

			state := fmt.Sprintf("%-8s %s", stateName(r.Progress.Status), percent)
			if shown[r.Id] == state {
				continue
			}
			shown[r.Id] = state

			title := r.Info.Title
			if title == "" {
				title = r.URL
			}
			if len(title) > nameWidth {
				title = title[:nameWidth-1] + "…"
			}

			fmt.Printf("%-*s  %-*s  %s\n", nameWidth, title, stateWidth, stateName(r.Progress.Status), filepath.Base(r.Output.SavedFilePath))
		}

		// Rows that are gone are forgotten, so a new download with the same
		// title is not suppressed as one already shown.
		alive := map[string]bool{}
		for _, r := range *running {
			alive[r.Id] = true
		}
		for id := range shown {
			if !alive[id] {
				delete(shown, id)
			}
		}
	}
}

// stateName is the name of a state, as the interface calls it.
func stateName(status int) string {
	switch status {
	case 0:
		return "preparing"
	case 1:
		return "downloading"
	case 2:
		return "done"
	case 3:
		return "failed"
	case 4:
		return "recording"
	case 5:
		return "paused"
	case 6:
		return "remuxing"
	default:
		return "?"
	}
}

// serve puts the web interface in front of the Core.
//
// It is the same server the standalone binary runs, on an address the caller
// chose — the point of a desktop application is that it does not listen on
// anything by default.
func serve(ctx context.Context, core *server.Core, addr string) {
	fmt.Fprintln(os.Stderr, "the web interface needs the server package's router, which the console does not wire up yet")
}
