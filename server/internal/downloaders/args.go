package downloaders

import (
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"

	"github.com/LeisureLinux/happy-downloader/v4/server/config"
	"github.com/LeisureLinux/happy-downloader/v4/server/internal/cookies"
	"github.com/LeisureLinux/happy-downloader/v4/server/internal/profiles"
)

// SanitizeRuntime qualifies a bare runtime path with a "node:" prefix.
//
// yt-dlp expects --js-runtimes to be "RUNTIME[:PATH]". A bare path such as
// /usr/bin/node is rejected with:
//
//	WARNING: Ignoring unsupported JavaScript runtime(s): /usr/bin/node
//	Supported runtimes: deno, node, bun, quickjs.
//
// which silently disables the JS challenge solver. The failure only becomes
// visible much later as an opaque "Sign in to confirm you're not a bot", so
// the obvious config value ("the path to node") has to keep working.
func SanitizeRuntime(runtime string) string {
	runtime = strings.TrimSpace(runtime)
	if runtime == "" || strings.Contains(runtime, ":") {
		return runtime
	}
	// A bare token is only qualified when it looks like a filesystem path.
	// Runtime names such as "node" or "quickjs" are already valid values for
	// --js-runtimes and must be passed through untouched.
	if !strings.HasPrefix(runtime, "/") && !strings.HasPrefix(runtime, ".") &&
		!strings.HasPrefix(runtime, "~") {
		return runtime
	}
	return "node:" + runtime
}

// BuildArgs assembles the yt-dlp argv for a URL from its site profile.
//
// Every invocation goes through this one builder — metadata fetch, format
// probe, playlist expansion and the download itself — because the stages must
// agree: when metadata ran with a thinner argv than the download, a URL could
// pass one stage and fail the next with no way to tell which.
//
// --ignore-config keeps the host's own yt-dlp config out of the picture, so
// the argv is complete by construction and the logged command line is enough
// to reproduce a download. The consequence is that anything the host config
// used to supply — a proxy above all — has to be passed explicitly, or the
// download fails on a network that never needed a profile to work.
func BuildArgs(url string, extra ...string) []string {
	// Refresh the converted cookie file first: this is the pre-download check
	// that covers a replaced export when the inotify watch is unavailable.
	SyncCookies()

	runtime := SanitizeRuntime(config.Instance().Paths.JSRuntimePath)

	profile, matched := profiles.ForURL(url)

	cookieStaging := ""
	if matched && profile.CookiePath != "" {
		cookieStaging = stageCookieFile(profile.CookiePath)
	}

	// The proxy is a property of the network, not of a site: whatever reaches
	// YouTube needs the same egress as everything else. A profile may override
	// it (one site behind a different route), otherwise the global value
	// applies, and without one there is no --proxy flag at all.
	proxy := profile.Proxy
	if proxy == "" {
		proxy = strings.TrimSpace(config.Instance().YtDlp.Proxy)
	}

	// POT provider, likewise: global unless the profile names its own.
	pot := profile.PotBaseURL
	if pot == "" {
		pot = strings.TrimSpace(config.Instance().YtDlp.PotBaseURL)
	}

	args := []string{url, "--ignore-config"}

	if runtime != "" {
		args = append(args, "--js-runtimes", runtime)
	}
	if pot != "" {
		args = append(args, "--extractor-args", "youtubepot-bgutilhttp:base_url="+pot)
	}
	if proxy != "" {
		args = append(args, "--proxy", proxy)
	}
	if cookieStaging != "" {
		args = append(args, "--cookies", cookieStaging)
	}

	// Profile extras come last so they can override anything above.
	if matched {
		args = append(args, profile.ExtraArgs...)
	}

	return append(args, extra...)
}

// cookieSyncHook lets the server refresh the converted cookie file before a
// download starts. It is a package-level function rather than a direct import
// of the watcher so this package stays free of the dependency; the server sets
// it at startup.
//
// It runs before the argv is built, so a cookie export replaced since the last
// download is picked up even when the inotify watch is unavailable.
var cookieSyncHook func()

// SetCookieSyncHook registers the pre-download cookie refresh. Called once at
// startup; passing nil disables it.
func SetCookieSyncHook(fn func()) {
	cookieSyncHook = fn
}

