package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/ryangerardwilson/tesseract/internal/assets"
	"github.com/ryangerardwilson/tesseract/internal/browser"
	"github.com/ryangerardwilson/tesseract/internal/keyd"
	"github.com/ryangerardwilson/tesseract/internal/quickshell"
	"github.com/ryangerardwilson/tesseract/internal/snapshot"
	"github.com/ryangerardwilson/tesseract/internal/tmux"
	"github.com/ryangerardwilson/tesseract/internal/version"
)

const usage = "valid shape: tesseract open picker | tesseract open browser | tesseract open space | tesseract reload bindings | tesseract restore sessions | tesseract snapshot sessions | tesseract attach session <name> | tesseract quickshell list | tesseract quickshell select <name> | tesseract space list | tesseract space new | tesseract space focus <workspace-id> <address> | tesseract space move <workspace-id> <address> | tesseract space close <workspace-id> | tesseract km config | tesseract km apply | tesseract km status"

const helpText = `tesseract
tmux session picker and restore utility

global actions:
  tesseract help
    show this help
  tesseract version
    print the installed version
  tesseract upgrade
    upgrade to the latest release

features:
  open the graphical radial session picker
  # open picker
  tesseract open picker

  open the terminal session browser
  # open browser
  tesseract open browser

  open the hyprland navigator: workspaces left to right, each one's windows stacked front to back
  # open space
  tesseract open space

  clean tesseract-managed bindings from the live tmux server and reapply them
  # reload bindings
  tesseract reload bindings

  restore saved tmux sessions now
  # restore sessions
  tesseract restore sessions

  save the current tmux sessions now
  # snapshot sessions
  tesseract snapshot sessions

  attach to a named session, or create it first if needed
  # attach session <session_name>
  tesseract attach session root

  expose tmux session data and actions for user-owned Quickshell config
  # quickshell list | quickshell select <session_name>
  tesseract quickshell list
  tesseract quickshell select root

  print the hyprland workspaces and their apps as JSON, focus one app, or move one window to a workspace
  # space list | space focus <workspace_id> <address> | space move <workspace_id> <address>
  tesseract space list
  tesseract space focus 3 0x55d0000000d4
  tesseract space move 2 0x55d0000000d4

  add the workspace past the highest id and land on it, or close every app on one workspace gracefully
  # space new | space close <workspace_id>
  tesseract space new
  tesseract space close 4

  edit, install, or inspect the managed keyd sticky-keys config
  # km config | km apply | km status
  tesseract km config
  tesseract km apply
  tesseract km status

  inside the browser, create or rename a named session
  # n | r
  n
`

type command struct {
	name  string
	value string
}

func Run(argv []string, api *tmux.API, out io.Writer, errOut io.Writer) int {
	if api == nil {
		api = tmux.NewAPI(nil)
	}
	if out == nil {
		out = os.Stdout
	}
	if errOut == nil {
		errOut = os.Stderr
	}
	if len(argv) == 0 || (len(argv) == 1 && argv[0] == "help") {
		fmt.Fprint(out, helpText)
		return 0
	}
	if len(argv) == 1 && argv[0] == "version" {
		fmt.Fprintln(out, version.Version)
		return 0
	}
	if len(argv) == 1 && argv[0] == "upgrade" {
		return upgradeApp(api, errOut)
	}
	if argv[0] == "help" || argv[0] == "version" || argv[0] == "upgrade" {
		fmt.Fprintln(errOut, "global actions must be used alone")
		return 1
	}
	if argv[0] == "km" {
		return runKM(argv[1:], api, out, errOut)
	}
	if isSpaceCommand(argv) {
		return runSpace(argv, api, out, errOut)
	}

	if shouldPrepareTmux(argv) {
		parsed, err := parseArgs(argv)
		if err != nil {
			fmt.Fprintln(errOut, err)
			return 1
		}
		if parsed.name == "session" {
			if _, err := snapshot.RestoreSavedSessionsIfNeeded(api); err != nil {
				fmt.Fprintln(errOut, err)
				return 1
			}
		}
		if _, err := tmux.EnsureIndexSession(api); err != nil {
			fmt.Fprintln(errOut, err)
			return 1
		}
	}

	if err := dispatch(argv, api, out); err != nil {
		fmt.Fprintln(errOut, err)
		return 1
	}
	return 0
}

func RunInvoked(argv0 string, argv []string, api *tmux.API, out io.Writer, errOut io.Writer) int {
	if filepath.Base(argv0) == "km" {
		argv = append([]string{"km"}, argv...)
	}
	return Run(argv, api, out, errOut)
}

func runKM(argv []string, api *tmux.API, out io.Writer, errOut io.Writer) int {
	if len(argv) == 0 || (len(argv) == 1 && argv[0] == "help") {
		keyd.WriteHelp(out)
		return 0
	}
	if len(argv) == 1 && argv[0] == "version" {
		fmt.Fprintln(out, version.Version)
		return 0
	}
	if len(argv) == 1 && argv[0] == "upgrade" {
		return upgradeApp(api, errOut)
	}
	if len(argv) == 1 && (argv[0] == "config" || argv[0] == "apply" || argv[0] == "status") {
		return keyd.NewRunner(out, errOut, os.Stdin).Run(argv)
	}
	keyd.WriteHelp(out)
	return 1
}

