package downloaders

import (
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/LeisureLinux/happy-downloader/v4/server/config"
)

// The final path of a finished download.
//
// yt-dlp's `postprocess:` progress template reports `info.filepath` while the
// file is still in the temporary directory: `--paths temp:` puts every
// intermediate there, and the MoveFiles postprocessor only runs afterwards.
// Recording that path as the saved file was the source of a long-lived bug:
// once the file had been moved into the download directory the row said
// "file missing", so people pressed download a second time — and the second
// run succeeded only because yt-dlp found the file already there.
//
// Two sources of truth, in order:
//
//  1. `__finaldir`, when yt-dlp reports it. That is where the file is going,
//     including any subdirectory the request asked for.
//  2. the configured download directory, with the temporary directory's
//     relative path preserved. `-P temp:` mirrors the home path's layout, so
//     "Linux/Video.mp4" under temp belongs at "Linux/Video.mp4" under home.
//
// A path that is already outside the temporary directory is left alone: a
// custom configuration, a livestream recording, or a build without a temp
// directory all produce paths that need no translation.
func finalOutputPath(reported string, finalDir string) string {
	reported = strings.TrimSpace(reported)
	if reported == "" {
		return ""
	}

	home := strings.TrimSpace(config.Instance().Paths.DownloadPath)
	temp := tempDir()

	if finalDir != "" {
		// Resolve relative finaldir against the download directory, as yt-dlp
		// does. A finaldir outside the download directory is still respected:
		// the operator chose it.
		if !filepath.IsAbs(finalDir) && home != "" {
			finalDir = filepath.Join(home, finalDir)
		}
		return filepath.Join(finalDir, filepath.Base(reported))
	}

	if home == "" {
		return reported
	}

	// A path in the temp directory is translated to its home counterpart.
	if temp != "" {
		rel, err := filepath.Rel(temp, reported)
		if err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return filepath.Join(home, rel)
		}
	}

	// A bare filename from a build with no temp directory.
	if !filepath.IsAbs(reported) {
		return filepath.Join(home, reported)
	}

	return reported
}

// fileExists reports whether a finished download left a playable file.
//
// Used to decide whether a non-zero exit from yt-dlp is a real failure or a
// postprocessor that complained about a video which is already on disk.
func fileExists(path string) bool {
	if strings.TrimSpace(path) == "" {
		return false
	}

	info, err := os.Stat(path)
	return err == nil && !info.IsDir() && info.Size() > 0
}

// findFinishedFile is the fallback for a download whose last postprocess
// event never arrived: a postprocessor can fail before yt-dlp emits it.
//
// The output template is the identifier. `%(title)s.%(ext)s` becomes a glob
// for the stem; only a file modified after this run started, in the
// directory the request asked for, can match. Otherwise a retry that failed
// before writing a byte would claim the file left by the first attempt and
// be logged as a success it never earned.
func findFinishedFile(g *GenericDownloader, started time.Time) string {
	root := strings.TrimSpace(config.Instance().Paths.DownloadPath)
	if root == "" {
		return ""
	}

	dir := root
	if requested := strings.TrimSpace(g.output.Path); requested != "" {
		if filepath.IsAbs(requested) {
			dir = requested
		} else {
			dir = filepath.Join(root, requested)
		}
	}

	title := strings.TrimSpace(g.Metadata.Title)
	stem := strings.TrimSpace(g.output.Filename)
	if stem == "" {
		stem = "%(title)s.%(ext)s"
	}
	stem = strings.TrimSuffix(stem, ".%(ext)s")
	stem = strings.ReplaceAll(stem, "%(title)s", title)
	if title == "" || strings.Contains(stem, "%(") {
		return ""
	}

	pattern := filepath.Join(dir, stem+".*")
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return ""
	}

	for _, path := range matches {
		name := filepath.Base(path)
		if strings.Contains(name, ".part") || strings.HasSuffix(name, ".ytdl") {
			continue
		}
		if !fileExists(path) {
			continue
		}
		info, statErr := os.Stat(path)
		if statErr != nil || info.ModTime().Before(started.Add(-time.Second)) {
			continue
		}
		return path
	}

	return ""
}
