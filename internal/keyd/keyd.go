package keyd

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const (
	appName       = "km"
	legacyAppName = "keyd_manager"
	systemConfig  = "/etc/keyd/sticky_keys.conf"
	keydSocket    = "/var/run/keyd.socket"
	defaultConfig = `[ids]
*

[main]
control = oneshot(control)
shift = oneshot(shift)
alt = oneshot(alt)
meta = oneshot(meta)
`
)

const HelpText = `km
edit, install, and inspect the managed keyd sticky-keys config

global actions:
  km help
  tesseract km help
    show this help
  km version
  tesseract km version
    print the tesseract version
  km upgrade
  tesseract km upgrade
    upgrade tesseract

features:
  open the managed keyd config in your editor
  # km config
  km config
  tesseract km config

  install the managed config into /etc/keyd and reload the service
  # km apply
  km apply
  tesseract km apply

  inspect the current keyd service status
  # km status
  km status
  tesseract km status
`

func WriteHelp(out io.Writer) {
	_, _ = fmt.Fprint(out, HelpText)
}

type CommandResult struct {
	Stdout string
	Stderr string
	Code   int
}

type Executor interface {
	LookPath(name string, pathValue string) (string, error)
	Run(args []string, env map[string]string) CommandResult
	RunAttached(args []string, out io.Writer, errOut io.Writer, env map[string]string) int
}

type shellExecutor struct{}

func (shellExecutor) LookPath(name string, pathValue string) (string, error) {
	return lookPath(name, pathValue)
}

func (shellExecutor) Run(args []string, env map[string]string) CommandResult {
	if len(args) == 0 {
		return CommandResult{Code: 1, Stderr: "empty command"}
	}
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Env = mergedEnv(env)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	code := 0
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			code = exitErr.ExitCode()
		} else {
			code = 127
			if stderr.Len() == 0 {
				_, _ = stderr.WriteString(err.Error())
			}
		}
	}
	return CommandResult{Stdout: stdout.String(), Stderr: stderr.String(), Code: code}
}

func (shellExecutor) RunAttached(args []string, out io.Writer, errOut io.Writer, env map[string]string) int {
	if len(args) == 0 {
		_, _ = fmt.Fprintln(errOut, "empty command")
		return 1
	}
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Env = mergedEnv(env)
	cmd.Stdout = out
	cmd.Stderr = errOut
	cmd.Stdin = os.Stdin
	if err := cmd.Run(); err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return exitErr.ExitCode()
		}
		_, _ = fmt.Fprintln(errOut, err)
		return 1
	}
	return 0
}

type Runner struct {
	Out          io.Writer
	Err          io.Writer
	In           io.Reader
	Exe          Executor
	Env          map[string]string
	EUID         int
	SocketExists func(string) bool
	Sleep        func(time.Duration)
}

func NewRunner(out io.Writer, errOut io.Writer, in io.Reader) Runner {
	return Runner{
		Out:          out,
		Err:          errOut,
		In:           in,
		Exe:          shellExecutor{},
		Env:          envMap(os.Environ()),
		EUID:         os.Geteuid(),
		SocketExists: fileExists,
		Sleep:        time.Sleep,
	}
}

func (r Runner) Run(args []string) int {
	if len(args) == 0 || (len(args) == 1 && args[0] == "help") {
		WriteHelp(r.Out)
		return 0
	}
	if len(args) == 1 {
		switch args[0] {
		case "config":
			return r.editConfig()
		case "apply":
			return r.applyConfig()
		case "status":
			return r.showStatus()
		}
	}
	WriteHelp(r.Out)
	return 1
}

func (r Runner) editConfig() int {
	target, err := r.ensureConfigFile()
	if err != nil {
		_, _ = fmt.Fprintln(r.Err, err)
		return 1
	}
	return r.Exe.RunAttached([]string{r.resolveEditor(), target}, r.Out, r.Err, r.Env)
}

func (r Runner) applyConfig() int {
	source, err := r.ensureConfigFile()
	if err != nil {
		_, _ = fmt.Fprintln(r.Err, err)
		return 1
	}

	if rc := r.ensureKeydInstalled(); rc != 0 {
		return rc
	}

	install := r.runRoot([]string{"install", "-Dm644", source, systemConfig}, false)
	if install.Code != 0 {
		return install.Code
	}

	enable := r.runRoot([]string{"systemctl", "enable", "--now", "keyd.service"}, false)
	if enable.Code != 0 {
		return enable.Code
	}

	r.waitForSocket()

	reload := r.runRoot([]string{"keyd", "reload"}, true)
	if reload.Code != 0 {
		combined := combinedOutput(reload)
		restart := r.runRoot([]string{"systemctl", "restart", "keyd.service"}, true)
		if restart.Code != 0 {
			writeIfPresent(r.Err, combined)
			writeIfPresent(r.Err, combinedOutput(restart))
			return restart.Code
		}

		r.waitForSocket()
		reload = r.runRoot([]string{"keyd", "reload"}, true)
		if reload.Code != 0 {
			writeIfPresent(r.Err, combinedOutput(reload))
			return reload.Code
		}
	}

	_, _ = fmt.Fprintf(r.Out, "Installed %s -> %s\n", source, systemConfig)
	_, _ = fmt.Fprintln(r.Out, "keyd config applied.")
	return 0
}

