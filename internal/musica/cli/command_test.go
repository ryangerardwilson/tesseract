package cli

import (
	"bytes"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ryangerardwilson/tesseract/internal/musica/config"
)

func noPlayer(t *testing.T) func() {
	t.Helper()
	restoreRun := SetRunAppForTest(func(config.AppConfig) int {
		t.Fatal("bare musica must not open the terminal player")
		return 1
	})
	previous := serveApp
	serveApp = func(config.AppConfig) int {
		t.Fatal("bare musica must not serve the player itself")
		return 1
	}
	return func() {
		restoreRun()
		serveApp = previous
	}
}

func TestBareMusicaOpensTheTesseractCenter(t *testing.T) {
	defer noPlayer(t)()
	t.Setenv("XDG_CONFIG_HOME", "/xdg")
	previous := startCenter
	defer func() { startCenter = previous }()
	var started []string
	startCenter = func(script string) error {
		started = append(started, script)
		return nil
	}

	var stdout, stderr bytes.Buffer
	if code := Command(nil, strings.NewReader(""), &stdout, &stderr); code != 0 {
		t.Fatalf("code %d %s", code, stderr.String())
	}
	if code := Main(nil, strings.NewReader(""), &stdout, &stderr); code != 0 {
		t.Fatalf("player.Run path: code %d %s", code, stderr.String())
	}
	want := filepath.Join("/xdg", "quickshell", "omarchy-launcher", "open.sh")
	if len(started) != 2 || started[0] != want || started[1] != want {
		t.Fatalf("started %q, want %q twice", started, want)
	}
}

func TestBareMusicaUsesHomeConfigWithoutAnAbsoluteXDG(t *testing.T) {
	defer noPlayer(t)()
	t.Setenv("XDG_CONFIG_HOME", "relative")
	t.Setenv("HOME", "/home/someone")
	previous := startCenter
	defer func() { startCenter = previous }()
	got := ""
	startCenter = func(script string) error {
		got = script
		return nil
	}
	var stdout, stderr bytes.Buffer
	if code := Command(nil, strings.NewReader(""), &stdout, &stderr); code != 0 || got != "/home/someone/.config/quickshell/omarchy-launcher/open.sh" {
		t.Fatalf("code %d script %q %s", code, got, stderr.String())
	}
}

func TestBareMusicaNamesTheLauncherWhenItCannotStart(t *testing.T) {
	defer noPlayer(t)()
	t.Setenv("XDG_CONFIG_HOME", "/xdg")
	previous := startCenter
	defer func() { startCenter = previous }()
	startCenter = func(string) error { return errors.New("no such file or directory") }

	var stdout, stderr bytes.Buffer
	if code := Command(nil, strings.NewReader(""), &stdout, &stderr); code != 1 || !strings.Contains(stderr.String(), "/xdg/quickshell/omarchy-launcher/open.sh musica") {
		t.Fatalf("code %d %s", code, stderr.String())
	}
}

func TestTheRealStarterReportsAMissingLauncher(t *testing.T) {
	if err := startCenter(filepath.Join(t.TempDir(), "open.sh")); err == nil {
		t.Fatal("starting a missing launcher script succeeded")
	}
}

func TestMusicaWithArgumentsStaysInTheBinary(t *testing.T) {
	previous := startCenter
	defer func() { startCenter = previous }()
	startCenter = func(string) error {
		t.Fatal("a command with arguments must not open the center")
		return nil
	}
	var stdout, stderr bytes.Buffer
	if code := Command([]string{"help"}, strings.NewReader(""), &stdout, &stderr); code != 0 || !strings.Contains(stdout.String(), "r2d2 musica") || !strings.Contains(stdout.String(), "musica serve") {
		t.Fatalf("code %d %s", code, stdout.String())
	}
}
