package downloaders

import (
	"context"
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
	"github.com/LeisureLinux/happy-downloader/v4/server/internal/tools"
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
	// The URL is put into the form the site's extractor recognises. People
	// paste links from an app rather than from the address bar, and the two
	// are different addresses for the same video.
	url = NormalizeURL(url)

	// A short link is followed to the page it points at. The extractors do
	// not know the shorteners — Weibo's "t.cn/abc" means nothing to them —
	// and the video is one request away.
	//
	// The failure is not fatal: a link that cannot be followed is passed on
	// as it is, and the extractor will say what it thinks of it.
	if resolved, err := ResolveShortLink(context.Background(), url); err == nil {
		url = resolved
	}

	// Refresh the converted cookie file first: this is the pre-download check
	// that covers a replaced export when the inotify watch is unavailable.
	SyncCookies()

	runtime := tools.Node()

	profile, _ := profiles.ForURL(url)

	// The cookie the download uses.
	//
	// There is one file, not one per site. A browser's export holds the
	// cookies of every site it has visited, so one file carries every
	// session there is, and yt-dlp picks out the ones the site it is
	// fetching asks for.
	//
	// The imported file wins over the automatic one: it is the credential
	// the person using the interface chose — an export from their own
	// machine, signed into their own account. The automatic export belongs
	// to whoever set up the host, which is not always the same person.
	// Whichever export was written last, decided now rather than at startup.
	//
	// It used to be a flag: "the imported one, if there is one". The flag was
	// set when the process started and when a file was uploaded, and never
	// looked at again — so after the automatic export had been refreshed a
	// few times the downloads were still on a file uploaded yesterday, or
	// the other way round, and the interface reported the flag either way.
	cookieSource := cookies.Active().Path

	cookieStaging := ""
	if cookieSource != "" {
		cookieStaging = stageCookieFile(cookieSource)
	}

	// The profile's own extra arguments. Everything else — the cookie, the
	// PO Token provider, the proxy — is read from the configuration when the
	// arguments are built, because they are properties of the installation
	// rather than of a site.
	//
	// Argv appends the profile's own extra args, so they must not be added
	// again here.
	args := append([]string{url}, profile.Argv(url, cookieStaging, runtime)...)

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
	cmd := exec.Command(tools.YtDlp(), args...)
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
	argv := append([]string{tools.YtDlp()}, args...)

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
