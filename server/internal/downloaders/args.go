package downloaders

import (
	"log/slog"
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

	// An explicitly configured cookie file. Left empty, yt-dlp still picks up
	// the user's own config, so this is additive rather than required.
	if cookie := strings.TrimSpace(config.Instance().Paths.CookiePath); cookie != "" {
		args = append(args, "--cookies", cookie)
	}

	return append(args, extra...)
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
