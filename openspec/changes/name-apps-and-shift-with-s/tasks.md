## 1. Spec

- [x] 1.1 Add the requirement in `specs/space/spec.md` and validate with `openspec validate name-apps-and-shift-with-s --strict --no-interactive`
- [x] 1.2 Say app, `s`, `SHIFT`, and shift mode in the open change `add-space-navigator`, keeping the `layers` field, the commands, the error messages, and every compositor layer, and validate with `openspec validate add-space-navigator --strict --no-interactive`
- [x] 1.3 Check the living specs `tesseract` and `c3po-resume` for the navigator's `m`, `MOVE`, or a window called a layer; they have none

## 2. Graphical navigator

- [x] 2.1 Draw `APP <n> / <count>`, `NO APPS`, and the rest and carrying footers in `SpaceStage.qml`
- [x] 2.2 Carry on `s` without Control after the confirm check in `SpaceNavigator.qml`, drop the bare `m` branch, and keep Ctrl+M as Enter
- [x] 2.3 Lint both files with `qmllint` and compare the warnings with copies of HEAD's files
- [x] 2.4 Confirm `grep -rn -E 'LAYER|MOVE ' quickshell/space-navigator` finds only `TESSERACT_SPACE_WAIT_LAYER` and `Key_M` is left only in the Ctrl+M confirm
- [x] 2.5 Drive `SpaceNavigator.qml` offscreen through real key events with `qmltestrunner` and `Quickshell.Io` stubbed outside the repository: the headline and footers, `s` and a put-down, Escape, bare `m` left unaccepted, Ctrl+S, and Ctrl+M; the live overlay is not opened

## 3. Terminal navigator

- [x] 3.1 Carry on `s` in `internal/spacenav`, keeping `enter` and `ctrl+m` as confirm
- [x] 3.2 Draw the rest and carrying footers in `internal/components/l3` and `No apps` in `internal/components/l2`
- [x] 3.3 Press `s` where the tests pressed `m`, assert `SHIFT` and both footers, and test that bare `m` and Ctrl+S leave the navigator as it was and Ctrl+M never picks up or puts down (`GOWORK=off go test ./internal/spacenav`)

## 4. Docs and checks

- [x] 4.1 Say app and `s` in the README's space navigator section and in `tesseract help`
- [x] 4.2 Run `GOWORK=off go test ./...` and `GOWORK=off go vet ./internal/spacenav/ ./internal/components/...`
- [x] 4.3 Re-validate both changes with `--strict --no-interactive`
- [ ] 4.4 Install, then press `s`, `m`, and Ctrl+M in the overlay and in the Super+T launcher
