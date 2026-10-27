package downloaders

import (
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/LeisureLinux/happy-downloader/v4/server/config"
)

// Where the in-progress files live.
//
// yt-dlp has a first-class design for this: `--paths temp:DIR` sends every
// intermediate file to DIR and moves the finished file to the home path when
// the download completes. The partial file is a `.part` file next to where
// the video will go, and yt-dlp resumes it by default.
//
// So the download directory holds only finished files, and the half-downloaded
// ones live in a directory of their own. That is what stops the file browser
// from listing a half-written video as if it were a download that had
// finished.
//
// There is a catch, and it is in yt-dlp's own documentation: "--paths temp:"
// is ignored when "--output" is an absolute path. The output template has to
// be relative, with the home path given separately as "--paths home:". So the
// two are always passed together, never one without the other.

// tempDir returns the directory for in-progress files.
//
// It lives under the state directory rather than under the download
// directory, so that it is on the same filesystem as the rest of the
// application's state and is not mistaken for a download.
//
// When there is no state directory configured — tests, or a partial startup —
// the OS temp directory is used. Falling back to the download directory would
// put the partial files back where the problem was.
func tempDir() string {
	dir := strings.TrimSpace(config.Instance().Paths.LocalDatabasePath)

	if dir == "" {
		return filepath.Join(os.TempDir(), "happy-downloader")
	}

	return filepath.Join(dir, "partial")
}

// outputPaths returns the "--paths" arguments for a download, and whether a
// temporary directory could be prepared.
//
// The download directory is the home path, because that is where the finished
// file belongs. The temporary directory holds the partial file until the
// download completes.
func outputPaths() (home string, temp string, err error) {
	home = config.Instance().Paths.DownloadPath

	if home == "" {
		return "", "", errors.New("no download path is configured")
	}

	temp = tempDir()

	if err := os.MkdirAll(temp, 0o755); err != nil {
		// The temporary directory could not be created, so there is nowhere
		// to put the partial file. The download still works — it just leaves
		// the partial file next to the finished one, which is what yt-dlp
		// did before this existed. Better than refusing to download.
		return home, "", nil
	}

	return home, temp, nil
}

// pathArgs returns the yt-dlp arguments that place the output.
//
// The template is relative because that is the only way "--paths temp:" is
// honoured; the directories are passed as "--paths home:" and "--paths
// temp:" instead.
func pathArgs(home, temp string, filename string) []string {
	args := []string{"-P", "home:" + home}

	if temp != "" {
		args = append(args, "-P", "temp:"+temp)
	}

	// The template is relative to the home path.
	//
	// It used to be an absolute path, and that is what made "--paths temp:"
	// impossible: yt-dlp ignores the temporary path when the output is
	// absolute. The path is still checked against the download directory
	// before it is used, so a name that tries to escape it is refused.
	return append(args, "-o", filename)
}

// relativeOutputName returns the output template, relative to the download
// directory.
//
// An absolute path is turned into one relative to the download directory, and
// a name that tries to leave it is an error. The check is the same one the
// absolute form used, moved before the conversion rather than after it.
func relativeOutputName(outputPath, filename string) (string, error) {
	root := config.Instance().Paths.DownloadPath

	rel, err := filepath.Rel(root, outputPath)
	if err != nil {
		return "", err
	}

	if strings.HasPrefix(rel, "..") {
		return "", errors.New(ErrIsNotSubPath)
	}

	return rel, nil
}
