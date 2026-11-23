## 1. Spec

- [x] 1.1 Validate with `openspec validate add-space-navigator --strict --no-interactive`

## 2. Hyprland space

- [x] 2.1 Add `internal/space`: read `hyprctl workspaces -j`, `hyprctl clients -j`, and `hyprctl activeworkspace -j`, build the X axis and the Z layers ordered by `focusHistoryID`
- [x] 2.2 Add the focus command: check the target against a fresh client list, then dispatch `hl.dsp.focus` for the workspace and then the window
- [x] 2.3 Cover ordering, filtering, the JSON shape, target checks, and the exact dispatches with fixture JSON (`GOWORK=off go test ./internal/space`)

## 3. Commands

- [x] 3.1 Add `tesseract space list`, `tesseract space focus <workspace-id> <address>`, and `tesseract open space`, and list them in the usage string and help
- [x] 3.2 Cover the commands with a fake `hyprctl` on `PATH` (`GOWORK=off go test ./internal/cli`)

## 4. Navigators

- [x] 4.1 Add the Quickshell overlay under `quickshell/space-navigator/`, embed it with the assets, and launch it from `tesseract open space` (`qmllint`, `GOWORK=off go test ./internal/assets ./internal/quickshell`)
- [x] 4.2 Add the terminal fallback navigator and test its keys (`GOWORK=off go test ./internal/spacenav`)

## 5. Check

- [x] 5.1 Run the whole module (`GOWORK=off go test ./...`, `GOWORK=off go vet ./...`)
- [x] 5.2 Confirm `c3po ast gap` reports `over 0`
- [x] 5.3 Document the space commands and keys in `README.md`

## 6. Orbital overlay

- [x] 6.1 Require the orbital system in the spec delta, proposal, and design, and re-validate with `openspec validate add-space-navigator --strict --no-interactive`
- [x] 6.2 Replace the rail, the slabs, and the card deck with the orbital stage (`SpaceStage`, `SpaceCamera`, `SpaceBody`, `SpaceSatellite`, `SpaceRing`, `SpaceSphere`, `SpaceGlow`), drawn with Qt Quick items and shapes through one projection and without Qt Quick 3D (`qmllint`)
- [x] 6.3 Animate it as an orbiting system: settle out of a spin while the camera falls in on open, turn the orbit on X, ease the shells between radii on Z, rush the chosen satellite before the hide and the focus, and keep idle motion to the chosen electron's swing and the halo's breathing
- [x] 6.4 Keep the assets test honest for the new files, and guard that no navigator file uses Qt Quick 3D and every text is plain text (`GOWORK=off go test ./internal/assets`)
- [x] 6.5 Render the stage offscreen with fixture data through Qt's OpenGL renderer at rest, during the open, mid-orbit, at two shell depths, with ten workspaces and a twelve-window workspace, and mid-rush; the live overlay is not opened
- [x] 6.6 Run `GOWORK=off go test -count=1 ./internal/space/ ./internal/spacenav/ ./internal/quickshell/ ./internal/cli/ ./internal/assets/`
- [x] 6.7 Describe the orbital system in `README.md`

## 7. Cycling axes

- [x] 7.1 Require both axes to cycle, and a cycle to move the picture one step, in the spec delta and design, and re-validate with `openspec validate add-space-navigator --strict --no-interactive`
- [x] 7.2 Cycle `h`/`l` and `k`/`j` in the overlay and the terminal navigator, and test each end of each axis (`GOWORK=off go test -count=1 ./internal/spacenav/ ./internal/quickshell/ ./internal/space/`)
- [x] 7.3 Ease the orbit and the shells to step counts that do not wrap, drawn as two crossfading laps, and drive the stage offscreen through each cycle in both directions; the live overlay is not opened

## 8. Move mode

- [x] 8.1 Require the shells and move mode in the spec delta, proposal, and design, and re-validate with `openspec validate add-space-navigator --strict --no-interactive`
- [x] 8.2 Add `tesseract space move <workspace-id> <address>`: check the window against a fresh client list, then one `hl.dsp.window.move` dispatch; cover it with fixture JSON and a fake `hyprctl` (`GOWORK=off go test ./internal/space ./internal/cli`)
- [x] 8.3 Add move mode to the terminal navigator: the first `m` picks up, `hjkl` carry, Escape or `q` cancels, Enter does nothing, and the second `m` moves after exit (`GOWORK=off go test ./internal/spacenav`)
- [x] 8.4 Add move mode to the overlay: the carry state and keys in `shell.qml`, the carried layout in `SpaceStage`, and the brighter carried electron in `SpaceSatellite` (`qmllint`)
- [x] 8.5 Drive `shell.qml` offscreen with Quickshell stubbed outside the repository through the first `m`, carries on X and Z with cycles, both cancels, Enter while carrying, and both put-downs, and compare the stage's navigation poses with the stage before move mode; the live overlay is not opened
- [x] 8.6 Keep the assets test naming the navigator's files, failing while `SpacePlate.qml` remains (`GOWORK=off go test ./internal/assets`)
- [x] 8.7 Run `GOWORK=off go test -count=1 ./internal/space/ ./internal/spacenav/ ./internal/quickshell/ ./internal/assets/ ./internal/cli/`
- [x] 8.8 Describe move mode and `tesseract space move` in `README.md`

## 9. One hostable navigator

- [x] 9.1 Require one navigator component that a shell hosts in the spec delta, proposal, and design, and re-validate with `openspec validate add-space-navigator --strict --no-interactive`
- [x] 9.2 Move the list, the selection, the carry, the keys, and the commands from `shell.qml` into `SpaceNavigator.qml`, with `hideRequested`, `finished`, and `closeRequested` for its host, and leave `shell.qml` the overlay's panel (`qmllint`)
- [x] 9.3 Let a host clear the void with `SpaceStage.voidOpacity`
- [x] 9.4 Keep the assets test naming the navigator's files, and guard that only `SpaceNavigator.qml` draws the stage and `shell.qml` hosts it (`GOWORK=off go test ./internal/assets`)
- [x] 9.5 Load `SpaceNavigator.qml` by file URL in a windowless Quickshell config, and drive the overlay offscreen through real key events with Quickshell stubbed outside the repository: the list, a move, Tab, Enter, Escape after a carry, and the second `m`; the live overlay is not opened
- [x] 9.6 Run `GOWORK=off go test -count=1 ./internal/space/ ./internal/spacenav/ ./internal/quickshell/ ./internal/assets/ ./internal/cli/`
- [x] 9.7 Describe the hostable component in `README.md`
