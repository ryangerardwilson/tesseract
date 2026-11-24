## ADDED Requirements

### Requirement: The navigator calls a window an app and shifts it with s

Both navigators SHALL call a window on a shell an app wherever the operator reads them, and MUST NOT call it a layer. The graphical navigator's headline SHALL read `WORKSPACE <name>   ·   APP <n> / <count>`, counting the front app as 1, and `WORKSPACE <name>   ·   NO APPS` on a workspace with no apps. Its footer SHALL read `H L  WORKSPACE      K J  APP      ENTER  FOCUS      N  NEW      DD  CLOSE      Q  CLOSE` with no window carried and `SHIFT      H L  WORKSPACE      K J  APP      S  PUT DOWN      Q  CANCEL` while one is carried. `s` without Control SHALL pick up the lit app and a second `s` SHALL put it down, with the carries, cycles, cancel, and dispatches that "The graphical navigator carries a window in shift mode" requires. Bare `m` MUST NOT pick up or put down a window or change anything else in either navigator. While no focus or move is under way, the graphical navigator SHALL leave bare `m` unhandled, so a host MAY bind it. Ctrl+M SHALL confirm as Enter does when no window is carried, SHALL do nothing while one is carried, and MUST NOT pick up or put down a window. Ctrl+S MUST NOT pick up or put down a window. The terminal navigator SHALL take `s`, `m`, Ctrl+M, and Ctrl+S the same way. Its footer SHALL read `enter focus  h/l workspace  k/j app  n new  dd close  q quit` with no window carried, broken before `n new` on a terminal narrower than that line, and `SHIFT  h/l k/j carry  s put down  q cancel` while one is carried, and its stack SHALL read `No apps` on a workspace with no apps. Compositor layers SHALL keep their names: `TESSERACT_SPACE_WAIT_LAYER`, the wait for the host layer to go, and the `hyprctl layers -j` check SHALL be unchanged, and `tesseract space list` SHALL keep its `layers` field.

#### Scenario: The headline counts apps

- **GIVEN** the graphical navigator rests on workspace 3's front app, and workspace 3 has two apps
- **WHEN** the headline is drawn
- **THEN** it MUST read `WORKSPACE 3   ·   APP 1 / 2`
- **AND** it MUST NOT say `LAYER`

#### Scenario: A workspace with no apps

- **GIVEN** the graphical navigator rests on workspace 4, which has no apps
- **WHEN** the headline is drawn
- **THEN** it MUST read `WORKSPACE 4   ·   NO APPS`

#### Scenario: The footer at rest

- **GIVEN** the graphical navigator is open and no window is carried
- **WHEN** the footer is drawn
- **THEN** it MUST read `H L  WORKSPACE      K J  APP      ENTER  FOCUS      N  NEW      DD  CLOSE      Q  CLOSE`

#### Scenario: The footer while shifting

- **GIVEN** the graphical navigator rests on workspace 3's front app
- **WHEN** the operator presses `s`
- **THEN** the footer MUST read `SHIFT      H L  WORKSPACE      K J  APP      S  PUT DOWN      Q  CANCEL` until the carry ends
- **AND** it MUST NOT say `MOVE` or `LAYER`

#### Scenario: s picks up and s puts down

- **GIVEN** workspaces 1, 2, 3, and 4, and the graphical navigator rests on workspace 1's front app `0x1a`
- **WHEN** the operator presses `s`, then `h`, then `s`
- **THEN** `h` MUST carry `0x1a` round to workspace 4, and nothing MUST be dispatched before the second `s`
- **AND** the second `s` MUST hide the overlay, run `tesseract space move 4 0x1a`, and close it

#### Scenario: Escape cancels a shift

- **GIVEN** the graphical navigator was on workspace 3's second app when `s` picked it up, and it has since been carried to workspace 2
- **WHEN** the operator presses Escape
- **THEN** the picture MUST return to workspace 3's second app with every app in its old place, and the footer MUST stop saying `SHIFT`
- **AND** nothing MUST be dispatched

#### Scenario: Bare m is left to the host

- **GIVEN** a shell hosts the graphical navigator, which rests on an app
- **WHEN** the operator presses `m`, then `s`, then `m`
- **THEN** the first `m` MUST NOT pick up the app, and the second `m` MUST NOT put it down
- **AND** the navigator MUST leave each `m` unhandled for its host, and MUST NOT run a command or ask its host to close

#### Scenario: Ctrl+M confirms and never shifts

- **GIVEN** the graphical navigator rests on workspace 2's app `0x2b`
- **WHEN** the operator presses Ctrl+M
- **THEN** the satellite of `0x2b` MUST rush toward the eye, and the overlay MUST close and run `tesseract space focus 2 0x2b`
- **AND** no window MUST be picked up

#### Scenario: Ctrl+M while shifting does nothing

- **GIVEN** the graphical navigator carries a window, where it was picked up or in a new place
- **WHEN** the operator presses Ctrl+M
- **THEN** the window MUST stay carried where it is, and nothing MUST be dispatched

#### Scenario: Ctrl+S does not shift

- **GIVEN** the graphical navigator rests on an app, or carries a window
- **WHEN** the operator presses Ctrl+S
- **THEN** no window MUST be picked up or put down, and nothing MUST be dispatched

#### Scenario: The terminal footers

- **GIVEN** the terminal navigator rests on workspace 3's front app
- **WHEN** the operator presses `s`
- **THEN** the footer MUST change from `enter focus  h/l workspace  k/j app  n new  dd close  q quit` to `SHIFT  h/l k/j carry  s put down  q cancel`

#### Scenario: The terminal navigator shifts with s

- **GIVEN** the terminal navigator shows workspaces 1, 2, and 3 with workspace 1's front app `0x1a` selected
- **WHEN** the operator presses `m`, `s`, `h`, `m`, Enter, and `s`
- **THEN** `m` and Enter MUST change nothing, and nothing MUST be dispatched before the second `s`
- **AND** after the navigator exits, hyprctl MUST receive `dispatch 'hl.dsp.window.move({ workspace = "3", window = "address:0x1a" })'` and no other dispatch

#### Scenario: The terminal stack names apps

- **GIVEN** the terminal navigator rests on a workspace with no apps
- **WHEN** it draws the stack
- **THEN** the stack MUST read `No apps`

#### Scenario: Compositor layers keep their names

- **GIVEN** the overlay hides before a focus or a move
- **WHEN** the detached command runs with `TESSERACT_SPACE_WAIT_LAYER=1`
- **THEN** it MUST poll `hyprctl layers -j` until the host layer is gone, as before
- **AND** `tesseract space list` MUST still list each workspace's apps under `layers`
