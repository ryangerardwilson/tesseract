package assets

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWriteAndRenderTmuxConfig(t *testing.T) {
	target := t.TempDir()

	if err := Write(target); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(target, "quickshell", "session-picker", "shell.qml")); err != nil {
		t.Fatalf("picker asset missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(target, "tmux", "tmux.conf")); err != nil {
		t.Fatalf("tmux config asset missing: %v", err)
	}
	configPath := filepath.Join(target, "tmux.conf")
	if err := RenderTmuxConfig(configPath, "F8", "/tmp/tesseract"); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	if strings.Contains(text, "__TMUX_INDEX_KEY__") || strings.Contains(text, "__TM_PUBLIC_LAUNCHER__") {
		t.Fatalf("template placeholders were not replaced:\n%s", text)
	}
	if !strings.Contains(text, "source-file ~/.config/tmux/tmux.conf.d/*.conf") {
		t.Fatalf("rendered root config missing fragment loader:\n%s", text)
	}

	pickerData, err := os.ReadFile(filepath.Join(target, "tmux.conf.d", "20-managed-picker.conf"))
	if err != nil {
		t.Fatal(err)
	}
	pickerText := string(pickerData)
	if strings.Contains(pickerText, "__TMUX_INDEX_KEY__") || strings.Contains(pickerText, "__TM_PUBLIC_LAUNCHER__") {
		t.Fatalf("fragment placeholders were not replaced:\n%s", pickerText)
	}
	if !strings.Contains(pickerText, "F8") || !strings.Contains(pickerText, "/tmp/tesseract") {
		t.Fatalf("rendered picker fragment missing replacements:\n%s", pickerText)
	}

	for _, name := range []string{"even-horizontal-panes", "equalize-panes"} {
		helperInfo, err := os.Stat(filepath.Join(target, "scripts", name))
		if err != nil {
			t.Fatal(err)
		}
		if helperInfo.Mode().Perm() != 0o755 {
			t.Fatalf("%s mode = %o, want 755", name, helperInfo.Mode().Perm())
		}
	}
}
