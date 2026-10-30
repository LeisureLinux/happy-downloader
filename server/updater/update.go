package updater

import (
	"os/exec"

	"github.com/LeisureLinux/happy-downloader/v4/server/internal/tools"
)

// Update using the builtin function of yt-dlp
func UpdateExecutable() error {
	cmd := exec.Command(tools.YtDlp(), "-U")
	tools.Quiet(cmd)

	err := cmd.Start()
	if err != nil {
		return err
	}

	return cmd.Wait()
}
