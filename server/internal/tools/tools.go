package tools

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	"github.com/LeisureLinux/happy-downloader/v4/server/config"
)

// Where the external programs are.
//
// yt-dlp and ffmpeg are separate programs, and they are the two things a
// download cannot happen without. A distribution that ships only the binary
// asks the person installing it to install Python and a media toolkit as
// well, which is a bad afternoon for everybody.
//
// So the programs are looked for in three places, in this order:
//
//  1. the configured path, when there is one — an installation that knows
//     where its ffmpeg is says so, and that is always the answer;
//  2. next to this binary — which is where a bundled distribution puts them,
//     and which is what makes the whole thing a folder that can be copied;
//  3. the PATH — which is where a package manager puts them.
//
// The middle one is the point. It is what lets a Windows build ship as one
// folder with yt-dlp.exe and ffmpeg.exe beside happy-downloader.exe, and it
// is why the executable's own directory is consulted before the PATH: the
// bundled copy is the one the user expects to be used.

// ExecutableSuffix is what an executable is called on this platform.
func ExecutableSuffix() string {
	if runtime.GOOS == "windows" {
		return ".exe"
	}
	return ""
}

// Find looks for an external program.
//
// The name is the plain one — "yt-dlp", "ffmpeg" — and the platform's
// suffix is added here, so a caller does not have to know what it is running
// on. A configured path that already carries the suffix is left alone.
//
// An empty result means it was not found, and the caller says so in terms
// the person can act on.
func Find(name string) (string, error) {
	if name == "" {
		return "", errors.New("no program name was given")
	}

	// A configured path is the answer, whatever it points at. It is not
	// looked up in the PATH, because a path that was configured as one is
	// exactly that.
	if configured := configuredPath(name); configured != "" {
		if executable(configured) {
			return configured, nil
		}

		return "", &NotFoundError{
			Name:   name,
			Path:   configured,
			Reason: "the configured path does not point at an executable",
		}
	}

	// Beside this binary. A bundled distribution puts everything there, and
	// this is the copy the person shipped with.
	if beside := besideExecutable(name); beside != "" {
		return beside, nil
	}

	// And the PATH, which is where a package manager puts them.
	if found, err := exec.LookPath(name); err == nil {
		return found, nil
	}

	return "", &NotFoundError{
		Name:   name,
		Reason: "it is not installed, and it is not beside the application",
	}
}

// NotFoundError says a program is not there.
//
// It says what was looked for and where, because "yt-dlp: not found" on its
// own sends people looking through the PATH when the answer is usually
// "install it, or point at it".
type NotFoundError struct {
	// Name is the program, as it is called.
	Name string
	// Path is the configured path, when there was one.
	Path string
	// Reason is why the other places did not have it.
	Reason string
}

func (e *NotFoundError) Error() string {
	if e.Path != "" {
		return e.Name + " was not found: " + e.Path + " does not point at an executable"
	}

	return e.Name + " was not found: " + e.Reason
}

// configuredPath is the configured location of a program, if there is one.
func configuredPath(name string) string {
	paths := config.Instance().Paths

	switch name {
	case "yt-dlp":
		return paths.DownloaderPath
	case "ffmpeg":
		return paths.FFmpegPath
	}

	return ""
}

// besideExecutable is a program sitting next to this binary.
//
// The executable's own directory is asked for, rather than the working
// directory: a service is started from / and a desktop application from
// wherever the user double-clicked it, and neither is where the program
// lives.
func besideExecutable(name string) string {
	self, err := os.Executable()
	if err != nil {
		return ""
	}

	dir := filepath.Dir(self)

	for _, candidate := range []string{
		name + ExecutableSuffix(),
		name,
	} {
		path := filepath.Join(dir, candidate)

		if executable(path) {
			return path
		}
	}

	return ""
}

// executable reports whether a path is a file that can be run.
//
// On Windows the extension is what makes a file executable, and on Unix it
// is the permission bit. Both are checked, because the file may have come
// from an archive that did not keep them.
func executable(path string) bool {
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		return false
	}

	if runtime.GOOS == "windows" {
		return strings.EqualFold(filepath.Ext(path), ".exe")
	}

	return info.Mode()&0111 != 0
}

// Describe is where each program was found, for the status page.
//
// It is the answer to "why is it using that one", which comes up when a
// machine has two versions of ffmpeg.
func Describe() map[string]string {
	out := map[string]string{}

	for _, name := range []string{"yt-dlp", "ffmpeg"} {
		if configured := configuredPath(name); configured != "" {
			out[name] = configured
			continue
		}

		if beside := besideExecutable(name); beside != "" {
			out[name] = beside
			continue
		}

		if found, err := exec.LookPath(name); err == nil {
			out[name] = found
			continue
		}

		out[name] = ""
	}

	return out
}

// The resolved paths, worked out once.
//
// Looking a program up is a handful of stat calls and, on the first run, a
// walk of the PATH. A download builds its arguments several times — the
// metadata fetch, the format probe, the download itself — and each of them
// would look it up again.
//
// The answer cannot change while the process runs, because the thing it
// depends on is the layout of the disk beside the binary, and that does not
// move under a running program. So it is worked out once and kept.
var (
	resolveOnce sync.Once
	ytDlpPath   string
	ffmpegPath  string
	nodePath    string
)

// Resolve works out where the external programs are.
//
// It is called once at startup. A program that is not there resolves to an
// empty string, and the caller decides what to say about it: refusing to
// start is better than starting and failing on every download.
func Resolve() {
	resolveOnce.Do(func() {
		if found, err := Find("yt-dlp"); err == nil {
			ytDlpPath = found
		}

		if found, err := Find("ffmpeg"); err == nil {
			ffmpegPath = found
		}

		nodePath = resolveNode()
	})
}

// YtDlp is where yt-dlp is, or an empty string when it was not found.
func YtDlp() string {
	Resolve()
	return ytDlpPath
}

// FFmpeg is where ffmpeg is, or an empty string when it was not found.
func FFmpeg() string {
	Resolve()
	return ffmpegPath
}

// Node is the JavaScript runtime, in the form yt-dlp wants it.
//
// YouTube's challenge solving fails without one, so it is worth knowing
// whether there is one: an empty answer here is the difference between a
// download that works and one that fails with a message about a challenge.
func Node() string {
	Resolve()
	return nodePath
}

// resolveNode finds the JavaScript runtime.
//
// It is asked for by name — "node", or whatever the configuration says — and
// the answer is in the form yt-dlp takes: "node" when it is on the PATH, and
// "node:/usr/bin/node" when the configuration named a path.
func resolveNode() string {
	configured := strings.TrimSpace(config.Instance().Paths.JSRuntimePath)

	if configured == "" {
		// yt-dlp finds node on its own when it is on the PATH, and the
		// configuration does not have to say so.
		if found, err := exec.LookPath("node"); err == nil {
			return "node:" + found
		}

		return ""
	}

	if strings.Contains(configured, ":") {
		return configured
	}

	if !strings.HasPrefix(configured, "/") && !strings.HasPrefix(configured, ".") &&
		!strings.HasPrefix(configured, "~") {
		// A bare token is a name, not a path: "node" means the one on the
		// PATH.
		return configured
	}

	return "node:" + configured
}