func (r Runner) showStatus() int {
	return r.Exe.RunAttached([]string{"systemctl", "status", "keyd.service", "--no-pager", "--lines=20"}, r.Out, r.Err, r.Env)
}

func (r Runner) ensureKeydInstalled() int {
	if _, err := r.Exe.LookPath("pacman", r.env("PATH")); err != nil {
		_, _ = fmt.Fprintln(r.Err, "This flow expects Arch Linux with pacman.")
		return 1
	}

	probe := r.Exe.Run([]string{"pacman", "-Q", "keyd"}, r.Env)
	if probe.Code == 0 {
		return 0
	}
	install := r.runRoot([]string{"pacman", "-S", "--needed", "--noconfirm", "keyd"}, false)
	return install.Code
}

func (r Runner) runRoot(command []string, captureOutput bool) CommandResult {
	args := append([]string{}, command...)
	if r.EUID != 0 {
		args = append([]string{"sudo"}, args...)
	}
	if captureOutput {
		return r.Exe.Run(args, r.Env)
	}
	code := r.Exe.RunAttached(args, r.Out, r.Err, r.Env)
	return CommandResult{Code: code}
}

func (r Runner) waitForSocket() {
	for count := 0; count < 10; count++ {
		if r.SocketExists(keydSocket) {
			return
		}
		r.Sleep(200 * time.Millisecond)
	}
}

func (r Runner) ensureConfigFile() (string, error) {
	target := r.configPath()
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return "", err
	}
	if fileExists(target) {
		return target, nil
	}

	legacy := r.legacyConfigPath()
	if fileExists(legacy) {
		content, err := os.ReadFile(legacy)
		if err != nil {
			return "", err
		}
		return target, os.WriteFile(target, content, 0o644)
	}
	return target, os.WriteFile(target, []byte(defaultConfig), 0o644)
}

func (r Runner) configPath() string {
	return filepath.Join(r.configBase(), appName, "keyd.config")
}

func (r Runner) legacyConfigPath() string {
	return filepath.Join(r.configBase(), legacyAppName, "keyd.config")
}

func (r Runner) configBase() string {
	if value := r.env("XDG_CONFIG_HOME"); value != "" {
		return value
	}
	return filepath.Join(r.homeDir(), ".config")
}

func (r Runner) homeDir() string {
	if value := r.env("HOME"); value != "" {
		return value
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return home
}

func (r Runner) resolveEditor() string {
	if value := r.env("VISUAL"); value != "" {
		return value
	}
	if value := r.env("EDITOR"); value != "" {
		return value
	}
	return "vim"
}

func (r Runner) env(key string) string {
	if r.Env != nil {
		return r.Env[key]
	}
	return os.Getenv(key)
}

func combinedOutput(result CommandResult) string {
	parts := []string{}
	for _, part := range []string{result.Stdout, result.Stderr} {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			parts = append(parts, trimmed)
		}
	}
	return strings.Join(parts, "\n")
}

func writeIfPresent(out io.Writer, value string) {
	if value != "" {
		_, _ = fmt.Fprintln(out, value)
	}
}

func envMap(values []string) map[string]string {
	result := map[string]string{}
	for _, value := range values {
		key, val, ok := strings.Cut(value, "=")
		if !ok {
			continue
		}
		result[key] = val
	}
	return result
}

func mergedEnv(overrides map[string]string) []string {
	merged := envMap(os.Environ())
	for key, value := range overrides {
		merged[key] = value
	}
	result := make([]string, 0, len(merged))
	for key, value := range merged {
		result = append(result, key+"="+value)
	}
	sort.Strings(result)
	return result
}

func lookPath(name string, pathValue string) (string, error) {
	if strings.ContainsRune(name, os.PathSeparator) {
		if isExecutable(name) {
			return name, nil
		}
		return "", os.ErrNotExist
	}
	for _, dir := range filepath.SplitList(pathValue) {
		if dir == "" {
			dir = "."
		}
		candidate := filepath.Join(dir, name)
		if isExecutable(candidate) {
			return candidate, nil
		}
	}
	return "", exec.ErrNotFound
}

func isExecutable(path string) bool {
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		return false
	}
	return info.Mode().Perm()&0o111 != 0
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
