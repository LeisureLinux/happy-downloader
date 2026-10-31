//go:build windows

package main

import (
	"context"
	"io/fs"
	"log/slog"

	"github.com/LeisureLinux/happy-downloader/v4/server"
)

// runFrontend serves the interface and shows it.
//
// The server runs in a goroutine and the window on the main one, because a
// window belongs to the thread that created it and its message loop has to run
// there. The address is passed over a channel rather than assumed: the window
// is created when the port is actually listening, not a second before.
func runFrontend(ctx context.Context, appFS fs.FS) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	ready := make(chan string, 1)
	done := make(chan error, 1)

	go func() {
		done <- server.Run(ctx, &server.RunConfig{
			App: appFS,
			// Nothing to report: the window is about to be opened at exactly
			// this address, so opening a browser at it as well would leave two
			// copies of the interface on the screen.
			Ready: func(url string) { ready <- url },
		})
	}()

	var url string

	select {
	case url = <-ready:
	case err := <-done:
		// Already running elsewhere, or a failure before listening. Both are
		// answers, and the browser is the right place to say so.
		if err == nil {
			return nil
		}

		return err
	case <-ctx.Done():
		return nil
	}

	if err := openAppWindow(url); err != nil {
		// No WebView2 on this machine. The browser is what is left, and it is
		// what this program did before it had a window: the interface is the
		// same either way.
		slog.Warn("no window available, opening a browser instead", "err", err)

		whenReady(url)

		return <-done
	}

	// The window was closed. It is the whole interface, so closing it is the
	// same instruction as quitting: the server is stopped the way a signal
	// would stop it, so the database is written and a file in progress is
	// finished.
	slog.Info("window closed; stopping")

	cancel()

	return <-done
}
