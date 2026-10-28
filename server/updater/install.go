package updater

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
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
func updateWithPackageManager(ctx context.Context, manager, version string) error {
	// uv and pip take different arguments for the same thing.
	args := []string{"install", "--upgrade", "yt-dlp==" + version}

	if strings.Contains(filepath.Base(manager), "uv") {
		// uv needs to be told which environment to install into.
		args = append([]string{"pip", "install"}, args...)
		args = append(args, "--python", pythonOf(manager))
	} else {
		args = append([]string{"install", "--upgrade"}, args[2:]...)
	}

	fmt.Printf("updating yt-dlp with %s\n", filepath.Base(manager))

	cmd := exec.CommandContext(ctx, manager, args...)
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
