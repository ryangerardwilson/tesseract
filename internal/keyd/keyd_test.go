package keyd

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestNoArgsMatchesHelp(t *testing.T) {
	noArgs := runCLI(t, nil, nil, nil)
	helpArgs := runCLI(t, []string{"help"}, nil, nil)

	if noArgs.code != 0 {
		t.Fatalf("no args exit = %d, stderr = %s", noArgs.code, noArgs.stderr)
	}
	if noArgs.stdout != helpArgs.stdout {
		t.Fatalf("no args help did not match help command")
	}
}

func TestEnsureConfigSeedsXDGTarget(t *testing.T) {
	configHome := t.TempDir()
	result := runCLI(t, []string{"config"}, map[string]string{
		"XDG_CONFIG_HOME": configHome,
		"EDITOR":          "/usr/bin/true",
	}, nil)

	if result.code != 0 {
		t.Fatalf("exit = %d, stderr = %s", result.code, result.stderr)
	}
	target := filepath.Join(configHome, "km", "keyd.config")
	if _, err := os.Stat(target); err != nil {
		t.Fatalf("expected config target: %v", err)
	}
}

func TestEnsureConfigMigratesLegacyTarget(t *testing.T) {
	configHome := t.TempDir()
	legacyDir := filepath.Join(configHome, legacyAppName)
	if err := os.MkdirAll(legacyDir, 0o755); err != nil {
		t.Fatal(err)
	}
	legacyConfig := filepath.Join(legacyDir, "keyd.config")
	if err := os.WriteFile(legacyConfig, []byte("[ids]\n*\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	result := runCLI(t, []string{"config"}, map[string]string{
		"XDG_CONFIG_HOME": configHome,
		"EDITOR":          "/usr/bin/true",
	}, nil)

	if result.code != 0 {
		t.Fatalf("exit = %d, stderr = %s", result.code, result.stderr)
	}
	target := filepath.Join(configHome, "km", "keyd.config")
	content, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != "[ids]\n*\n" {
		t.Fatalf("migrated content = %q", content)
	}
}

func TestApplyRetriesReloadAfterSocketError(t *testing.T) {
	configHome := t.TempDir()
	target := filepath.Join(configHome, "km", "keyd.config")
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, []byte("[main]\ncontrol = oneshot(control)\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	exe := &recordingExecutor{
		lookPath: map[string]string{"pacman": "/usr/bin/pacman"},
		runResults: []CommandResult{
			{Code: 0},
			{Code: 1, Stderr: "failed to connect to /var/run/keyd.socket"},
			{Code: 0},
			{Code: 0},
		},
	}

	result := runCLIWithOptions(t, []string{"apply"}, map[string]string{"XDG_CONFIG_HOME": configHome}, exe, func(r *Runner) {
		r.EUID = 0
		r.SocketExists = func(string) bool { return true }
		r.Sleep = func(time.Duration) {}
	})

	if result.code != 0 {
		t.Fatalf("exit = %d, stdout = %s, stderr = %s", result.code, result.stdout, result.stderr)
	}
	want := [][]string{
		{"pacman", "-Q", "keyd"},
		{"install", "-Dm644", target, systemConfig},
		{"systemctl", "enable", "--now", "keyd.service"},
		{"keyd", "reload"},
		{"systemctl", "restart", "keyd.service"},
		{"keyd", "reload"},
	}
	if !reflect.DeepEqual(exe.commands, want) {
		t.Fatalf("commands = %#v, want %#v", exe.commands, want)
	}
}

type cliResult struct {
	stdout string
	stderr string
	code   int
}

func runCLI(t *testing.T, args []string, env map[string]string, exe Executor) cliResult {
	t.Helper()
	return runCLIWithOptions(t, args, env, exe, nil)
}

func runCLIWithOptions(t *testing.T, args []string, env map[string]string, exe Executor, configure func(*Runner)) cliResult {
	t.Helper()
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	merged := envMap(os.Environ())
	for key, value := range env {
		merged[key] = value
	}
	if exe == nil {
		exe = shellExecutor{}
	}
	runner := Runner{
		Out:          &stdout,
		Err:          &stderr,
		In:           strings.NewReader(""),
		Exe:          exe,
		Env:          merged,
		EUID:         os.Geteuid(),
		SocketExists: fileExists,
		Sleep:        time.Sleep,
	}
	if configure != nil {
		configure(&runner)
	}
	code := runner.Run(args)
	return cliResult{stdout: stdout.String(), stderr: stderr.String(), code: code}
}

type recordingExecutor struct {
	lookPath     map[string]string
	runResults   []CommandResult
	commands     [][]string
	attachedArgs []string
}

func (e *recordingExecutor) LookPath(name string, pathValue string) (string, error) {
	if path, ok := e.lookPath[name]; ok {
		return path, nil
	}
	return lookPath(name, pathValue)
}

func (e *recordingExecutor) Run(args []string, env map[string]string) CommandResult {
	e.commands = append(e.commands, append([]string{}, args...))
	if len(e.runResults) == 0 {
		return CommandResult{}
	}
	result := e.runResults[0]
	e.runResults = e.runResults[1:]
	return result
}

func (e *recordingExecutor) RunAttached(args []string, out io.Writer, errOut io.Writer, env map[string]string) int {
	e.commands = append(e.commands, append([]string{}, args...))
	e.attachedArgs = append([]string{}, args...)
	return 0
}
