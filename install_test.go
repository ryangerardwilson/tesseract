package tesseract_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func writeExecutable(t *testing.T, path string, body string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
}

func runInstaller(t *testing.T, homeDir string, pathPrefix string, args ...string) string {
	t.Helper()
	installer, err := filepath.Abs("install.sh")
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("/usr/bin/bash", append([]string{installer}, args...)...)
	env := make([]string, 0, len(os.Environ())+3)
	for _, entry := range os.Environ() {
		if strings.HasPrefix(entry, "BASH_ENV=") || strings.HasPrefix(entry, "HOME=") || strings.HasPrefix(entry, "PATH=") {
			continue
		}
		env = append(env, entry)
	}
	env = append(env, "HOME="+homeDir)
	if os.Getenv("TM_INSTALL_ACTIVATE_SYSTEMD") == "" {
		env = append(env, "TM_INSTALL_ACTIVATE_SYSTEMD=0")
	}
	if pathPrefix != "" {
		env = append(env, "PATH="+pathPrefix+":/usr/bin:/bin")
	} else {
		env = append(env, "PATH=/usr/bin:/bin")
	}
	cmd.Env = env
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("installer failed: %v\n%s", err, out)
	}
	return string(out)
}

func TestInstallerVersionPrintsLatestRelease(t *testing.T) {
	tmp := t.TempDir()
	binDir := filepath.Join(tmp, "bin")
	homeDir := filepath.Join(tmp, "home")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(homeDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeExecutable(t, filepath.Join(binDir, "curl"), "#!/usr/bin/env bash\nif [[ \"$*\" == *\"releases/latest\"* ]]; then\n  printf 'https://github.com/ryangerardwilson/tesseract/releases/tag/v0.1.21\\n'\n  exit 0\nfi\necho unexpected curl call >&2\nexit 1\n")

	out := runInstaller(t, homeDir, binDir, "version")

	if strings.TrimSpace(out) != "0.1.21" {
		t.Fatalf("unexpected version output: %q", out)
	}
}

func TestInstallerLocalBinaryInstallWritesManagedFiles(t *testing.T) {
	tmp := t.TempDir()
	homeDir := filepath.Join(tmp, "home")
	if err := os.MkdirAll(homeDir, 0o755); err != nil {
		t.Fatal(err)
	}
	sourceBinary := filepath.Join(homeDir, "source-tesseract")
	writeExecutable(t, sourceBinary, `#!/usr/bin/env bash
set -euo pipefail
case "${1:-}" in
  version)
    printf '0.0.0\n'
    ;;
  __install-assets)
    mkdir -p "$2/quickshell/session-picker"
    printf 'picker\n' > "$2/quickshell/session-picker/shell.qml"
    mkdir -p "$2/tmux/tmux.conf.d" "$2/tmux/scripts"
    printf 'source-file ~/.config/tmux/tmux.conf.d/*.conf\n' > "$2/tmux/tmux.conf"
    ;;
  __render-tmux-config)
    mkdir -p "$(dirname "$2")"
    printf 'key=%s launcher=%s\n' "$3" "$4" > "$2"
    ;;
  *)
    printf 'tesseract:%s\n' "$*"
    ;;
esac
`)

	legacy := filepath.Join(homeDir, ".tm", "state")
	if err := os.MkdirAll(legacy, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(legacy, "kept"), []byte("row"), 0o644); err != nil {
		t.Fatal(err)
	}

	out := runInstaller(t, homeDir, "", "from", sourceBinary, "tmux", "key", "F8")

	internalBinary := filepath.Join(homeDir, ".tesseract", "bin", "tesseract")
	publicLauncher := filepath.Join(homeDir, ".local", "bin", "tesseract")
	configPath := filepath.Join(homeDir, ".config", "tmux", "tmux.conf")
	if _, err := os.Stat(internalBinary); err != nil {
		t.Fatalf("internal binary missing: %v", err)
	}
	kept, err := os.ReadFile(filepath.Join(homeDir, ".tesseract", "state", "kept"))
	if err != nil || string(kept) != "row" {
		t.Fatalf("legacy state was not moved: %v %q", err, kept)
	}
	if _, err := os.Stat(filepath.Join(homeDir, ".tm")); !os.IsNotExist(err) {
		t.Fatalf("legacy home remains: %v", err)
	}
	link, err := os.Readlink(filepath.Join(homeDir, ".local", "bin", "tm"))
	if err != nil || link != "tesseract" {
		t.Fatalf("compat launcher: %v %q", err, link)
	}
	kmLink, err := os.Readlink(filepath.Join(homeDir, ".local", "bin", "km"))
	if err != nil || kmLink != "tesseract" {
		t.Fatalf("km launcher: %v %q", err, kmLink)
	}
	musicaLink, err := os.Readlink(filepath.Join(homeDir, ".local", "bin", "musica"))
	if err != nil || musicaLink != "tesseract" {
		t.Fatalf("musica launcher: %v %q", err, musicaLink)
	}
	if _, err := os.Stat(publicLauncher); err != nil {
		t.Fatalf("public launcher missing: %v", err)
	}
	launcherText, err := os.ReadFile(publicLauncher)
	if err != nil || !strings.Contains(string(launcherText), `exec -a "$0"`) {
		t.Fatalf("launcher does not keep its invocation name: %v\n%s", err, launcherText)
	}
	if _, err := os.Stat(filepath.Join(homeDir, ".tesseract", "app", "quickshell", "session-picker", "shell.qml")); err != nil {
		t.Fatalf("asset missing: %v", err)
	}
	config, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(config), "key=F8") || !strings.Contains(string(config), publicLauncher) {
		t.Fatalf("unexpected tmux config:\n%s", config)
	}
	if !strings.Contains(out, "Manually add to ~/.bashrc if needed: export PATH="+filepath.Join(homeDir, ".local", "bin")+":$PATH") {
		t.Fatalf("missing manual PATH guidance:\n%s", out)
	}
}

