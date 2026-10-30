//go:build windows

package downloaders

import (
	"os/exec"
	"syscall"
)

// prepareProcess puts the command in its own process group.
//
// On Unix that is a process group, and the whole tree can be signalled at
// once. Windows has the same idea — a job object — but it is not reachable
// through the standard library, and a job object needs a handle that has to
// be closed when the process ends.
//
// So on Windows the command is left as it is, and killing it kills the
// parent. The children of yt-dlp — ffmpeg, and whatever the extractor
// starts — are not in a group that can be signalled as one, so they are left
// to end with it.
//
// The consequence is worth knowing: stopping a recording on Windows kills
// yt-dlp, and a merge that was under way is not guaranteed to be stopped with
// it. The file is still put together, because the copy is what writes it and
// it ends when the pipe closes.
func prepareProcess(cmd *exec.Cmd) {
	// CREATE_NEW_PROCESS_GROUP, so that a console control event does not
	// reach the children either.
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP,
	}
}

// killProcessGroup stops a command and everything it started.
//
// There is no group to signal on Windows without a job object, so the process
// itself is killed. Its children are left to end when the pipe they write to
// closes, which is what ends the copy that writes the file.
func killProcessGroup(cmd *exec.Cmd) error {
	if cmd == nil || cmd.Process == nil {
		return nil
	}

	return cmd.Process.Kill()
}
