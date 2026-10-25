//go:build !linux

package settings

import "time"

// Non-Linux platforms: os.FileInfo.Sys() has no portable birth-time shape.
// The modification time alone is reported instead.

func birthTime(_ any) time.Time { return time.Time{} }
