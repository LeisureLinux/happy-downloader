//go:build !windows

package sys

import (
	"github.com/LeisureLinux/happy-downloader/v4/server/config"
	"golang.org/x/sys/unix"
)

// FreeSpace gets the available Bytes writable to download directory
func FreeSpace() (uint64, error) {
	var stat unix.Statfs_t

	if err := unix.Statfs(config.Instance().Paths.DownloadPath, &stat); err != nil {
		return 0, err
	}

	return stat.Bavail * uint64(stat.Bsize), nil
}
