//go:build windows

package sys

import (
	"github.com/LeisureLinux/happy-downloader/v4/server/config"
	"golang.org/x/sys/windows"
)

// FreeSpace gets the available Bytes writable to download directory.
//
// Windows has no statfs, so the free space is asked of the volume the
// download directory is on. GetDiskFreeSpaceEx is the one that reports the
// space available to the caller rather than the space on the volume, which
// is the number a person can use.
func FreeSpace() (uint64, error) {
	path, err := windows.UTF16PtrFromString(config.Instance().Paths.DownloadPath)
	if err != nil {
		return 0, err
	}

	var available, total, totalFree uint64

	if err := windows.GetDiskFreeSpaceEx(path, &available, &total, &totalFree); err != nil {
		return 0, err
	}

	return available, nil
}
