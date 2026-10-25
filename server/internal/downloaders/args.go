package downloaders

import (
	"io"
	"log/slog"
	"os"
	"os/exec"
	"strings"
	"sync"
	"syscall"

	"github.com/marcopiovanello/yt-dlp-web-ui/v4/server/config"
	"github.com/marcopiovanello/yt-dlp-web-ui/v4/server/internal/cookies"
	"github.com/marcopiovanello/yt-dlp-web-ui/v4/server/internal/profiles"
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
// The profile for the URL decides the arguments (cookie, POT provider, proxy,
// extras). --ignore-config keeps the host's own yt-dlp config out of the
// picture, so the argv is complete by construction and reproducible from the
// log line alone.
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

	if !matched {
		// No profile store (tests): config-file behaviour only.
		cookieStaging = resolveCookiePath()
	}

	args := []string{url}

	if !matched {
		// Legacy path without a profile: keep the config-driven behaviour
		// but still isolate from the host config file.
		args = append(args, "--ignore-config")

		if runtime != "" {
			args = append(args, "--js-runtimes", runtime)
		}
		if c := config.Instance().YtDlp.PotBaseURL; c != "" {
			args = append(args, "--extractor-args", "youtubepot-bgutilhttp:base_url="+c)
		}
		if c := config.Instance().YtDlp.Proxy; c != "" {
			args = append(args, "--proxy", c)
		}
		if c := cookieStaging; c != "" {
			args = append(args, "--cookies", c)
		}

		return append(args, extra...)
	}

	// Profile path: the site decides.
	args = append(args, profile.Argv(cookieStaging, runtime)...)

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
//
// The source here is the per-profile CookiePath, converted on demand when the
// export is newer than the last conversion.
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
// The conversion result is cached next to the export with a ".cookies.txt"
// suffix; when the export is newer the conversion is redone. A failed
// conversion keeps the previous result, so a partially copied file does not
// break a working session.
func convertSource(source string) (string, bool) {
	stagedMu.Lock()
	defer stagedMu.Unlock()

	info, err := os.Stat(source)
	if err != nil || info.IsDir() {
		return "", false
	}

	converted := source + ".cookies.txt"

	ci, err := os.Stat(converted)
	if err == nil && !info.ModTime().After(ci.ModTime()) {
		return converted, true
	}

	raw, err := os.ReadFile(source)
	if err != nil {
		return "", false
	}

	out, _, err := cookiesConvert(raw)
	if err != nil {
		slog.Warn("cookie conversion failed",
			slog.String("source", source), slog.String("err", err.Error()))
		// A previous conversion may still be usable.
		if ci != nil {
			return converted, true
		}
		return "", false
	}

	if err := os.WriteFile(converted, []byte(out), 0o600); err != nil {
		return "", false
	}

	return converted, true
}

// stagedMu guards the per-source conversion cache.
var stagedMu sync.Mutex

// resolveCookiePath is the legacy no-profile path.
func resolveCookiePath() string {
	converted := config.CookiesPath()

	info, err := os.Stat(converted)
	if err != nil || info.IsDir() {
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
// Without the argv there is no way to tell whether the metadata stage or the
// download stage ran, what runtime was in effect, or whether cookies were
// passed at all — the questions that decide whether the next step is to fix
// the config or to pick a different video.
func LogCommand(stage, url string, args []string) {
	argv := append([]string{config.Instance().Paths.DownloaderPath}, args...)

	// Quote arguments containing whitespace so the line is copy-pasteable.
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

// cookiesConvert is aliased so args.go reads without the import list growing;
// the conversion itself lives in the cookies package.
var cookiesConvert = cookies.Convert
