## Why

The space navigator moves through the workspaces that exist and carries apps between them, but a fresh workspace still needs a keybind outside the menu, and emptying a workspace means closing its apps one at a time. The operator wants both from the electron navigator: `n` for a new workspace and `dd` to close the selected one.

## What Changes

- `tesseract space new` focuses the workspace one past the highest positive id. Hyprland makes it on focus, and no window moves.
- `tesseract space close <workspace-id>` sends one graceful close per app that `tesseract space list` lists on that workspace, and never kills. When the workspace was the active one and nothing is left on it, it focuses the nearest other workspace on the X axis, the next lower id or else the next higher, so Hyprland drops the empty one. A window that stays (a save dialog, or an app that ignored the close) keeps the workspace, and focus stays there. The only workspace, and a workspace not on the X axis, are refused.
- In the graphical navigator, bare `n` adds the workspace and selects it, and bare `d` followed by a second bare `d` within about 400 ms closes the selected workspace. Both take the confirm path of Enter: the host hides, the command starts in its own session with `TESSERACT_SPACE_WAIT_LAYER=1`, and the host closes. The closes therefore run after the menu is gone, so a save dialog can be reached.
- A single `d`, Ctrl+D, Shift+D, a held `d`, or a `d` followed by another key closes nothing; the other key does its own job. `n` and `d` do nothing while an app is carried, and `dd` does nothing when only one workspace is on the X axis.
- The terminal navigator takes `n` and `dd` the same way and runs the command after it exits.
- The footers gain `N  NEW` and `DD  CLOSE` (`n new  dd close` in the terminal). The usage string lists `tesseract space new` and `tesseract space close <workspace-id>`.

## Non-goals

- No `hl.dsp.window.kill`, `closewindow`, `focuswindow`, `movetoworkspacesilent`, or `hyprctl keyword`, and no second close for an app that stays.
- No closing of special workspaces, hidden or unmapped windows, the launcher, or any layer-shell surface.
- No change to the Super+T launcher. It already hides on `hideRequested` and keeps the space reopen mark on `finished`, so `n` and `dd` count as confirms there. Its tmux radial keeps `n` for a new session, and Musica and the other views keep their keys.
- Compositor layers keep their names, and `tesseract space list` keeps its `layers` field.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `space`: two commands and two navigator keys. `space` is not a living spec yet. The open change `add-space-navigator` adds it, so this change ADDs three requirements and brings the sentences of `add-space-navigator` and `name-apps-and-shift-with-s` that name the only changes, the host's close, the terminal keys, the usage forms, and the footers in line with them.

## Impact

`internal/space` (`New`, `Close`, `ParseWorkspace`), `internal/cli` (dispatch, usage, help), `internal/spacenav` and `internal/components/l3` (keys, footer), `quickshell/space-navigator/SpaceNavigator.qml` and `SpaceStage.qml` (keys, footer), README. Tests reach Hyprland only through a fake runner or a fake `hyprctl` on `PATH`.
