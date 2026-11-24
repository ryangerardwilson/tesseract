## ADDED Requirements

### Requirement: tesseract space new adds a workspace

`tesseract space new` SHALL read `hyprctl workspaces -j` and take the id one higher than the highest positive workspace id it reports, or 1 when none is positive. Lower ids that are free SHALL NOT be reused. It SHALL then run exactly one dispatch, `hyprctl dispatch 'hl.dsp.focus({ workspace = "<id>" })'`, which makes Hyprland create that workspace and land on it. It MUST NOT move any window, and MUST NOT run `hl.dsp.window.move`, `movetoworkspacesilent`, `focuswindow`, or `hyprctl keyword`. With `TESSERACT_SPACE_WAIT_LAYER=1` it SHALL dispatch only once the host layer is gone. A dispatch that exits non-zero or replies with anything other than `ok` MUST make the command exit non-zero. On success it SHALL print the new workspace id as JSON.

#### Scenario: A workspace past the highest id

- **GIVEN** `hyprctl workspaces -j` reports workspace ids -98 (`special:musica-hidden`), 3, 1, 2, 4, and -1337 (`name:scratch`)
- **WHEN** the operator runs `tesseract space new`
- **THEN** hyprctl MUST receive `dispatch 'hl.dsp.focus({ workspace = "5" })'` and no other dispatch
- **AND** the command MUST print `{"workspace":5}`

#### Scenario: A gap is not reused

- **GIVEN** `hyprctl workspaces -j` reports workspaces 1 and 7
- **WHEN** the operator runs `tesseract space new`
- **THEN** it MUST focus workspace 8

#### Scenario: New waits for the host layer

- **GIVEN** `TESSERACT_SPACE_WAIT_LAYER=1` and `hyprctl layers -j` still shows `omarchy-launcher`
- **WHEN** `tesseract space new` runs
- **THEN** it MUST read `hyprctl layers -j` again until the launcher is gone, and only then dispatch the focus

### Requirement: tesseract space close closes the apps of one workspace

`tesseract space close <workspace-id>` SHALL accept only a positive integer `<workspace-id>`. It SHALL read the space afresh as `tesseract space list` does. A workspace that is not on the X axis, and the only workspace on the X axis, MUST be refused before any dispatch. Otherwise, once the host layer is gone when `TESSERACT_SPACE_WAIT_LAYER=1`, it SHALL run one dispatch for each app listed on that workspace, `hyprctl dispatch 'hl.dsp.window.close({ window = "address:<address>" })'`, the graceful close request, front app first.

It MUST NOT run `hl.dsp.window.kill`, `closewindow`, `focuswindow`, `movetoworkspacesilent`, or `hyprctl keyword`, and MUST NOT send a second close to any window. It MUST NOT close a window on another workspace, a window on a special workspace, a hidden or unmapped window, the launcher, or any layer-shell surface. A close that exits non-zero or replies with anything other than `ok` SHALL NOT stop the other closes, and it SHALL make the command exit non-zero once they are sent.

When the closed workspace was the active one, the command SHALL read `hyprctl clients -j` until no mapped window is left on that workspace, for at most about 1.5 s. If none is left, it SHALL focus the nearest other workspace on the X axis, the next lower id or else the next higher, with `hl.dsp.focus({ workspace = "<id>" })`, so Hyprland drops the empty workspace. If a window is still mapped there, it SHALL leave that workspace focused and close nothing more. A closed workspace that was not the active one SHALL get no focus dispatch. On success it SHALL print the workspace id, the addresses it closed, and any workspace it focused, as JSON.

#### Scenario: Closing a background workspace

- **GIVEN** the active workspace is 3, and workspace 1 lists the apps `0x55d0000000f6` and then `0x55d0000000b2` and also holds the unmapped window `0x55d000000118`
- **WHEN** the operator runs `tesseract space close 1`
- **THEN** hyprctl MUST receive `dispatch 'hl.dsp.window.close({ window = "address:0x55d0000000f6" })'` and then `dispatch 'hl.dsp.window.close({ window = "address:0x55d0000000b2" })'`
- **AND** it MUST receive no other dispatch, none for `0x55d000000118`, and no focus

#### Scenario: Closing the active workspace lands on the next lower one

- **GIVEN** the active workspace is 3 of workspaces 1, 2, 3, and 4, and both of its apps close when asked
- **WHEN** the operator runs `tesseract space close 3`
- **THEN** hyprctl MUST receive the two closes and then `dispatch 'hl.dsp.focus({ workspace = "2" })'`

#### Scenario: The lowest active workspace lands on the next higher one

- **GIVEN** the active workspace is 1 of workspaces 1, 2, 3, and 4, and its apps close when asked
- **WHEN** the operator runs `tesseract space close 1`
- **THEN** it MUST focus workspace 2 after the closes

#### Scenario: A window that stays keeps the workspace

- **GIVEN** the active workspace is 3 and one of its apps raises a save dialog instead of closing
- **WHEN** the operator runs `tesseract space close 3`
- **THEN** hyprctl MUST receive one close per app and no focus dispatch
- **AND** nothing MUST be killed or closed a second time

#### Scenario: The only workspace is refused

- **GIVEN** workspace 1 is the only workspace on the X axis
- **WHEN** the operator runs `tesseract space close 1`
- **THEN** the command MUST exit non-zero
- **AND** hyprctl MUST NOT receive any dispatch

