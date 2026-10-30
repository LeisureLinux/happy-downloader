//go:build windows

package tools

import (
	"os/exec"
	"syscall"
)

// CREATE_NO_WINDOW, from the Windows process creation flags.
//
// A program built as a windowed application has no console, but that is a
// property of the program, not of the processes it starts. A child that is a
// console program — yt-dlp is one, and so are ffmpeg and ffprobe — is given a
// console of its own unless it is told not to have one, and Windows gives it a
// window to go with it.
const createNoWindow = 0x08000000

// Quiet keeps a child process from opening a console window.
//
// Without this, every invocation of yt-dlp flashes a black window across the
// screen: once when the version is read at startup, once when a URL is
// analysed, once more per download. They are short-lived windows that steal
// the focus, and the only thing a user can do about them is minimise
// something that is already gone.
//
// HideWindow alone is not enough: it hides the window of a console that is
// still created. CREATE_NO_WINDOW stops the console existing at all.
func Quiet(cmd *exec.Cmd) {
	if cmd == nil {
		return
	}

	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}

	cmd.SysProcAttr.HideWindow = true
	// Or-ed rather than assigned: the caller may have asked for its own
	// process group, and losing that would break stopping a download.
	cmd.SysProcAttr.CreationFlags |= createNoWindow
}
