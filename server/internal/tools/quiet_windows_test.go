//go:build windows

package tools

import (
	"os/exec"
	"testing"
)

// A child process must not get a console window.
//
// This is the difference between a downloader that is used and one that is
// closed again: every invocation of yt-dlp, ffmpeg or ffprobe is a console
// program, and a windowed application that starts one without saying otherwise
// gives it a console and a window of its own.
func TestQuietKeepsTheConsoleWindowAway(t *testing.T) {
	cmd := exec.Command("cmd", "/c", "echo", "hello")

	Quiet(cmd)

	if cmd.SysProcAttr == nil {
		t.Fatal("Quiet left SysProcAttr nil")
	}

	if !cmd.SysProcAttr.HideWindow {
		t.Error("HideWindow is not set")
	}

	if cmd.SysProcAttr.CreationFlags&createNoWindow == 0 {
		t.Errorf("CREATE_NO_WINDOW is not set: CreationFlags = %#x", cmd.SysProcAttr.CreationFlags)
	}
}

// Flags the caller asked for are kept: a download is stopped by its process
// group, and quietening the child must not take that away.
func TestQuietKeepsTheProcessGroup(t *testing.T) {
	cmd := exec.Command("cmd", "/c", "echo", "hello")
	cmd.SysProcAttr = newSysProcAttr(newGroupFlag())

	Quiet(cmd)

	if cmd.SysProcAttr.CreationFlags&newGroupFlag() == 0 {
		t.Errorf("the process group flag was lost: CreationFlags = %#x", cmd.SysProcAttr.CreationFlags)
	}
}
