package rest

import (
	"context"
	"log/slog"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/LeisureLinux/happy-downloader/v4/server/config"
)

// The yt-dlp version, and how long it is worth remembering.
//
// Asking yt-dlp for its version is not free. The standalone binary takes
// four seconds to start — it is a self-contained PyInstaller bundle that
// unpacks itself — and the pip-installed one takes nearly two. The version
// endpoint is called on every page load, because the footer shows it, so the
// page waited four seconds for it.
//
// The version does not change between updates, so it is remembered: the first
// call asks, and the rest are answered from memory. The cache is dropped
// when yt-dlp is updated, because that is the only moment the answer can
// change.
var (
	versionMu    sync.RWMutex
	versionCache string
	versionAt    time.Time
)

// The version is remembered for an hour. It cannot change without an update,
// and an update drops the cache by itself, so this is only a bound on how
// long a stale answer survives a restart that happened to miss the drop.
const versionCacheTTL = time.Hour

// ytDlpVersion returns the version of the installed yt-dlp.
func ytDlpVersion(ctx context.Context) string {
	versionMu.RLock()
	cached := versionCache
	fresh := time.Since(versionAt) < versionCacheTTL
	versionMu.RUnlock()

	if cached != "" && fresh {
		return cached
	}

	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, config.Instance().Paths.DownloaderPath, "--version")

	out, err := cmd.Output()
	if err != nil {
		// A yt-dlp that will not report its version is a yt-dlp that will
		// not download either. The cached answer is better than nothing.
		slog.Debug("could not read the yt-dlp version", slog.Any("err", err))
		return cached
	}

	// yt-dlp prints a bare date; some channels prefix a "v". Strip it so the
	// footer does not render "yt-dlp vv2026.10.21".
	version := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(string(out)), "v"))

	versionMu.Lock()
	versionCache = version
	versionAt = time.Now()
	versionMu.Unlock()

	return version
}

// ForgetYtdlpVersion drops the remembered version.
//
// It is called after an update, which is the only moment the answer can
// change. The RPC package calls it, so it is exported.
func ForgetYtdlpVersion() {
	versionMu.Lock()
	versionCache = ""
	versionAt = time.Time{}
	versionMu.Unlock()
}
