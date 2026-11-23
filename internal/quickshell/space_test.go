package quickshell

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ryangerardwilson/tesseract/internal/tmux"
)

// fakeQuickshell puts a quickshell on an otherwise empty PATH, so the real
// overlay can never start during a test. The script records its arguments
// and TESSERACT_SPACE_COMMAND, then exits with code.
func fakeQuickshell(t *testing.T, code string) (api *tmux.API, appDir string, recordPath string) {
	t.Helper()
	binDir := t.TempDir()
	recordPath = filepath.Join(t.TempDir(), "record")
	script := "#!/bin/sh\n" +
		"for arg in \"$@\"; do printf '%s\\n' \"$arg\"; done > \"$SPACE_TEST_RECORD\"\n" +
		"printf 'command=%s\\n' \"$TESSERACT_SPACE_COMMAND\" >> \"$SPACE_TEST_RECORD\"\n" +
		"exit " + code + "\n"
	if err := os.WriteFile(filepath.Join(binDir, "quickshell"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", binDir)
	appDir = t.TempDir()
	api = tmux.NewAPI(map[string]string{"HOME": t.TempDir(), "PATH": binDir, "SPACE_TEST_RECORD": recordPath})
	return api, appDir, recordPath
}

func TestOpenSpaceNeedsTheNavigatorFiles(t *testing.T) {
	api, appDir, recordPath := fakeQuickshell(t, "0")

	err := OpenSpace(api, appDir)

	if err == nil || !strings.Contains(err.Error(), "not installed") {
		t.Fatalf("err = %v, want the navigator to be missing", err)
	}
	if _, statErr := os.Stat(recordPath); !os.IsNotExist(statErr) {
		t.Fatal("quickshell must not start without the navigator files")
	}
}

func TestOpenSpaceStartsTheOverlayWithThisBinary(t *testing.T) {
	api, appDir, recordPath := fakeQuickshell(t, "0")
	navigatorDir := filepath.Join(appDir, "quickshell", "space-navigator")
	if err := os.MkdirAll(navigatorDir, 0o755); err != nil {
		t.Fatal(err)
	}

	if err := OpenSpace(api, appDir); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(recordPath)
	if err != nil {
		t.Fatal(err)
	}
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	want := "--path\n" + navigatorDir + "\ncommand=" + executable + "\n"
	if string(data) != want {
		t.Fatalf("quickshell saw:\n%s\nwant:\n%s", data, want)
	}
}

func TestOpenSpaceReportsAFailedOverlay(t *testing.T) {
	api, appDir, _ := fakeQuickshell(t, "1")
	if err := os.MkdirAll(filepath.Join(appDir, "quickshell", "space-navigator"), 0o755); err != nil {
		t.Fatal(err)
	}

	err := OpenSpace(api, appDir)

	if err == nil || !strings.Contains(err.Error(), "exited") {
		t.Fatalf("err = %v, want the overlay failure so the terminal navigator can take over", err)
	}
}
