//go:build !windows

package downloaders

import (
	"os/exec"
	"syscall"
)

// prepareProcess puts the command in its own process group.
//
// yt-dlp uses child processes — ffmpeg for merging, and whatever the
// extractor needs — and killing the parent does not always kill them. A
// recording that was stopped left its children writing into a file nobody
// pointed at any more.
//
// Its own group is what makes the whole tree killable at once.
func prepareProcess(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

// killProcessGroup stops a command and everything it started.
//
// The group is signalled rather than the process, so the children go with it.
// The signal is SIGTERM first and SIGKILL only if it is still there, because
// a recording that is being put into its container needs a moment to finish
// writing.
func killProcessGroup(cmd *exec.Cmd) error {
	if cmd == nil || cmd.Process == nil {
		return nil
	}

	pgid, err := syscall.Getpgid(cmd.Process.Pid)
	if err != nil {
		// The process is already gone, which is the outcome that was wanted.
		return cmd.Process.Kill()
	}

	if err := syscall.Kill(-pgid, syscall.SIGTERM); err != nil {
		return cmd.Process.Kill()
	}

	return nil
}
