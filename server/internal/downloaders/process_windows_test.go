//go:build windows

package downloaders

import (
	"os/exec"
	"syscall"
	"testing"
)

// Every yt-dlp invocation goes through here, so this is where "no console
// window" has to be true.
func TestPreparedProcessHasNoConsoleWindow(t *testing.T) {
	cmd := exec.Command("cmd", "/c", "echo", "hello")

	prepareProcess(cmd)

	if cmd.SysProcAttr == nil {
		t.Fatal("SysProcAttr is nil")
	}

	if !cmd.SysProcAttr.HideWindow {
		t.Error("HideWindow is not set, so yt-dlp would open a console")
	}

	const createNoWindow = 0x08000000

	if cmd.SysProcAttr.CreationFlags&createNoWindow == 0 {
		t.Errorf("CREATE_NO_WINDOW is not set: CreationFlags = %#x", cmd.SysProcAttr.CreationFlags)
	}

	// And the process group stays, because that is what stopping a download
	// uses.
	if cmd.SysProcAttr.CreationFlags&syscall.CREATE_NEW_PROCESS_GROUP == 0 {
		t.Errorf("the process group was lost: CreationFlags = %#x", cmd.SysProcAttr.CreationFlags)
	}
}
