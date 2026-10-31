//go:build windows

package main

import (
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	webview "github.com/jchv/go-webview2"
	"github.com/jchv/go-webview2/webviewloader"

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

// The runtime to use, and where it came from.
//
// The Evergreen runtime on the machine wins whenever it is there. It is the
// one the operating system keeps up to date, and the one the machine's browser
// is: the bundled copy is Chromium 109, and a 109 renders like Chrome 109 —
// which is not how the browser next to it renders. The difference shows up as
// "this video plays in Chrome and not in the window", and it is not a bug in
// either of them.
//
// The bundled copy is for the machines that have nothing else. Windows 7 is
// that machine: the last runtime that runs there is 109, and it can no longer
// be installed.
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
	// What the machine has, if anything. An empty answer with no error means
	// WebView2 is not installed at all.
	installed, err := webviewloader.GetInstalledVersion()
	if err != nil {
		slog.Warn("cannot tell which WebView2 runtime is installed", "err", err)
	}

	switch {
	case installed != "":
		// Nothing to set: the loader finds this one by itself, and it is
		// newer than anything that could be shipped here.
		slog.Info("using the installed WebView2 runtime", "version", installed)

	default:
		if dir := bundledRuntimeDir(); dir != "" {
			slog.Info("no WebView2 runtime installed; using the bundled one", "path", dir)
			_ = os.Setenv("WEBVIEW2_BROWSER_EXECUTABLE_FOLDER", dir)
		} else {
			slog.Warn("no WebView2 runtime installed and none bundled")
		}
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
			// The icon in the title bar and the taskbar, taken from this
			// executable's own resources.
			//
			// Without it the window wears the generic icon of a program with no
			// icon, next to a taskbar entry that has the right one — because the
			// taskbar reads the executable and the window reads whatever the
			// window class was given. The build embeds the icon as resource 1
			// through go-winres, which is where this number comes from.
			IconId: 1,
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
