package downloaders

import (
	"io"
	"log/slog"
	"os"
	"os/exec"
	"strings"
	"syscall"

	"github.com/marcopiovanello/yt-dlp-web-ui/v4/server/config"
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

// BuildArgs assembles the yt-dlp argv shared by every code path that shells
// out to the downloader: metadata fetch, format probe, playlist expansion and
// the download itself.
//
// Keeping one builder matters because the stages must agree. When metadata ran
// with a different (thinner) argv than the download, a URL could pass the
// metadata stage and fail the download, or the reverse, with no way to tell
// which stage was at fault from the UI.
func BuildArgs(url string, extra ...string) []string {
	runtime := SanitizeRuntime(config.Instance().Paths.JSRuntimePath)

	args := []string{url}

	if runtime != "" {
		args = append(args, "--js-runtimes", runtime)
	}

	// Cookie file, if one is available. An explicitly configured
	// paths.cookie_path wins; otherwise the file uploaded through the UI is
	// used, so a headless host needs no flag at all. Left empty, yt-dlp still
	// reads its own config.
	if cookie := resolveCookiePath(); cookie != "" {
		args = append(args, "--cookies", cookie)
	}

	return append(args, extra...)
}

// resolveCookiePath picks the cookie file to pass to yt-dlp.
//
// The uploaded file is written next to the database by the REST layer. It is
// preferred over paths.cookie_path because it is the one the UI can refresh
// without a restart, which is the only way to rotate an expiring session on a
// headless host.
// resolveCookiePath picks the cookie file to pass to yt-dlp.
//
// yt-dlp's --cookies both reads the file and writes its cookie jar back into
// it, so the uploaded file is copied to a scratch location first. Handing the
// upload over directly lets yt-dlp replace it with a jar that lacks the login
// cookies the upload was made for — which surfaces as "cookies are no longer
// valid" on the next run.
//
// The uploaded file is preferred over paths.cookie_path because it is the one
// the UI can refresh without a restart, which is the only way to rotate an
// expiring session on a headless host.
func resolveCookiePath() string {
	explicit := strings.TrimSpace(config.Instance().Paths.CookiePath)
	if explicit != "" {
		return explicit
	}

	uploaded := config.CookiesPath()
	info, err := os.Stat(uploaded)
	if err != nil || info.IsDir() {
		return ""
	}

	// Keep the upload pristine and give yt-dlp a copy it may rewrite.
	staging := uploaded + ".ytdlp-jar"

	src, err := os.Open(uploaded)
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
