//go:build !windows

package main

import (
	"context"
	"io/fs"

	"github.com/LeisureLinux/happy-downloader/v4/server"
)

// runFrontend serves the interface, and leaves showing it to a browser.
//
// There is no window here: a server has nobody at the screen, and on a desktop
// the interface belongs to whatever browser the user has. whenReady reports the
// address and does nothing else.
func runFrontend(ctx context.Context, appFS fs.FS) error {
	return server.Run(ctx, &server.RunConfig{
		App:   appFS,
		Ready: whenReady,
	})
}
