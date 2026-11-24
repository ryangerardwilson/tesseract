## Why

The space navigator calls a window on a shell a layer: its headline says `LAYER 1 / 2` and its footer `K J  LAYER`. The operator calls that window an app, and Hyprland already calls the navigator's own surface a layer. `m` picks up a window, so a host cannot bind bare `m`, and the Super+T launcher, which hosts the navigator, now opens Musica on it.

## What Changes

- The graphical navigator's headline reads `WORKSPACE 3   ·   APP 1 / 2`, and `NO APPS` on a workspace with no apps. It never says `LAYER`.
- Its footer reads `H L  WORKSPACE      K J  APP      ENTER  FOCUS      N  NEW      DD  CLOSE      Q  CLOSE` at rest, and `SHIFT      H L  WORKSPACE      K J  APP      S  PUT DOWN      Q  CANCEL` while a window is carried.
- `s` picks up the lit app and a second `s` puts it down, with the carries, cycles, cancel, and dispatches `m` had. Bare `m` does nothing in the navigator, so a host can bind it. Ctrl+M is still Enter and never shifts, and Ctrl+S does not shift.
- The terminal navigator takes `s` the same way. Its footer reads `enter focus  h/l workspace  k/j app  n new  dd close  q quit` at rest and `SHIFT  h/l k/j carry  s put down  q cancel` while carrying, and a workspace with no apps reads `No apps`.
- The open change `add-space-navigator`, the README, and `tesseract help` say app for a window on a shell, `s` for the shift key, `SHIFT` for the footer word, and shift mode for move mode.

## Non-goals

- Compositor layers keep their names: `TESSERACT_SPACE_WAIT_LAYER`, the wait for the host layer to go, the `hyprctl layers -j` check, and the exclusive layer surface.
- `tesseract space list` keeps its `layers` field. The error messages, the Go and QML identifiers, and the commands `tesseract space move` and `tesseract space focus` keep their names.
- No Musica bind here. The launcher that binds bare `m` lives in the home repository.
- The session browser's `m`, which marks a session, and the session radial are unchanged.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `space`: the navigators call a window on a shell an app and shift it with `s`. `space` is not a living spec yet: the open change `add-space-navigator` adds it, so this change ADDs one requirement to it.

## Impact

- `quickshell/space-navigator/SpaceNavigator.qml` takes `s` after the confirm check and leaves bare `m` to its host. `SpaceStage.qml` draws the new headline and footer.
- `internal/spacenav` takes `s`. `internal/components/l3` draws the new footers and `internal/components/l2` the empty stack.
- The `internal/spacenav` tests press `s` and cover bare `m`, Ctrl+M, and Ctrl+S.
- `README.md`, the help text in `internal/cli/cli.go`, and `openspec/changes/add-space-navigator` change words only.
- A shell that hosts `SpaceNavigator.qml`, such as the Super+T launcher, gets bare `m` back unhandled and shifts with `s`.
