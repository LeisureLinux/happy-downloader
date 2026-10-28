package filebrowser

import (
	"os"
	"path/filepath"
	"time"
)

// The figures of a folder.
//
// A folder is shown in the same table as the files, so it needs numbers in
// the same columns. What they say is about the folder's contents rather than
// about the folder:
//
//   - how many files it holds, in the column that would otherwise be the
//     length of a video
//   - when its newest file was written, in the column that would otherwise be
//     when a file was downloaded
//   - how much disk it takes, in the column that would otherwise be a file's
//     size
//
// The newest file is used rather than the folder's own modification time,
// because moving a file into a folder changes the folder's timestamp without
// anything having been downloaded into it.

// folderFigures returns a folder's file count, the time of its newest file and
// its size on disk.
//
// The walk is bounded: a folder with tens of thousands of files would take a
// while to count, and the listing is re-read every time the page is opened.
const folderWalkLimit = 5000

func folderFigures(path string) (count int, newest time.Time, size int64) {
	_ = filepath.Walk(path, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			// A file that cannot be read is skipped rather than failing
			// the whole listing.
			return nil
		}

		if p == path {
			return nil
		}

		count++

		if info.IsDir() {
			return nil
		}

		size += info.Size()

		if info.ModTime().After(newest) {
			newest = info.ModTime()
		}

		if count >= folderWalkLimit {
			return filepath.SkipAll
		}

		return nil
	})

	return count, newest, size
}
