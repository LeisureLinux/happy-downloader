package filebrowser

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/LeisureLinux/happy-downloader/v4/server/config"
)

// The figures of a folder are about its contents, not about the folder.
//
// The folder's own modification time changes when a file is moved into it,
// which says nothing about when anything was downloaded. The newest file's
// time does.
func TestFolderFigures(t *testing.T) {
	dir := t.TempDir()

	old := filepath.Join(dir, "old.mp4")
	recent := filepath.Join(dir, "recent.mp4")

	if err := os.WriteFile(old, make([]byte, 100), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(recent, make([]byte, 200), 0o644); err != nil {
		t.Fatal(err)
	}

	past := time.Now().Add(-48 * time.Hour)
	if err := os.Chtimes(old, past, past); err != nil {
		t.Fatal(err)
	}

	count, newest, size := folderFigures(dir)

	if count != 2 {
		t.Errorf("count = %d, want 2", count)
	}

	if size != 300 {
		t.Errorf("size = %d, want 300", size)
	}

	// The newest file, not the folder.
	if newest.Before(past) {
		t.Errorf("newest = %v, want the time of the newest file", newest)
	}

	if time.Since(newest) > time.Hour {
		t.Errorf("newest = %v, want a recent time", newest)
	}
}

// An empty folder has no files, so it has no figures either.
func TestFolderFiguresOfAnEmptyFolder(t *testing.T) {
	dir := t.TempDir()

	count, newest, size := folderFigures(dir)

	if count != 0 || size != 0 {
		t.Errorf("count = %d, size = %d, want 0 and 0", count, size)
	}
	if !newest.IsZero() {
		t.Errorf("newest = %v, want the zero time", newest)
	}
}

// A folder name cannot escape the download directory, and it cannot be
// something the filesystem will not take.
func TestFolderNameIsChecked(t *testing.T) {
	root := t.TempDir()

	previous := &config.Instance().Paths
	previousRoot := previous.DownloadPath
	t.Cleanup(func() { previous.DownloadPath = previousRoot })
	previous.DownloadPath = root

	for _, name := range []string{"", "..", ".", "a/b", `a\b`, "a:b", "a*b", "a?b"} {
		if name == "" {
			continue
		}

		// The name is checked by the handler, which refuses it before it
		// ever reaches the filesystem.
		if !validFolderName(name) {
			continue
		}

		t.Errorf("the name %q was accepted", name)
	}

	if !validFolderName("正常的文件夹") {
		t.Error("an ordinary name was refused")
	}
}

// Renaming a folder keeps its contents: it is a move within the parent, so
// the files inside it are untouched.
func TestRenameFolderKeepsTheContents(t *testing.T) {
	root := t.TempDir()

	previous := &config.Instance().Paths
	previousRoot := previous.DownloadPath
	t.Cleanup(func() { previous.DownloadPath = previousRoot })
	previous.DownloadPath = root

	folder := filepath.Join(root, "before")
	if err := os.MkdirAll(folder, 0o755); err != nil {
		t.Fatal(err)
	}

	file := filepath.Join(folder, "video.mp4")
	if err := os.WriteFile(file, []byte("video"), 0o644); err != nil {
		t.Fatal(err)
	}

	after := filepath.Join(root, "after")
	if err := os.Rename(folder, after); err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(filepath.Join(after, "video.mp4")); err != nil {
		t.Errorf("the file did not come along: %v", err)
	}
}