func TestInstallerActivatesTmSnapshotTimer(t *testing.T) {
	tmp := t.TempDir()
	binDir := filepath.Join(tmp, "bin")
	homeDir := filepath.Join(tmp, "home")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(homeDir, 0o755); err != nil {
		t.Fatal(err)
	}
	logPath := filepath.Join(tmp, "systemctl.log")
	t.Setenv("TM_INSTALL_ACTIVATE_SYSTEMD", "1")
	t.Setenv("SYSTEMCTL_LOG", logPath)
	writeExecutable(t, filepath.Join(binDir, "systemctl"), `#!/usr/bin/env bash
set -euo pipefail
printf '%s\n' "$*" >> "$SYSTEMCTL_LOG"
`)

	sourceBinary := filepath.Join(homeDir, "source-tesseract")
	writeExecutable(t, sourceBinary, `#!/usr/bin/env bash
set -euo pipefail
case "${1:-}" in
  version)
    printf '0.0.0\n'
    ;;
  __install-assets)
    mkdir -p "$2/tmux/tmux.conf.d" "$2/tmux/scripts" "$2/quickshell/session-picker"
    printf 'source-file ~/.config/tmux/tmux.conf.d/*.conf\n' > "$2/tmux/tmux.conf"
    ;;
  __render-tmux-config)
    mkdir -p "$(dirname "$2")"
    : > "$2"
    ;;
  *)
    exit 0
    ;;
esac
`)

	_ = runInstaller(t, homeDir, binDir, "from", sourceBinary)

	log, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read systemctl log: %v", err)
	}
	commands := string(log)
	for _, want := range []string{
		"--user daemon-reload",
		"--user enable tesseract-restore.service tesseract-snapshot.timer",
		"--user reset-failed tesseract-restore.service tesseract-snapshot.service",
		"--user start tesseract-restore.service",
		"--user start tesseract-snapshot.timer",
	} {
		if !strings.Contains(commands, want) {
			t.Fatalf("missing systemctl command %q in:\n%s", want, commands)
		}
	}
	if !strings.Contains(commands, "disable --now tm-restore.service tm-snapshot.timer") {
		t.Fatalf("installer left the previous units enabled:\n%s", commands)
	}

	timer, err := os.ReadFile(filepath.Join(homeDir, ".config", "systemd", "user", "tesseract-snapshot.timer"))
	if err != nil {
		t.Fatalf("read snapshot timer: %v", err)
	}
	for _, want := range []string{"OnActiveSec=15min", "OnUnitActiveSec=15min", "WantedBy=timers.target"} {
		if !strings.Contains(string(timer), want) {
			t.Fatalf("snapshot timer missing %q:\n%s", want, timer)
		}
	}
	restore, err := os.ReadFile(filepath.Join(homeDir, ".config", "systemd", "user", "tesseract-restore.service"))
	if err != nil {
		t.Fatalf("read restore service: %v", err)
	}
	if !strings.Contains(string(restore), "After=graphical-session-pre.target") {
		t.Fatalf("restore service missing session PATH order:\n%s", restore)
	}
}
