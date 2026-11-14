package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ryangerardwilson/tesseract/internal/tmux"
)

func TestRunHelpDoesNotNeedTmux(t *testing.T) {
	var out bytes.Buffer
	var errOut bytes.Buffer

	code := Run([]string{"help"}, tmux.NewAPI(map[string]string{"HOME": t.TempDir()}), &out, &errOut)

	if code != 0 {
		t.Fatalf("unexpected code: %d stderr=%s", code, errOut.String())
	}
	if !strings.Contains(out.String(), "tesseract open picker") {
		t.Fatalf("help missing command list:\n%s", out.String())
	}
}

func TestRunVersionDoesNotNeedTmux(t *testing.T) {
	var out bytes.Buffer
	var errOut bytes.Buffer

	code := Run([]string{"version"}, tmux.NewAPI(map[string]string{"HOME": t.TempDir()}), &out, &errOut)

	if code != 0 {
		t.Fatalf("unexpected code: %d stderr=%s", code, errOut.String())
	}
	if strings.TrimSpace(out.String()) != "0.0.0" {
		t.Fatalf("unexpected version output: %q", out.String())
	}
}

func TestRunRejectsRemovedBareCommands(t *testing.T) {
	var out bytes.Buffer
	var errOut bytes.Buffer

	code := Run([]string{"restore"}, tmux.NewAPI(map[string]string{"HOME": t.TempDir()}), &out, &errOut)

	if code == 0 {
		t.Fatal("expected invalid command to fail")
	}
	if !strings.Contains(errOut.String(), "valid shape") {
		t.Fatalf("unexpected stderr: %s", errOut.String())
	}
}

func TestRunKMConfigSeedsFile(t *testing.T) {
	configHome := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", configHome)
	t.Setenv("EDITOR", "/usr/bin/true")
	var out bytes.Buffer
	var errOut bytes.Buffer

	code := Run([]string{"km", "config"}, tmux.NewAPI(map[string]string{"HOME": t.TempDir()}), &out, &errOut)

	if code != 0 {
		t.Fatalf("unexpected code: %d stderr=%s", code, errOut.String())
	}
	target := filepath.Join(configHome, "km", "keyd.config")
	if _, err := os.Stat(target); err != nil {
		t.Fatalf("expected config: %v", err)
	}
}

func TestRunInvokedAsKMPrintsTesseractVersion(t *testing.T) {
	var out bytes.Buffer
	var errOut bytes.Buffer

	code := RunInvoked("/home/ryan/.local/bin/km", []string{"version"}, tmux.NewAPI(map[string]string{"HOME": t.TempDir()}), &out, &errOut)

	if code != 0 {
		t.Fatalf("unexpected code: %d stderr=%s", code, errOut.String())
	}
	if strings.TrimSpace(out.String()) != "0.0.0" {
		t.Fatalf("unexpected version output: %q", out.String())
	}
}

func TestRunKMHelpDoesNotPrepareTmux(t *testing.T) {
	var out bytes.Buffer
	var errOut bytes.Buffer

	code := Run([]string{"km"}, tmux.NewAPI(map[string]string{"HOME": t.TempDir()}), &out, &errOut)

	if code != 0 {
		t.Fatalf("unexpected code: %d stderr=%s", code, errOut.String())
	}
	if !strings.Contains(out.String(), "tesseract km config") {
		t.Fatalf("help missing keyd command:\n%s", out.String())
	}
}