#### Scenario: A workspace off the axis or a malformed id is refused

- **GIVEN** a window reports workspace 7, which `hyprctl workspaces -j` does not report
- **WHEN** the operator runs `tesseract space close 7`, `tesseract space close 0`, or `tesseract space close special`
- **THEN** each command MUST exit non-zero
- **AND** hyprctl MUST NOT receive any dispatch

#### Scenario: Close waits for the host layer

- **GIVEN** `TESSERACT_SPACE_WAIT_LAYER=1` and `hyprctl layers -j` still shows `omarchy-launcher`
- **WHEN** `tesseract space close 2` runs
- **THEN** it MUST send no close until the launcher is gone

### Requirement: The navigators add a workspace with n and close one with dd

In the graphical navigator and the terminal navigator, with no window carried, bare `n` SHALL add a workspace through `tesseract space new`, and the new workspace SHALL become the selected one. Bare `d` followed by a second bare `d` within about 400 ms SHALL close the selected workspace through `tesseract space close <workspace-id>`, and that workspace SHALL leave the orbit. In the graphical navigator, both SHALL take the path of Enter: the navigator SHALL ask its host to hide, start the command in its own session with `TESSERACT_SPACE_WAIT_LAYER=1`, and tell its host to close once it is started. The operator SHALL land on the new workspace, and a save dialog that closing raises SHALL be reachable. The terminal navigator SHALL exit and then run the same command.

A single `d`, Ctrl+D, Shift+D, a held `d` that only repeats, and a `d` followed by any other key MUST NOT close anything, and that other key SHALL do its own job. While a window is carried, `n` and `d` SHALL do nothing. With one workspace on the X axis, `dd` SHALL do nothing. Bare `n` and `dd` MUST NOT change Hyprland except through these commands. A host SHALL treat `n` and `dd` as it treats Enter, so a host that remembers a confirm, such as the Super+T launcher, still reopens on the space navigator.

The graphical footer at rest SHALL read `H L  WORKSPACE      K J  APP      ENTER  FOCUS      N  NEW      DD  CLOSE      Q  CLOSE`. The terminal footer at rest SHALL read `enter focus  h/l workspace  k/j app  n new  dd close  q quit`, broken onto a second line before `n new` when the terminal is narrower than that line. The footers while carrying SHALL NOT name `n` or `dd`.

#### Scenario: n lands on a new workspace

- **GIVEN** workspaces 1, 2, 3, and 4, and the graphical navigator resting on workspace 3
- **WHEN** the operator presses `n`
- **THEN** workspace 5 MUST be the selected workspace, with no apps
- **AND** the navigator MUST ask its host to hide and start `tesseract space new` with `TESSERACT_SPACE_WAIT_LAYER=1` in its own session, and then tell the host to close
- **AND** no window MUST move

#### Scenario: dd closes the selected workspace

- **GIVEN** the graphical navigator rests on workspace 2 of workspaces 1, 2, and 3
- **WHEN** the operator presses `d` and then `d` within 400 ms
- **THEN** workspace 2 MUST leave the orbit
- **AND** the navigator MUST ask its host to hide and start `tesseract space close 2` with `TESSERACT_SPACE_WAIT_LAYER=1`, and then tell the host to close

#### Scenario: A lone d closes nothing

- **GIVEN** the graphical navigator rests on workspace 2
- **WHEN** the operator presses `d`, waits more than 400 ms, and presses `d` once more
- **THEN** nothing MUST be closed or dispatched
- **AND** the later `d` MUST arm a new 400 ms window

#### Scenario: A d before another key

- **GIVEN** the graphical navigator rests on workspace 2 of workspaces 1, 2, and 3
- **WHEN** the operator presses `d` and then `l`
- **THEN** workspace 3 MUST be selected and nothing MUST be closed
- **AND** a `d` right after the `l` MUST only arm a new window

#### Scenario: Ctrl+D and a held d close nothing

- **GIVEN** the graphical navigator rests on a workspace with apps
- **WHEN** the operator presses Ctrl+D twice, or holds `d` so that it repeats
- **THEN** nothing MUST be closed or dispatched

#### Scenario: n and dd wait while an app is carried

- **GIVEN** the graphical navigator carries an app
- **WHEN** the operator presses `n`, or `d` twice
- **THEN** nothing MUST be added, closed, or dispatched, and the app MUST stay carried

#### Scenario: The only workspace stays

- **GIVEN** workspace 1 is the only workspace on the X axis
- **WHEN** the operator presses `d` twice
- **THEN** nothing MUST be closed or dispatched

#### Scenario: The terminal navigator adds and closes

- **GIVEN** the terminal navigator rests on workspace 2 of workspaces 1, 2, and 3
- **WHEN** the operator presses `n`, or presses `d` twice within 400 ms
- **THEN** the terminal navigator MUST exit and then run the checks and dispatches of `tesseract space new`, or of `tesseract space close 2`

#### Scenario: The footers name the keys

- **GIVEN** a navigator at rest with no app carried
- **WHEN** its footer is drawn
- **THEN** the graphical footer MUST read `H L  WORKSPACE      K J  APP      ENTER  FOCUS      N  NEW      DD  CLOSE      Q  CLOSE`
- **AND** the terminal footer MUST read `enter focus  h/l workspace  k/j app  n new  dd close  q quit` on a terminal wide enough for it, and break before `n new` on a narrower one
