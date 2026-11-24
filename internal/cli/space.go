package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"github.com/ryangerardwilson/tesseract/internal/quickshell"
	"github.com/ryangerardwilson/tesseract/internal/space"
	"github.com/ryangerardwilson/tesseract/internal/spacenav"
	"github.com/ryangerardwilson/tesseract/internal/tmux"
)

// isSpaceCommand reports whether argv is one of the Hyprland space forms.
// They never touch tmux.
func isSpaceCommand(argv []string) bool {
	return argv[0] == "space" || (len(argv) == 2 && argv[0] == "open" && argv[1] == "space")
}

func runSpace(argv []string, api *tmux.API, out io.Writer, errOut io.Writer) int {
	if err := dispatchSpace(argv, api, out); err != nil {
		fmt.Fprintln(errOut, err)
		return 1
	}
	return 0
}

func dispatchSpace(argv []string, api *tmux.API, out io.Writer) error {
	switch {
	case argv[0] == "open":
		return openSpace(api)
	case len(argv) == 2 && argv[1] == "list":
		current, err := space.List(space.Hyprctl)
		if err != nil {
			return err
		}
		return json.NewEncoder(out).Encode(current)
	case len(argv) == 2 && argv[1] == "new":
		id, err := space.New(space.Hyprctl)
		if err != nil {
			return err
		}
		return json.NewEncoder(out).Encode(map[string]int{"workspace": id})
	case len(argv) == 3 && argv[1] == "close":
		id, err := space.ParseWorkspace(argv[2])
		if err != nil {
			return err
		}
		closed, err := space.Close(space.Hyprctl, id)
		if err != nil {
			return err
		}
		return json.NewEncoder(out).Encode(closed)
	case len(argv) == 4 && argv[1] == "focus":
		target, err := space.ParseTarget(argv[2], argv[3])
		if err != nil {
			return err
		}
		if err := space.Focus(space.Hyprctl, target); err != nil {
			return err
		}
		return json.NewEncoder(out).Encode(target)
	case len(argv) == 4 && argv[1] == "move":
		target, err := space.ParseTarget(argv[2], argv[3])
		if err != nil {
			return err
		}
		if err := space.Move(space.Hyprctl, target); err != nil {
			return err
		}
		return json.NewEncoder(out).Encode(target)
	}
	return errors.New(space.Usage)
}

// openSpace opens the Quickshell overlay, and the terminal navigator when the
// overlay cannot run from an interactive terminal.
func openSpace(api *tmux.API) error {
	err := quickshell.OpenSpace(api, appDir(api))
	if err == nil {
		return nil
	}
	if stdinIsTerminal() && stdoutIsTerminal() {
		return spacenav.Run(space.Hyprctl)
	}
	return err
}
