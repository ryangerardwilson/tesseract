## Why

tesseract navigates tmux sessions, but the operator has no matching way to move around the Hyprland desktop. Super+H/J/K/L focus by tile direction and `navigate-workspace-shader.sh` steps the workspace id by one, but nothing shows the workspaces side by side with the windows stacked inside each one, and nothing lets the operator pick a buried window without walking every tile.

## What Changes

- A new `space` model of the desktop with two axes. X is the normal workspaces (positive ids) in ascending id order, as one row. Z is the windows of one workspace, stacked like layers: the front layer is the window that workspace focused most recently, ordered by `focusHistoryID` among that workspace's windows. There is no Y axis and no grid.
- `tesseract space list` prints that space as JSON, X in order, each workspace's Z layers front first. It only reads Hyprland.
- `tesseract space focus <workspace-id> <address>` checks the layer against a fresh client list, then focuses that workspace and that window through `hl.dsp.focus`. It never moves a window.
- `tesseract space move <workspace-id> <address>` checks the window against a fresh client list, then moves it to that workspace with one dispatch, `hl.dsp.window.move({ workspace = "<id>", window = "address:<addr>" })`: the stock Hyprland bind with the window selector focus uses.
- `tesseract open space` opens a Quickshell overlay that draws the space as an orbital system on a black void. The selected workspace is the nucleus, a glowing glass sphere at the centre with its number on it. The other workspaces are smaller glass bodies on one equatorial orbit around it, the X axis, seen from just above its plane: lower ids to the left, higher ids to the right, foreshortened as the orbit turns away. The selected workspace's windows are electrons on concentric shells around the nucleus, the Z axis: the front layer's shell is the innermost, tight and bright, each deeper layer's shell is wider and dimmer, and each shell carries one satellite with that window's class and title. Left/Right or `h`/`l` turn the whole system about its vertical axis so the chosen workspace swings to the front and becomes the nucleus, Up/`k` and Down/`j` step inward and outward through the shells while they ease between radii, Enter rushes the chosen satellite toward the eye and then focuses, and `q`/Escape close. Opening settles the system out of a slow spin while the camera falls in toward the nucleus. The overlay gets its data from `tesseract space list`, focuses through `tesseract space focus`, and moves through `tesseract space move`; it does not parse `hyprctl`.
- Move mode: `m` picks up the lit electron's window. `h`/`l` carry it round the orbit to another workspace, which becomes the nucleus with the carried electron lit on its tight shell, and `j`/`k` carry it one shell deeper or toward the front among that workspace's layers; both cycle and ease one step. The carried electron is brighter and the footer says `MOVE`. A second `m` puts the window down with one `tesseract space move`, Escape or `q` puts the picture back and dispatches nothing, and Enter does nothing while a window is carried. Hyprland does not change until the second `m`. Behind another window, the window is focused after the move so it lands on top, because `/usr/share/hypr` documents no z-order dispatcher's arguments.
- The first version of the overlay drew a flat row of workspace tiles and a vertical list of window cards. The operator rejected it: a list does not read as depth. The second version stood the workspaces as glass slabs on one straight rail, with the selected workspace's windows as a deck of cards. The operator found it much better but asked for the whole picture to be rethought as a spherical, planetary, atomic system instead of slabs on a rail. This change requires the orbital system.
- The overlay's state, keys, and commands live in one component, `quickshell/space-navigator/SpaceNavigator.qml`, which the overlay's panel hosts and another Quickshell shell, such as the Super+T launcher, can host too, so every host shows the same picture with the same keys.
- When the overlay cannot run, `tesseract open space` falls back to a terminal navigator in the session browser's dense keyboard style, with the same keys and the same move mode.
- The usage string and help list the four new forms.

## Non-goals

- No Y axis: no polar ring, no second orbit tilted up, no column, grid, or second row of workspaces, and no ordering by a window's pixel position.
- No Qt Quick 3D, no floor rail or grid, no particles, no planet names, and no colour beyond white glass on a near-black void.
- No change to Super+H/J/K/L, no new Hyprland bind, and no change to `navigate-workspace-shader.sh` or anything under `~/.config/hypr`.
- `special:` workspaces and negative-id workspaces are not on the X axis. A pinned window is not copied onto every column.
- Opening, moving inside, carrying, cancelling, and dismissing the navigator never focus or move anything. Confirm and the second `m` of move mode are the only changes.
- No `movetoworkspacesilent`, no `hyprctl keyword`, and no z-order dispatch whose arguments `/usr/share/hypr` does not document.
- The existing tmux commands, the session picker under `quickshell/session-picker/`, and the snapshot requirement in `openspec/specs/tesseract/spec.md` are unchanged.

## Capabilities

### New Capabilities

- `space`: the Hyprland desktop as an X axis of workspaces and a Z axis of window layers, listed as JSON, focused one layer at a time, and navigated from a Quickshell overlay or a terminal fallback.

### Modified Capabilities

None.

## Impact

- New package `internal/space` owns the Hyprland reads (`hyprctl workspaces -j`, `hyprctl clients -j`, `hyprctl activeworkspace -j`), the ordering, the focus dispatches, and the move dispatch. `internal/tmux` stays tmux-only.
- New terminal navigator `internal/spacenav`, with rendering in `internal/components/l2` and `internal/components/l3`.
- New Quickshell overlay `quickshell/space-navigator/`, embedded with the other assets and launched by `internal/quickshell`. It draws its spheres and rings with Qt Quick items and Qt Quick Shapes through its own perspective projection and needs no Qt Quick 3D.
- `internal/cli` gains `tesseract open space`, `tesseract space list`, `tesseract space focus`, and `tesseract space move`.
- Runtime needs Hyprland 0.56 with the Lua dispatcher (`hl.dsp.focus`). The overlay needs Quickshell; without it the terminal navigator runs.
