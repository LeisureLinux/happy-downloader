package config

import (
	"os"
	"path/filepath"
)

// Where the downloads go when nobody has said otherwise.
//
// It used to be ".", which means the working directory — the folder the
// program was started from. On a desktop that is the folder the program was
// unpacked into, so every download landed beside the executable: in
// "Program Files" on a machine where the person cannot write there anyway, and
// in the one place the uninstaller will delete without asking.
//
// The answer is the place the system already keeps downloads, under a folder
// of our own: "Downloads/happy-downloader". It exists on every desktop, it is
// somewhere the person can find without being told, and it is theirs to
// delete.
func DefaultDownloadPath() string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		// Without a home directory there is nowhere better than where the
		// program is, which is at least somewhere it can write.
		return "."
	}

	return filepath.Join(home, "Downloads", "happy-downloader")
}
