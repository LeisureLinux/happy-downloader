package settings

import (
	"time"

	"golang.org/x/sys/unix"
)

// birthTime extracts the file creation time from os.FileInfo.Sys() on Linux.
// Only meaningful where the filesystem records one: ext4 does for fresh
// inodes, tmpfs and btrfs always do.
//
// The return is a zero Time when no birth time is available, and the caller
// falls back to showing just the modification time.
func birthTime(sys any) time.Time {
	if st, ok := sys.(*unix.Stat_t); ok {
		// Ctim is inode change time on Linux; statx BirthTime is not exposed
		// through os.FileInfo. For a file written once and never touched since
		// (an scp target), ctime equals the creation time in practice.
		ts := st.Ctim
		if ts.Sec == 0 && ts.Nsec == 0 {
			return time.Time{}
		}
		return time.Unix(ts.Sec, ts.Nsec)
	}

	return time.Time{}
}
