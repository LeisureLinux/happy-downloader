package profiles

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
)

// warnOnce keeps a per-browser complaint from filling the log.
//
// The check runs for every command line that is built, which is several times
// per download; saying it once is enough to explain what happened.
var (
	warnedMu sync.Mutex
	warned   = map[string]bool{}
	warnOnce = func(browser, message string, args ...any) {
		warnedMu.Lock()
		already := warned[browser]
		warned[browser] = true
		warnedMu.Unlock()

		if already {
			return
		}

		// slog is imported at the call site's package; keeping the variable
		// here avoids importing it twice.
		logWarn(message, append([]any{"browser", browser}, args...)...)
	}
)

// BrowserInstalled reports whether the browser has a cookie database where it
// would be expected.
//
// The names are yt-dlp's, and a profile may follow a colon — "chrome:Profile 1"
// — which is not part of the question: the database exists or it does not.
func BrowserInstalled(browser string) bool {
	name := strings.ToLower(browser)
	if i := strings.IndexAny(name, ":+"); i >= 0 {
		name = name[:i]
	}

	dirs := browserProfileDirs(name)

	if len(dirs) == 0 {
		// A browser this code does not know how to look for is left alone: it
		// may be one yt-dlp supports and this does not, and refusing to use it
		// would be a worse answer than trying.
		return true
	}

	for _, dir := range dirs {
		if statDir(dir) {
			return true
		}
	}

	return false
}

func statDir(path string) bool {
	info, err := os.Stat(path)

	return err == nil && info.IsDir()
}

// browserProfileDirs is where each browser keeps its profiles, per platform.
//
// Only the browsers whose location is known are listed. An empty answer means
// "not known", and the caller tries anyway.
func browserProfileDirs(name string) []string {
	home, _ := os.UserHomeDir()

	switch runtime.GOOS {
	case "windows":
		appData := os.Getenv("APPDATA")
		localAppData := os.Getenv("LOCALAPPDATA")

		switch name {
		case "firefox":
			return []string{filepath.Join(appData, "Mozilla", "Firefox", "Profiles")}
		case "chrome":
			return []string{filepath.Join(localAppData, "Google", "Chrome", "User Data")}
		case "chromium":
			return []string{filepath.Join(localAppData, "Chromium", "User Data")}
		case "edge":
			return []string{filepath.Join(localAppData, "Microsoft", "Edge", "User Data")}
		case "brave":
			return []string{filepath.Join(localAppData, "BraveSoftware", "Brave-Browser", "User Data")}
		case "opera":
			return []string{filepath.Join(appData, "Opera Software", "Opera Stable")}
		case "vivaldi":
			return []string{filepath.Join(localAppData, "Vivaldi", "User Data")}
		}

	case "darwin":
		if home == "" {
			return nil
		}

		support := filepath.Join(home, "Library", "Application Support")

		switch name {
		case "firefox":
			return []string{filepath.Join(support, "Firefox", "Profiles")}
		case "chrome":
			return []string{filepath.Join(support, "Google", "Chrome")}
		case "chromium":
			return []string{filepath.Join(support, "Chromium")}
		case "edge":
			return []string{filepath.Join(support, "Microsoft Edge")}
		case "brave":
			return []string{filepath.Join(support, "BraveSoftware", "Brave-Browser")}
		case "opera":
			return []string{filepath.Join(support, "com.operasoftware.Opera")}
		case "vivaldi":
			return []string{filepath.Join(support, "Vivaldi")}
		}

	default:
		if home == "" {
			return nil
		}

		switch name {
		case "firefox":
			return []string{
				filepath.Join(home, ".mozilla", "firefox"),
				filepath.Join(home, "snap", "firefox", "common", ".mozilla", "firefox"),
				filepath.Join(home, ".var", "app", "org.mozilla.firefox", ".mozilla", "firefox"),
			}
		case "chrome":
			return []string{filepath.Join(home, ".config", "google-chrome")}
		case "chromium":
			return []string{
				filepath.Join(home, ".config", "chromium"),
				filepath.Join(home, "snap", "chromium", "common", "chromium"),
			}
		case "edge":
			return []string{filepath.Join(home, ".config", "microsoft-edge")}
		case "brave":
			return []string{filepath.Join(home, ".config", "BraveSoftware", "Brave-Browser")}
		case "opera":
			return []string{filepath.Join(home, ".config", "opera")}
		case "vivaldi":
			return []string{filepath.Join(home, ".config", "vivaldi")}
		}
	}

	return nil
}
