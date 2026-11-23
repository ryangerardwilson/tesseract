package quickshell

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/ryangerardwilson/tesseract/internal/assets"
	"github.com/ryangerardwilson/tesseract/internal/tmux"
)

// OpenSpace runs the space navigator overlay and waits for it to close. The
// overlay lists and focuses through this binary's space commands, named by
// TESSERACT_SPACE_COMMAND, and never runs hyprctl itself.
func OpenSpace(api *tmux.API, appDir string) error {
	if appDir == "" {
		appDir = assets.DefaultAppDir(api.Env["HOME"])
	}
	navigatorDir := filepath.Join(appDir, "quickshell", "space-navigator")
	if info, err := os.Stat(navigatorDir); err != nil || !info.IsDir() {
		return fmt.Errorf("tesseract space navigator is not installed")
	}
	command, err := pickerCommand(navigatorDir)
	if err != nil {
		return err
	}
	executable, err := os.Executable()
	if err != nil || executable == "" {
		executable = "tesseract"
	}
	cmd := exec.Command(command[0], command[1:]...)
	cmd.Env = append(apiEnvList(api), "TESSERACT_SPACE_COMMAND="+executable)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("tesseract space navigator exited: %w", err)
	}
	return nil
}