// SyncCookies runs the registered hook, if any. Safe to call at any time.
func SyncCookies() {
	if cookieSyncHook != nil {
		cookieSyncHook()
	}
}

// cookieStagingMu serialises staging-file writes: concurrent downloads would
// otherwise race the copy and hand one yt-dlp process a truncated jar.
var cookieStagingMu sync.Mutex

// stageCookieFile copies a converted cookie export to a scratch path.
//
// yt-dlp's --cookies both reads the file and writes its cookie jar back into
// it. Handing over the converted file directly lets one download replace it
// with a jar missing the login cookies, which surfaces as "cookies are no
// longer valid" on the next run.
func stageCookieFile(source string) string {
	cookieStagingMu.Lock()
	defer cookieStagingMu.Unlock()

	converted, ok := convertSource(source)
	if !ok {
		return ""
	}

	staging := converted + ".ytdlp-jar"

	src, err := os.Open(converted)
	if err != nil {
		return ""
	}
	defer src.Close()

	dst, err := os.OpenFile(staging, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		return ""
	}
	defer dst.Close()

	if _, err := io.Copy(dst, src); err != nil {
		return ""
	}

	return staging
}

// convertSource ensures a Netscape conversion of the export exists and is
// current, returning its path.
//
// The result lives in the state directory, keyed by a hash of the source path,
// so two profiles pointing at different exports do not overwrite each other
// and nothing is written next to the export itself. Writing beside the export
// also collided with the cookie watcher, which maintains the same conversion
// under a different name: one of the two went stale, and the stale one was the
// one downloads read.
//
// A failed conversion keeps the previous result, so a partially copied file
// does not break a working session.
func convertSource(source string) (string, bool) {
	stagedMu.Lock()
	defer stagedMu.Unlock()

	info, err := os.Stat(source)
	if err != nil || info.IsDir() {
		return "", false
	}

	// Resolved through config so the watcher and this builder always agree on
	// the file they maintain.
	converted := config.CookiesPath(source)

	ci, err := os.Stat(converted)
	if err == nil && !info.ModTime().After(ci.ModTime()) {
		return converted, true
	}

	raw, err := os.ReadFile(source)
	if err != nil {
		return "", false
	}

	out, _, err := cookies.Convert(raw)
	if err != nil {
		slog.Warn("cookie conversion failed",
			slog.String("source", source), slog.String("err", err.Error()))
		// A previous conversion may still be usable.
		if ci != nil {
			return converted, true
		}
		return "", false
	}

	if err := os.MkdirAll(filepath.Dir(converted), 0o755); err != nil {
		return "", false
	}

	if err := os.WriteFile(converted, []byte(out), 0o600); err != nil {
		return "", false
	}

	return converted, true
}

// stagedMu guards the conversion cache.
var stagedMu sync.Mutex

// NewCommand builds an *exec.Cmd in the detached process-group shape the
// downloaders use, so Stop() can signal the whole tree.
func NewCommand(args []string) *exec.Cmd {
	cmd := exec.Command(config.Instance().Paths.DownloaderPath, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	return cmd
}

// LogCommand emits the resolved yt-dlp command line.
//
// A download failure surfaces in the UI as a single blob of yt-dlp stderr.
// Without the argv there is no way to tell which stage ran, what runtime was
// in effect, or whether cookies and the proxy were passed at all.
func LogCommand(stage, url string, args []string) {
	argv := append([]string{config.Instance().Paths.DownloaderPath}, args...)

	quoted := make([]string, len(argv))
	for i, a := range argv {
		if strings.ContainsAny(a, " \t\"'") {
			quoted[i] = "'" + strings.ReplaceAll(a, "'", `'\''`) + "'"
		} else {
			quoted[i] = a
		}
	}

	slog.Info("yt-dlp exec",
		slog.String("stage", stage),
		slog.String("url", url),
		slog.String("argv", strings.Join(quoted, " ")),
	)
}

// Preview returns the argv a URL would produce, without starting yt-dlp.
//
// This backs the UI's "what will actually run" panel: the resolved command
// line is the only authoritative answer to which cookie, POT provider, proxy
// and extras apply.
func Preview(url string) []string {
	return BuildArgs(url)
}
