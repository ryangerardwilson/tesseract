## 1. Spec

- [x] 1.1 ADD the three requirements in `specs/space/spec.md` and validate with `c3po ast exec -- openspec validate add-and-close-workspaces --strict --no-interactive`
- [x] 1.2 Bring the space spec of the open changes in line: `add-space-navigator` (the only changes, the host's close, the terminal keys, the usage forms) and `name-apps-and-shift-with-s` (the footers at rest), and validate both with `--strict`

## 2. Commands

- [x] 2.1 `space.New` focuses the highest positive id plus one after the host layer is gone, and moves nothing
- [x] 2.2 `space.Close` refuses the only workspace and one off the X axis, sends one graceful close per listed app, and lands on the nearest workspace only when the active one is left empty
- [x] 2.3 `tesseract space new` and `tesseract space close <workspace-id>` in the CLI, the usage string, and the help
- [x] 2.4 Tests with the fake runner and the fake `hyprctl` on `PATH`; nothing reaches the live compositor

## 3. Navigators

- [x] 3.1 `SpaceNavigator.qml`: bare `n` adds and selects, bare `dd` within 400 ms closes, both through the confirm path; a lone `d`, Ctrl+D, a held `d`, or a `d` before another key close nothing; nothing while carrying; the only workspace stays
- [x] 3.2 `SpaceStage.qml` footer at rest gains `N  NEW` and `DD  CLOSE`
- [x] 3.3 `internal/spacenav` takes `n` and `dd` and runs the commands after it exits; the terminal footer gains `n new  dd close`
- [x] 3.4 qmllint on both QML files, and the keys driven offscreen with `qmltestrunner` and stubbed `Quickshell.Io`, through the launcher as host

## 4. Checks

- [x] 4.1 `GOWORK=off go test ./...` and `go vet` on the edited packages
- [x] 4.2 Install from `/tmp` with `TM_INSTALL_ACTIVATE_SYSTEMD=0`, confirm `tesseract-restore.service` is inactive and the installed QML matches the worktree
- [x] 4.3 README and `tesseract help` name `n`, `dd`, `space new`, and `space close`
