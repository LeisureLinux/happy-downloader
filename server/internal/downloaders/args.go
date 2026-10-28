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
	"time"

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

	// The cookie the download uses: the one the user imported, if there is
	// one, and the automatic export otherwise.
	//
	// The imported one wins because it is the credential the person using
	// the interface chose — a browser extension export from their own
	// machine, signed into their own account. The automatic export belongs to
	// whoever set up the host, which is not always the same person.
	cookieSource := profile.CookiePath
	if matched && cookieSource == "" {
		cookieSource = config.ResolvedCookieSource()
	}

	if imported := ImportedCookiePath(); imported != "" {
		cookieSource = imported
	}

	cookieStaging := ""
	if cookieSource != "" {
		cookieStaging = stageCookieFile(cookieSource)
	}

	// The proxy is a property of the site, not of the network.
	//
	// It used to be the other way round: the global setting was the
	// fallback for every site a profile did not name, so a proxy that was
	// there for YouTube was also sent to Bilibili — and Bilibili answers a
	// request from another country with 412 rather than with the video.
	//
	// So a proxy is now something a site asks for, and an empty proxy in a
	// profile means "reach it directly". The global setting is not a
	// fallback any more; it is the value the profiles are seeded with, which
	// is where a proxy for YouTube belongs.
	proxy := profile.Proxy

	// POT provider, likewise: global unless the profile names its own.
	pot := profile.PotBaseURL
	if pot == "" {
		pot = strings.TrimSpace(config.Instance().YtDlp.PotBaseURL)
	}

	// One implementation, in Profile.Argv: two copies of this logic is how the
	// POT provider lost its proxy the first time. The resolved values are
	// pushed into a copy of the profile so Argv stays free of the config
	// singleton and remains directly testable.
	resolved := profile
	resolved.Proxy = proxy
	resolved.PotBaseURL = pot

	// Argv appends the profile's own extra args, so they must not be added
	// again here.
	args := append([]string{url}, resolved.Argv(cookieStaging, runtime)...)

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

// stageCookieFile copies a converted cookie export to a scratch file for one
// yt-dlp invocation.
//
// Two reasons for the copy. yt-dlp's --cookies both reads the file and writes
// its cookie jar back into it, so handing over the converted file directly
// would let a download replace it with a jar missing the login cookies. And
// the copy has to be per-invocation: yt-dlp rewrites the file it was given,
// so a shared staging path means one process truncates it while another is
// still reading, and that reader fails with a cookie error that has nothing to
// do with the cookies.
func stageCookieFile(source string) string {
	converted, ok := convertSource(source)
	if !ok {
		return ""
	}

	src, err := os.Open(converted)
	if err != nil {
		return ""
	}
	defer src.Close()

	// The state directory, not the OS temp dir: it is on the same filesystem as
	// the converted file and is already the place this server writes to.
	dst, err := os.CreateTemp(filepath.Dir(converted), "cookies-*.jar")
	if err != nil {
		return ""
	}
	defer dst.Close()

	if err := dst.Chmod(0o600); err != nil {
		slog.Debug("could not restrict cookie jar permissions", slog.String("err", err.Error()))
	}

	if _, err := io.Copy(dst, src); err != nil {
		os.Remove(dst.Name())
		return ""
	}

	// Jars are disposable and one is created per invocation, so sweep the ones
	// from finished downloads. Without this the state directory accumulates a
	// file per download forever.
	sweepStaleJars(filepath.Dir(converted))

	return dst.Name()
}

// staleJarAge is how long a staging jar is kept before being swept. Generous
// enough that a long download is never racing its own cleanup.
const staleJarAge = 6 * time.Hour

func sweepStaleJars(dir string) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}

	cutoff := time.Now().Add(-staleJarAge)

	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".jar") ||
			!strings.HasPrefix(e.Name(), "cookies-") {
			continue
		}
		info, err := e.Info()
		if err != nil || info.ModTime().After(cutoff) {
			continue
		}
		os.Remove(filepath.Join(dir, e.Name()))
	}
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
//
// This is also where a new attempt at using the imported cookie begins. Every
// yt-dlp invocation goes through here, and one invocation makes several
// requests, so yt-dlp reports a rotated cookie once per request — counting
// each report would have removed the import after one attempt rather than
// two.
func NewCommand(args []string) *exec.Cmd {
	cmd := exec.Command(config.Instance().Paths.DownloaderPath, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	cookies.NewAttempt()

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

// stripOutputArgs removes an -o/--output pair left in a stored request.
//
// Older builds appended the resolved output path to the downloader's own
// params, so entries created before that was fixed carry an -o that the
// argument whitelist rejects. Retrying one of those would fail with "param -o
// not allowed" forever; the output path is re-derived on start anyway.
func StripOutputArgs(params []string) []string {
	clean := make([]string, 0, len(params))

	for i := 0; i < len(params); i++ {
		switch params[i] {
		case "-o", "--output":
			i++ // drop the flag and its value
		default:
			clean = append(clean, params[i])
		}
	}

	return clean
}
