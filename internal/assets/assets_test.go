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

func TestWriteIncludesTheSpaceNavigator(t *testing.T) {
	target := t.TempDir()

	if err := Write(target); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(filepath.Join(target, "quickshell", "space-navigator"))
	if err != nil {
		t.Fatalf("space navigator assets missing: %v", err)
	}
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		names = append(names, entry.Name())
	}
	// The navigator draws the workspaces as spheres and the windows as
	// electrons on shells, with SpaceSatellite; SpaceNavigator owns it and
	// shell.qml hosts it. It installs nothing else.
	want := "SpaceBody.qml SpaceCamera.qml SpaceGlow.qml SpaceNavigator.qml SpaceRing.qml SpaceSatellite.qml SpaceSphere.qml SpaceStage.qml shell.qml"
	if got := strings.Join(names, " "); got != want {
		t.Fatalf("space navigator assets = %s, want %s", got, want)
	}
	if _, err := os.Stat(filepath.Join("..", "..", "quickshell", "space-navigator", "SpacePlate.qml")); !os.IsNotExist(err) {
		t.Fatalf("quickshell/space-navigator/SpacePlate.qml must not exist: %v", err)
	}
}

// The navigator draws its spheres and rings with Qt Quick items and shapes
// through its own projection because Qt Quick 3D is not installed, and it
// shows window titles, which are untrusted.
func TestSpaceNavigatorNeedsNoQtQuick3DAndShowsPlainText(t *testing.T) {
	target := t.TempDir()

	if err := Write(target); err != nil {
		t.Fatal(err)
	}
	files, err := filepath.Glob(filepath.Join(target, "quickshell", "space-navigator", "*.qml"))
	if err != nil || len(files) == 0 {
		t.Fatalf("space navigator QML missing: %v", err)
	}
	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		source := string(data)
		if strings.Contains(source, "QtQuick3D") {
			t.Fatalf("%s uses Qt Quick 3D", filepath.Base(file))
		}
		texts := strings.Count(source, "Text {")
		plain := strings.Count(source, "textFormat: Text.PlainText")
		if texts != plain {
			t.Fatalf("%s has %d Text items but %d plain text formats", filepath.Base(file), texts, plain)
		}
	}
}

// One component owns the navigator, so every shell that hosts it shows the
// same picture: only SpaceNavigator draws the stage, the overlay hosts it,
// and the window and the process are left to the host.
func TestSpaceNavigatorIsTheOneOwnerTheOverlayHosts(t *testing.T) {
	target := t.TempDir()

	if err := Write(target); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(target, "quickshell", "space-navigator")
	files, err := filepath.Glob(filepath.Join(dir, "*.qml"))
	if err != nil || len(files) == 0 {
		t.Fatalf("space navigator QML missing: %v", err)
	}
	stages := map[string]int{}
	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		if count := strings.Count(string(data), "SpaceStage {"); count > 0 {
			stages[filepath.Base(file)] = count
		}
	}
	if len(stages) != 1 || stages["SpaceNavigator.qml"] != 1 {
		t.Fatalf("stages drawn = %v, want one, in SpaceNavigator.qml", stages)
	}
	shell, err := os.ReadFile(filepath.Join(dir, "shell.qml"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(shell), "SpaceNavigator {") {
		t.Fatal("shell.qml does not host SpaceNavigator")
	}
	navigator, err := os.ReadFile(filepath.Join(dir, "SpaceNavigator.qml"))
	if err != nil {
		t.Fatal(err)
	}
	for _, hostOnly := range []string{"Qt.quit()", "PanelWindow", "Quickshell.env"} {
		if strings.Contains(string(navigator), hostOnly) {
			t.Fatalf("SpaceNavigator.qml uses %s, which belongs to its host", hostOnly)
		}
	}
}
