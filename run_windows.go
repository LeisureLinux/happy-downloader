//go:build windows

package main

import (
	"context"
	"io/fs"
	"log/slog"
	"os"
	"time"

	"github.com/LeisureLinux/happy-downloader/v4/server"
)

// runFrontend serves the interface and shows it.
//
// The server runs in a goroutine and the window on the main one, because a
// window belongs to the thread that created it and its message loop has to run
// there. The address is passed over a channel rather than assumed: the window
// is created when the port is actually listening, not a second before.
func runFrontend(ctx context.Context, appFS fs.FS) error {
	// HD_NO_WINDOW=1 serves the interface and opens nothing.
	//
	// It exists for the places where a window is the wrong answer and there is
	// no way to say so: a session that is not on a screen — a scheduled task, a
	// remote session being driven by a script — and the test that installs the
	// program and asks it what it is running.
	if os.Getenv("HD_NO_WINDOW") != "" {
		slog.Info("HD_NO_WINDOW is set: serving without a window")

		return server.Run(ctx, &server.RunConfig{App: appFS, Ready: whenReady})
	}

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

	// A window that was closed within this long was not somebody closing a
	// window: it is a window that could not be shown. There is no desktop to
	// put it on, or the runtime started and gave up.
	const instant = 1500 * time.Millisecond

	lived, err := openAppWindow(url)

	if err != nil {
		// No WebView2 on this machine. The browser is what is left, and it is
		// what this program did before it had a window: the interface is the
		// same either way.
		slog.Warn("no window available, opening a browser instead", "err", err)

		whenReady(url)

		return <-done
	}

	if lived < instant {
		// The server is fine and the window is not. It is left running, and
		// the interface is opened where it can be: stopping a working server
		// because a window failed to appear would make the program look
		// broken on a machine where it works.
		slog.Warn("the window did not stay open; leaving the server running",
			"lived", lived.String())

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
