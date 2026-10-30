package updater

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/LeisureLinux/happy-downloader/v4/server/internal/tools"
)

// How yt-dlp was installed, and how it should be updated.
//
// The two installations are not interchangeable and they are not equally
// good:
//
//   - A package installation — pip or uv — is a small script that runs the
//     interpreter. It starts in 1.7 seconds.
//   - A standalone binary is a self-contained bundle that unpacks itself on
//     every run. It starts in 4.2 seconds.
//
// Every download spawns yt-dlp, so the difference is paid on every download.
// Replacing a package installation with the standalone binary would make
// every download two and a half seconds slower, to no benefit.
//
// So the install method is detected and kept: a package installation is
// updated by its package manager, and a binary is replaced by a binary.

// packageManager returns the package manager that installed yt-dlp, or an
// empty string when it was installed some other way.
//
// The test is the yt-dlp itself: the pip and uv installations are a script
// that begins with a shebang pointing at an interpreter, and the standalone
// one is an ELF binary.
func packageManager(ytdlpPath string) string {
	file, err := os.Open(ytdlpPath)
	if err != nil {
		return ""
	}
	defer file.Close()

	header := make([]byte, 4)
	if _, err := file.Read(header); err != nil {
		return ""
	}

	// An ELF binary: not a package installation.
	if header[0] == 0x7f && header[1] == 'E' {
		return ""
	}

	// A script. Its first line says which interpreter it uses, and the
	// directory it lives in says which environment it belongs to.
	first, err := os.ReadFile(ytdlpPath)
	if err != nil {
		return ""
	}

	line := string(first)
	if i := strings.IndexByte(line, '\n'); i >= 0 {
		line = line[:i]
	}

	if !strings.HasPrefix(line, "#!") {
		return ""
	}

	// The environment is the one the script lives in: .../venv/bin/yt-dlp
	// belongs to .../venv.
	dir := filepath.Dir(ytdlpPath)

	if name, ok := executableIn(dir, "uv"); ok {
		return name
	}

	if name, ok := executableIn(dir, "pip3"); ok {
		return name
	}

	if name, ok := executableIn(dir, "pip"); ok {
		return name
	}

	// Not in the environment. It may still be on the path — a venv created
	// by uv has no uv of its own, and the one that made it is elsewhere.
	if path := lookupInPath("uv"); path != "" {
		return path
	}

	// And it may be in the per-user directory, which is where a tool
	// installed without a package manager ends up. It is not on the PATH of
	// a service started by the system, which is how this server runs.
	if home, err := os.UserHomeDir(); err == nil {
		if path, ok := executableIn(filepath.Join(home, ".local", "bin"), "uv"); ok {
			return path
		}
	}

	return ""
}

// lookupInPath finds an executable on the path.
func lookupInPath(name string) string {
	for _, dir := range filepath.SplitList(os.Getenv("PATH")) {
		if dir == "" {
			continue
		}

		if path, ok := executableIn(dir, name); ok {
			return path
		}
	}

	return ""
}

// executableIn looks for an executable in a directory.
func executableIn(dir, name string) (string, bool) {
	path := filepath.Join(dir, name)

	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		return "", false
	}

	if info.Mode()&0o111 == 0 {
		return "", false
	}

	return path, true
}

// updateWithPackageManager updates yt-dlp through its package manager.
//
// The version is pinned, so a package index that has moved on does not
// decide what gets installed: the release that was checked is the release
// that is installed.
func updateWithPackageManager(ctx context.Context, manager, ytdlpPath, version string) error {
	// The two take different arguments for the same thing.
	//
	//   uv:  uv pip install --upgrade yt-dlp==<version> --python <env>
	//   pip: pip install --upgrade yt-dlp==<version>
	//
	// The version is pinned, so a package index that has moved on does not
	// decide what gets installed: the release that was checked is the
	// release that is installed.
	requirement := "yt-dlp==" + version

	var args []string

	if strings.Contains(filepath.Base(manager), "uv") {
		args = []string{"pip", "install", "--upgrade", requirement}

		// uv has to be told which environment to install into, or it picks
		// one of its own.
		if python := pythonOf(ytdlpPath); python != "" {
			args = append(args, "--python", python)
		}
	} else {
		args = []string{"install", "--upgrade", requirement}
	}

	fmt.Printf("updating yt-dlp with %s\n", filepath.Base(manager))

	cmd := exec.CommandContext(ctx, manager, args...)
	tools.Quiet(cmd)

	out, err := cmd.CombinedOutput()

	if err != nil {
		return fmt.Errorf("%s failed: %w\n%s", filepath.Base(manager), err, out)
	}

	fmt.Println(strings.TrimSpace(string(out)))

	return nil
}

// pythonOf returns the interpreter that belongs to a package manager.
//
// uv is told which environment to install into, and the environment is the
// one the manager lives in: .../venv/bin/uv belongs to .../venv.
func pythonOf(manager string) string {
	dir := filepath.Dir(manager)

	for _, name := range []string{"python3", "python"} {
		if path, ok := executableIn(dir, name); ok {
			return path
		}
	}

	return ""
}
