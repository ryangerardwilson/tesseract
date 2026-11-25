package cli

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
)

// startCenter starts the launcher script in its own session and does not
// wait for it: the launcher stays up after musica returns. Tests replace it.
var startCenter = func(script string) error {
	cmd := exec.Command(script, "musica")
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := cmd.Start(); err != nil {
		return err
	}
	return cmd.Process.Release()
}

// Command is the musica binary. It runs the same commands as player.Run.
func Command(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	return Main(args, stdin, stdout, stderr)
}

// centerScript is the launcher script that shows the Tesseract Musica
// center: <config>/quickshell/omarchy-launcher/open.sh, <config> being
// XDG_CONFIG_HOME when it is absolute and ~/.config otherwise.
func centerScript() (string, error) {
	base := os.Getenv("XDG_CONFIG_HOME")
	if !filepath.IsAbs(base) {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		base = filepath.Join(home, ".config")
	}
	return filepath.Join(base, "quickshell", "omarchy-launcher", "open.sh"), nil
}

// openCenter shows the saved library in the Tesseract Musica center. The
// launcher starts musica serve when no session is running.
func openCenter(stderr io.Writer) int {
	script, err := centerScript()
	if err == nil {
		err = startCenter(script)
	}
	if err != nil {
		fmt.Fprintf(stderr, "The Tesseract Musica center opens with %s musica: %v\n", script, err)
		return 1
	}
	return 0
}
