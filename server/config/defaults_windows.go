//go:build windows

package config

import (
	"os"
	"path/filepath"
)

// Where the application keeps its own files on Windows.
//
// It used to be ".", the working directory, which for a program started from
// a shortcut is the folder the program was unpacked into — "Program Files",
// where a user without administrator rights cannot write. The database, the
// configuration and the log all failed to save there, and the failure showed
// up as "settings do not stick" rather than as a permission error.
//
// %APPDATA% is the place Windows provides for exactly this, it belongs to the
// user, and it survives the program being moved or reinstalled.
func DefaultDataDir() string {
	if dir, err := os.UserConfigDir(); err == nil && dir != "" {
		return filepath.Join(dir, "happy-downloader")
	}

	return ""
}

// The configuration file, beside the rest of the application's files.
func DefaultConfigPath() string {
	dir := DefaultDataDir()
	if dir == "" {
		return "./config.yml"
	}

	return filepath.Join(dir, "config.yml")
}
