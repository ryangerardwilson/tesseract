package cli

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/ryangerardwilson/tesseract/internal/musica/app"
	"github.com/ryangerardwilson/tesseract/internal/musica/config"
	"github.com/ryangerardwilson/tesseract/internal/musica/download"
	"github.com/ryangerardwilson/tesseract/internal/musica/session"
	"github.com/ryangerardwilson/tesseract/internal/musica/store"
	"github.com/ryangerardwilson/tesseract/internal/version"
)

const installScriptURL = "https://raw.githubusercontent.com/ryangerardwilson/tesseract/main/install.sh"

var runApp = app.Run
var serveApp = app.Serve
var endSession = session.End
var reloadSession = session.Reload

func Main(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	switch {
	case sameArgs(args, "help"):
		fmt.Fprint(stdout, HelpText)
		return 0
	case sameArgs(args, "version"):
		fmt.Fprintln(stdout, version.Version)
		return 0
	case sameArgs(args, "upgrade"):
		return upgrade(stdout, stderr)
	case sameArgs(args, "download-worker"):
		if err := download.RunWorker(); err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		return 0
	case sameArgs(args, "end"):
		if err := endSession(); err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		fmt.Fprintln(stdout, "musica ended")
		return 0
	case sameArgs(args, "reload"):
		if err := reloadSession(); err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		fmt.Fprintln(stdout, "musica reloaded")
		return 0
	case len(args) > 0 && isGlobalAction(args[0]):
		fmt.Fprintln(stderr, "Use help, version, upgrade, end, or reload by itself.")
		return 1
	default:
		return dispatch(args, stdin, stdout, stderr)
	}
}

func dispatch(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	switch {
	case len(args) == 0:
		return openCenter(stderr)
	case sameArgs(args, "serve"):
		return runLibrary("", "", serveApp, stdin, stdout, stderr)
	case sameArgs(args, "config"):
		return openConfig(stderr)
	case len(args) == 2 && args[0] == "play":
		track := config.AbsPath(args[1])
		info, err := os.Stat(track)
		if err != nil || info.IsDir() {
			fmt.Fprintf(stderr, "Track not found: %s\n", track)
			return 1
		}
		return runLibrary(filepath.Dir(track), track, runApp, stdin, stdout, stderr)
	case len(args) == 2 && args[0] == "open":
		musicDir := config.AbsPath(args[1])
		info, err := os.Stat(musicDir)
		if err != nil || !info.IsDir() {
			fmt.Fprintf(stderr, "Music directory not found: %s\n", musicDir)
			return 1
		}
		return runLibrary(musicDir, "", runApp, stdin, stdout, stderr)
	default:
		fmt.Fprintln(stderr, Usage)
		return 1
	}
}

// runLibrary builds the config for the saved library, or for targetMusicDir
// this run, and runs it: the terminal TUI or musica serve.
func runLibrary(targetMusicDir, startupTrack string, run func(config.AppConfig) int, stdin io.Reader, stdout, stderr io.Writer) int {
	configPath := config.DefaultConfigPath()
	if err := config.AdoptLegacyConfig(configPath); err != nil {
		fmt.Fprintf(stderr, "%v\n", err)
		return 1
	}
	if targetMusicDir == "" {
		if err := bootstrapFirstRunConfig(configPath, "", stdin, stdout); err != nil {
			fmt.Fprintf(stderr, "%v\n", err)
			return 1
		}
	}
	appConfig := config.BuildAppConfig(targetMusicDir, nil, configPath)
	appConfig.StartupTrack = startupTrack
	_ = store.RememberMusicDir(appConfig.MusicDir)
	return run(appConfig)
}

func bootstrapFirstRunConfig(configPath, cliMusicDir string, stdin io.Reader, stdout io.Writer) error {
	userCfg := config.LoadUserConfig(configPath)
	if userCfg.MusicDir != "" {
		return nil
	}
	if dir, ok := store.SavedMusicDir(); ok {
		return config.SaveUserConfig(configPath, config.UserConfig{MusicDir: dir})
	}
	if cliMusicDir != "" {
		return config.SaveUserConfig(configPath, config.UserConfig{MusicDir: config.AbsPath(cliMusicDir)})
	}
	defaultDir := config.AbsPath("~/Music")
	if !readerIsTerminal(stdin) {
		return config.SaveUserConfig(configPath, config.UserConfig{MusicDir: defaultDir})
	}
	fmt.Fprintln(stdout, "First run setup: choose your music directory.")
	fmt.Fprintf(stdout, "Press Enter to use default: %s\n", defaultDir)
	fmt.Fprint(stdout, "Music directory: ")
	raw, _ := bufio.NewReader(stdin).ReadString('\n')
	chosen := strings.TrimSpace(raw)
	if chosen == "" {
		chosen = defaultDir
	}
	return config.SaveUserConfig(configPath, config.UserConfig{MusicDir: config.AbsPath(chosen)})
}

func openConfig(stderr io.Writer) int {
	path := config.DefaultConfigPath()
	if err := config.AdoptLegacyConfig(path); err != nil {
		fmt.Fprintf(stderr, "%v\n", err)
		return 1
	}
	if err := config.EnsureConfigFile(path); err != nil {
		fmt.Fprintf(stderr, "%v\n", err)
		return 1
	}
	editor := os.Getenv("VISUAL")
	if editor == "" {
		editor = os.Getenv("EDITOR")
	}
	if editor == "" {
		editor = "vim"
	}
	cmd := exec.Command(editor, path)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return exitErr.ExitCode()
		}
		fmt.Fprintf(stderr, "%v\n", err)
		return 1
	}
	return 0
}

func upgrade(stdout, stderr io.Writer) int {
	cmd := exec.Command("bash", "-c", "curl -fsSL "+installScriptURL+" | bash -s -- upgrade")
	cmd.Stdin = os.Stdin
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	if err := cmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return exitErr.ExitCode()
		}
		fmt.Fprintf(stderr, "%v\n", err)
		return 1
	}
	return 0
}

func sameArgs(args []string, values ...string) bool {
	if len(args) != len(values) {
		return false
	}
	for i := range args {
		if args[i] != values[i] {
			return false
		}
	}
	return true
}

func isGlobalAction(value string) bool {
	return value == "help" || value == "version" || value == "upgrade" || value == "end" || value == "reload"
}

func readerIsTerminal(reader io.Reader) bool {
	file, ok := reader.(*os.File)
	if !ok {
		return false
	}
	info, err := file.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice != 0
}

func SetRunAppForTest(fn func(config.AppConfig) int) func() {
	previous := runApp
	runApp = fn
	return func() {
		runApp = previous
	}
}
