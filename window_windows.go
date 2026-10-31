//go:build windows

package main

import (
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	webview "github.com/jchv/go-webview2"

	"github.com/LeisureLinux/happy-downloader/v4/server/config"
)

// The application's own window.
//
// Up to now the program started a server and asked the system to open a
// browser at it. That works everywhere a browser exists, which is everywhere —
// and it is also the reason the interface belongs to whichever browser happens
// to be the default, with its address bar, its tabs and its history.
//
// A WebView2 window is the same interface in a window that belongs to this
// program. It is the same HTML, served from the same port: nothing in the
// interface changes.
//
// There is no cgo here, so the Windows build still cross-compiles from Linux.

// bundledRuntimeDir is a WebView2 runtime shipped beside the program, used
// instead of whatever the machine has.
//
// The Evergreen runtime is present on Windows 10 and 11 and updates itself. On
// Windows 7 it does not exist and cannot be installed any more: the last
// version that runs there is 109, which is also the last version of Chrome for
// Windows 7. Shipping that runtime is what makes one installer work on both.
//
// The WebView2 loader reads WEBVIEW2_BROWSER_EXECUTABLE_FOLDER when it creates
// the environment and uses it in place of the folder it would have chosen, so
// pointing at the bundled copy needs no support from the library.
func bundledRuntimeDir() string {
	exe, err := os.Executable()
	if err != nil {
		return ""
	}

	dir := filepath.Join(filepath.Dir(exe), "webview2")
	if _, err := os.Stat(filepath.Join(dir, "msedgewebview2.exe")); err != nil {
		return ""
	}

	return dir
}

// openAppWindow shows the interface in a window and returns when it closes,
// with how long it was up.
//
// The duration is what separates a window somebody closed from a window that
// never worked. A WebView2 window that cannot be created — no desktop to put it
// on, a runtime that starts and stops — returns from the message loop at once,
// and treating that as "the user closed it" would stop a server that is
// working perfectly well.
//
// It must be called from the main goroutine: the window belongs to the thread
// that created it, and the message loop that runs it is the one that keeps it
// alive.
func openAppWindow(url string) (time.Duration, error) {
	if dir := bundledRuntimeDir(); dir != "" {
		slog.Info("using the bundled WebView2 runtime", "path", dir)
		_ = os.Setenv("WEBVIEW2_BROWSER_EXECUTABLE_FOLDER", dir)
	}

	opts := webview.WebViewOptions{
		// The interface's own storage — localStorage, the theme choice — lives
		// here rather than beside the executable, which on a Windows install is
		// a folder the user may not be able to write to.
		DataPath: filepath.Join(config.DefaultDataDir(), "webview2"),
		WindowOptions: webview.WindowOptions{
			Title:  "Happy Downloader",
			Width:  1180,
			Height: 820,
			Center: true,
		},
	}

	w := webview.NewWithOptions(opts)
	if w == nil {
		return 0, errors.New("WebView2 is not available on this machine")
	}

	defer w.Destroy()

	w.Navigate(url)

	start := time.Now()
	w.Run()

	return time.Since(start), nil
}