func dispatch(argv []string, api *tmux.API, out io.Writer) error {
	if len(argv) > 0 && argv[0] == "quickshell" {
		payload, err := quickshell.APIPayload(api, argv[1:])
		if err != nil {
			return err
		}
		return json.NewEncoder(out).Encode(payload)
	}
	if len(argv) == 1 && argv[0] == "__picker-payload" {
		payload, err := quickshell.Payload(api, "")
		if err != nil {
			return err
		}
		return json.NewEncoder(out).Encode(payload)
	}
	if len(argv) == 3 && argv[0] == "__picker-create" {
		_, err := quickshell.CreateSessionForPicker(api, argv[1], argv[2])
		return err
	}
	if len(argv) == 3 && argv[0] == "__picker-delete" {
		_, err := quickshell.DeleteSessionForPicker(api, argv[1], argv[2])
		return err
	}
	if len(argv) == 4 && argv[0] == "__picker-rename" {
		_, err := quickshell.RenameSessionForPicker(api, argv[1], argv[2], argv[3])
		return err
	}
	if len(argv) == 2 && argv[0] == "__desktop-select" {
		_, err := quickshell.DesktopSelectSession(api, argv[1])
		return err
	}
	if len(argv) == 2 && argv[0] == "__install-assets" {
		return assets.Write(argv[1])
	}
	if len(argv) == 4 && argv[0] == "__render-tmux-config" {
		return assets.RenderTmuxConfig(argv[1], argv[2], argv[3])
	}

	parsed, err := parseArgs(argv)
	if err != nil {
		return err
	}
	switch parsed.name {
	case "picker":
		return openPicker(api)
	case "reload":
		return tmux.ReloadManagedConfig(api)
	case "restore":
		_, err := snapshot.RestoreSavedSessions(api)
		return err
	case "snapshot":
		_, err := snapshot.WriteRuntimeSnapshotIfNonempty(api, nil)
		return err
	case "browser":
		rc := browser.Run(api, true)
		if rc != 0 {
			return fmt.Errorf("browser exited with status %d", rc)
		}
		return nil
	case "session":
		tmux.EnsureIndexSession(api)
		rc := tmux.AttachOrCreateSession(api, parsed.value)
		if rc != 0 {
			return fmt.Errorf("tmux exited with status %d", rc)
		}
		return nil
	default:
		return fmt.Errorf(usage)
	}
}

func parseArgs(argv []string) (command, error) {
	if len(argv) == 2 && argv[0] == "open" && argv[1] == "picker" {
		return command{name: "picker"}, nil
	}
	if len(argv) == 2 && argv[0] == "open" && argv[1] == "browser" {
		return command{name: "browser"}, nil
	}
	if len(argv) == 2 && argv[0] == "reload" && argv[1] == "bindings" {
		return command{name: "reload"}, nil
	}
	if len(argv) == 2 && argv[0] == "restore" && argv[1] == "sessions" {
		return command{name: "restore"}, nil
	}
	if len(argv) == 2 && argv[0] == "snapshot" && argv[1] == "sessions" {
		return command{name: "snapshot"}, nil
	}
	if len(argv) == 3 && argv[0] == "attach" && argv[1] == "session" && argv[2] != "" && argv[2][0] != '-' {
		return command{name: "session", value: argv[2]}, nil
	}
	return command{}, fmt.Errorf(usage)
}

func shouldPrepareTmux(argv []string) bool {
	parsed, err := parseArgs(argv)
	if err != nil {
		return false
	}
	return parsed.name == "browser" || parsed.name == "session"
}

func openPicker(api *tmux.API) error {
	selection, err := quickshell.PickSession(api, appDir(api))
	if err != nil {
		if _, restoreErr := snapshot.RestoreSavedSessionsIfNeeded(api); restoreErr != nil {
			return restoreErr
		}
		if stdinIsTerminal() && stdoutIsTerminal() {
			rc := browser.Run(api, false)
			if rc != 0 {
				return fmt.Errorf("browser exited with status %d", rc)
			}
			return nil
		}
		rc := tmux.AttachOrCreateSession(api, tmux.IndexSessionName)
		if rc != 0 {
			return fmt.Errorf("tmux exited with status %d", rc)
		}
		return nil
	}
	if selection == nil {
		return nil
	}
	sessionName, err := quickshell.ValidateSessionName(selection.Name)
	if err != nil {
		return err
	}
	if selection.Action == "select" && !api.HasSession(sessionName) {
		return fmt.Errorf("No such session: %s", sessionName)
	}
	rc := tmux.AttachOrCreateSession(api, sessionName)
	if rc != 0 {
		return fmt.Errorf("tmux exited with status %d", rc)
	}
	return nil
}

func upgradeApp(api *tmux.API, errOut io.Writer) int {
	script := filepath.Join(appDir(api), "install.sh")
	if _, err := os.Stat(script); err != nil {
		if local, localErr := filepath.Abs("install.sh"); localErr == nil {
			if _, statErr := os.Stat(local); statErr == nil {
				script = local
			}
		}
	}
	if _, err := os.Stat(script); err != nil {
		fmt.Fprintf(errOut, "install.sh is missing: %s\n", script)
		return 1
	}
	cmd := exec.Command("bash", script, "upgrade")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	if err := cmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return exitErr.ExitCode()
		}
		fmt.Fprintln(errOut, err)
		return 1
	}
	return 0
}

func appDir(api *tmux.API) string {
	if value := os.Getenv("TM_APP_DIR"); value != "" {
		return value
	}
	return assets.DefaultAppDir(api.Env["HOME"])
}

func stdinIsTerminal() bool {
	info, err := os.Stdin.Stat()
	return err == nil && (info.Mode()&os.ModeCharDevice) != 0
}

func stdoutIsTerminal() bool {
	info, err := os.Stdout.Stat()
	return err == nil && (info.Mode()&os.ModeCharDevice) != 0
}
