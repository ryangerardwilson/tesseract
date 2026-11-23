## ADDED Requirements

### Requirement: The X axis is the normal workspaces in id order

tesseract SHALL read workspaces with `hyprctl workspaces -j` and SHALL place on the X axis exactly the workspaces whose `id` is a positive integer, in ascending `id` order, as one row: a lower id is to the left of a higher id. A `special:` workspace, or any workspace whose `id` is below 1, MUST NOT be on the axis. tesseract MUST NOT add a workspace that `hyprctl workspaces -j` does not report, and MUST NOT arrange workspaces in a grid or a second row. A reported workspace with no windows SHALL stay on the axis with no layers.

#### Scenario: Workspaces sit left to right by id

- **GIVEN** `hyprctl workspaces -j` reports workspace ids 3, -98 (`special:musica-hidden`), 1, and 2, in that order
- **WHEN** tesseract builds the space
- **THEN** the X axis MUST be workspaces 1, 2, 3, left to right
- **AND** the special workspace MUST NOT be on it

#### Scenario: An empty workspace keeps its place

- **GIVEN** `hyprctl workspaces -j` reports workspaces 1 and 4, and no window is on workspace 4
- **WHEN** tesseract builds the space
- **THEN** workspace 4 MUST be on the X axis after workspace 1 with no layers

#### Scenario: A workspace Hyprland does not report is not invented

- **GIVEN** a window reports `workspace.id` 7 and `hyprctl workspaces -j` does not report workspace 7
- **WHEN** tesseract builds the space
- **THEN** there MUST be no workspace 7 on the X axis
- **AND** that window MUST NOT be a layer

### Requirement: The Z axis orders one workspace's windows by focusHistoryID

tesseract SHALL read windows with `hyprctl clients -j`. A window SHALL be a layer of exactly one workspace, the one its `workspace.id` names; a pinned window MUST NOT be copied onto any other workspace. A window whose `mapped` is false or whose `hidden` is true MUST NOT be a layer. The layers of one workspace SHALL be ordered by the `focusHistoryID` field, ascending, compared only among the windows of that workspace: the window with the lowest value is the front layer, and each higher value sits one layer deeper. `focusHistoryID` counts recency across the whole desktop, so at most one workspace, the one holding the most recently focused window, has a front layer whose value is 0. A window with a negative `focusHistoryID` SHALL sit behind every window with a non-negative one, and windows with equal values SHALL keep their `hyprctl clients -j` order. A window's position or size (`at`, `size`) MUST NOT affect the order.

#### Scenario: The most recently focused window of a workspace is the front layer

- **GIVEN** workspace 3 holds a terminal with `focusHistoryID` 0 at `at` [640, 26] and a PDF viewer with `focusHistoryID` 2 at `at` [0, 26]
- **WHEN** tesseract builds the space
- **THEN** the terminal MUST be layer 0 of workspace 3 and the PDF viewer layer 1

#### Scenario: A background workspace's front layer is relative

- **GIVEN** the window with `focusHistoryID` 0 is on workspace 3, and workspace 1 holds windows with `focusHistoryID` 5 and 3
- **WHEN** tesseract builds the space
- **THEN** the window with `focusHistoryID` 3 MUST be the front layer of workspace 1 and the window with 5 the layer behind it

#### Scenario: Hidden and unmapped windows are not layers

- **GIVEN** workspace 1 holds a window with `hidden` true, a window with `mapped` false, and a window with both `mapped` true and `hidden` false
- **WHEN** tesseract builds the space
- **THEN** workspace 1 MUST have exactly one layer, the mapped window that is not hidden

#### Scenario: A pinned window stays on its own workspace

- **GIVEN** a pinned window reports `workspace.id` 2 and workspaces 1, 2, and 3 exist
- **WHEN** tesseract builds the space
- **THEN** that window MUST be a layer of workspace 2 only

### Requirement: tesseract space list prints the space as JSON

`tesseract space list` SHALL print one JSON object. Its `workspaces` array SHALL be the X axis in order; each workspace SHALL carry `id`, `name`, and `layers`, and `layers` SHALL be its Z axis front first, each layer carrying `address`, `class`, and `title`. The object SHALL carry `active`, the `id` that `hyprctl activeworkspace -j` reports, when that workspace is on the X axis, and SHALL omit it otherwise. Listing MUST only read Hyprland state: it MUST NOT run `hyprctl dispatch` or `hyprctl keyword`, and it MUST NOT start, restore, or attach a tmux session.

#### Scenario: Listing reads and prints

- **GIVEN** Hyprland reports workspaces 1, 2, and 3, the active workspace 3, and workspace 3 holds a terminal with `focusHistoryID` 0 and a PDF viewer with `focusHistoryID` 2
- **WHEN** the operator runs `tesseract space list`
- **THEN** it MUST print workspaces 1, 2, and 3 in that order with `active` 3, and workspace 3's layers MUST be the terminal and then the PDF viewer
- **AND** hyprctl MUST NOT receive `dispatch` or `keyword`

### Requirement: tesseract space focus focuses one layer

`tesseract space focus <workspace-id> <address>` SHALL accept only a positive integer `<workspace-id>` and an `<address>` of `0x` followed by hexadecimal digits. It SHALL confirm from a fresh `hyprctl clients -j` that `<address>` is a layer of workspace `<workspace-id>`. It SHALL then run `hyprctl dispatch 'hl.dsp.focus({ workspace = "<workspace-id>" })'` followed by `hyprctl dispatch 'hl.dsp.focus({ window = "address:<address>" })'`, and no other dispatch. A malformed argument, or an address that is not a layer of that workspace, MUST exit non-zero before any dispatch. A dispatch that exits non-zero or replies with anything other than `ok` MUST make the command exit non-zero, and a failed workspace dispatch MUST stop the window dispatch. `tesseract space focus` MUST NOT run `hyprctl dispatch focuswindow`, `movetoworkspacesilent`, or `hyprctl keyword`, and MUST NOT move any window to another workspace.

#### Scenario: Focusing a layer

- **GIVEN** window `0x3a` is a mapped, not hidden window on workspace 3
- **WHEN** the operator runs `tesseract space focus 3 0x3a`
- **THEN** hyprctl MUST receive `dispatch 'hl.dsp.focus({ workspace = "3" })'` and then `dispatch 'hl.dsp.focus({ window = "address:0x3a" })'`
- **AND** it MUST receive no other dispatch

#### Scenario: A stale layer is refused

- **GIVEN** window `0x3a` is now on workspace 2, or no longer exists
- **WHEN** the operator runs `tesseract space focus 3 0x3a`
- **THEN** the command MUST exit non-zero
- **AND** hyprctl MUST NOT receive any dispatch

#### Scenario: A malformed target is refused

- **GIVEN** any desktop
- **WHEN** the operator runs `tesseract space focus 0 0x3a`, `tesseract space focus 3 special`, or `tesseract space focus 3 '0x3a" })'`
- **THEN** each command MUST exit non-zero
- **AND** hyprctl MUST NOT receive any dispatch

#### Scenario: Hyprland refuses the window

- **GIVEN** window `0x3a` is a layer of workspace 3 and Hyprland replies `hl.focus: window not found` to the window dispatch
- **WHEN** the operator runs `tesseract space focus 3 0x3a`
- **THEN** the command MUST exit non-zero and report that reply

### Requirement: tesseract space move moves one window to a workspace

`tesseract space move <workspace-id> <address>` SHALL accept only a positive integer `<workspace-id>` and an `<address>` of `0x` followed by hexadecimal digits. It SHALL confirm from a fresh `hyprctl clients -j` that `<address>` is a layer of a workspace. When that layer is on another workspace, it SHALL run exactly one dispatch, `hyprctl dispatch 'hl.dsp.window.move({ workspace = "<workspace-id>", window = "address:<address>" })'`: the stock Hyprland bind `hl.dsp.window.move({ workspace = i })` with the window selector that `tesseract space focus` uses. When the layer is already on that workspace, it SHALL dispatch nothing. A malformed argument, or an address that is not a layer, MUST exit non-zero before any dispatch. A dispatch that exits non-zero or replies with anything other than `ok` MUST make the command exit non-zero. tesseract MUST NOT run `movetoworkspacesilent` or `hyprctl keyword`, and MUST NOT send a z-order dispatch, such as `hl.dsp.window.alter_zorder` or `hl.dsp.window.bring_to_top`, whose arguments are not documented under `/usr/share/hypr`.

#### Scenario: Moving a window

- **GIVEN** window `0x3b` is a mapped, not hidden window on workspace 3
- **WHEN** the operator runs `tesseract space move 1 0x3b`
- **THEN** hyprctl MUST receive `dispatch 'hl.dsp.window.move({ workspace = "1", window = "address:0x3b" })'`
- **AND** it MUST receive no other dispatch

#### Scenario: A window already on the workspace stays

- **GIVEN** window `0x3b` is a layer of workspace 3
- **WHEN** the operator runs `tesseract space move 3 0x3b`
- **THEN** the command MUST exit zero
- **AND** hyprctl MUST NOT receive any dispatch

#### Scenario: A missing window is refused

- **GIVEN** no window with address `0x99` is a layer of any workspace
- **WHEN** the operator runs `tesseract space move 1 0x99`
- **THEN** the command MUST exit non-zero
- **AND** hyprctl MUST NOT receive any dispatch

#### Scenario: A malformed move is refused

- **GIVEN** any desktop
- **WHEN** the operator runs `tesseract space move 0 0x3a`, `tesseract space move 3 special`, or `tesseract space move 3 '0x3a" })'`
- **THEN** each command MUST exit non-zero
- **AND** hyprctl MUST NOT receive any dispatch

### Requirement: tesseract open space opens the graphical navigator

`tesseract open space` SHALL open a Quickshell overlay that draws the X axis and the selected workspace's layers as the orbital system that "The graphical navigator draws the space as an orbital system" requires, each layer showing its class and title, and that moves as "The graphical navigator moves like an orbiting system" requires. The overlay SHALL start on the active workspace's front layer, or on the first workspace when the active one is not on the X axis. Left or `h` SHALL select the workspace to the left and Right or `l` the workspace to the right, at that workspace's front layer. Up or `k` SHALL select the next layer toward the front, one shell inward, and Down or `j` the next layer deeper, one shell outward. Both axes SHALL cycle: Left or `h` on the first workspace SHALL select the last workspace and Right or `l` on the last SHALL select the first, at that workspace's front layer, and Up or `k` on the front layer SHALL select the deepest layer and Down or `j` on the deepest layer SHALL select the front layer. With one workspace, or one layer, a move on that axis SHALL keep the current selection, and a workspace with no layers has no layer to move to. Enter SHALL carry the selected layer's satellite out of its shell toward the eye, then close the overlay and focus the selected workspace and layer; with no layer selected, Enter SHALL do nothing. `m` SHALL carry the selected layer's window as "The graphical navigator carries a window in move mode" requires. With no window carried, `q`, Escape, or a click SHALL close the overlay without focusing anything. The overlay SHALL get its data from `tesseract space list`, SHALL focus through `tesseract space focus`, and SHALL move a window through `tesseract space move`; it MUST NOT run or parse `hyprctl` itself. Opening the navigator, moving inside it, carrying a window, cancelling a carry, and dismissing it MUST NOT change the focused window, the focused workspace, or any window's workspace; Enter and the second `m` of move mode SHALL be the only changes.

#### Scenario: Moving chooses without focusing

- **GIVEN** the overlay is open on workspace 3's front layer, and workspace 2 has two layers
- **WHEN** the operator presses `h` and then `j`
- **THEN** workspace 2's second layer MUST be selected
- **AND** the overlay MUST NOT have run `tesseract space focus` or any `hyprctl dispatch`

#### Scenario: Moves cycle past either end

- **GIVEN** the overlay shows workspaces 1, 2, and 3, and workspace 2 has three layers
- **WHEN** the operator presses `h` on workspace 1, or `l` on workspace 3
- **THEN** `h` MUST select workspace 3 and `l` MUST select workspace 1, each at its front layer
- **AND** on workspace 2, `k` on the front layer MUST select the third layer, and `j` on the third layer MUST select the front layer

#### Scenario: One workspace or one layer stays put

- **GIVEN** the overlay shows one workspace with one layer
- **WHEN** the operator presses `h`, `l`, `k`, or `j`
- **THEN** the same workspace and layer MUST stay selected

#### Scenario: Enter focuses the chosen layer

- **GIVEN** the overlay has workspace 2's layer with address `0x2b` selected
- **WHEN** the operator presses Enter
- **THEN** the satellite of `0x2b` MUST rush out of its shell toward the eye
- **AND** only after that MUST the overlay close and run `tesseract space focus 2 0x2b`

#### Scenario: Dismissing leaves the desktop as it was

- **GIVEN** the overlay is open and the operator has moved the selection
- **WHEN** the operator presses `q` or Escape
- **THEN** the overlay MUST close without running `tesseract space focus`

### Requirement: The graphical navigator carries a window in move mode

`m` SHALL pick up the selected layer's window, the lit electron, and start move mode; with no layer selected, `m` SHALL do nothing. Until a second `m`, Hyprland MUST NOT change. While a window is carried, Left or `h` SHALL carry it to the workspace to the left and Right or `l` to the workspace to the right, each cycling past either end as a move on X does, and the orbit SHALL follow as it does for a move on X: the destination workspace SHALL become the nucleus, with the carried window's electron lit on its tight shell, at its front layer, and off the workspace it left. Down or `j` SHALL carry the window one shell deeper in the destination workspace and Up or `k` one shell toward the front, each cycling between the front place and the deepest; the other layers SHALL keep their order, and the carried window SHALL take the place it was carried to. Each carry SHALL ease the picture one step, including a carry that cycles past an end, as a move does. The carried electron SHALL stay on the tight shell and be brighter than a selected electron, and the footer SHALL say `MOVE` until the carry ends. Enter SHALL do nothing while a window is carried. Escape or `q` SHALL cancel the carry: the picture SHALL return to where it stood before the first `m`, and nothing SHALL be dispatched. A second `m` SHALL put the window down. Put down where it was picked up, nothing SHALL change. Put down in a new place, the overlay SHALL hide, run `tesseract space move <workspace-id> <address>` for the destination workspace, and quit. Hyprland documents no arguments for a z-order dispatcher under `/usr/share/hypr`, so when the window was put down behind another window, or in a new place on its own workspace, the overlay SHALL run `tesseract space focus <workspace-id> <address>` after the move, and the window SHALL land on top of that workspace.

#### Scenario: The first m only picks up

- **GIVEN** the overlay rests on workspace 3's front layer, a terminal
- **WHEN** the operator presses `m`
- **THEN** the terminal's electron MUST stay on the tight shell and grow brighter, and the footer MUST say `MOVE`
- **AND** the overlay MUST NOT have run `tesseract space move`, `tesseract space focus`, or any `hyprctl dispatch`

#### Scenario: Carrying past the first workspace

- **GIVEN** workspaces 1, 2, 3, and 4, and the overlay carries window `0x1a`, picked up from workspace 1's front layer
- **WHEN** the operator presses `h`
- **THEN** the orbit MUST turn one step back, and workspace 4 MUST become the nucleus with the electron of `0x1a` lit on its tight shell
- **AND** workspace 1 MUST no longer show `0x1a`
- **AND** nothing MUST be dispatched

#### Scenario: Carrying deeper keeps the other layers in order

- **GIVEN** the overlay carries a window to the front of a workspace whose layers are a terminal and then a PDF viewer
- **WHEN** the operator presses `j`, and then `j` again
- **THEN** the carried window MUST stand behind the terminal, and then behind the PDF viewer, while the terminal stays in front of the PDF viewer
- **AND** each step MUST ease the picture one step with the carried electron on the tight shell
- **AND** a third `j` MUST carry the window to the front again in one step

#### Scenario: The second m moves the window

- **GIVEN** the overlay carries window `0x1a` to the front of workspace 4
- **WHEN** the operator presses `m`
- **THEN** the overlay MUST hide and run `tesseract space move 4 0x1a`, and then close
- **AND** it MUST NOT run `tesseract space focus`

#### Scenario: Put down behind another window

- **GIVEN** the overlay carries window `0x3a` to workspace 2 and one shell deeper than workspace 2's front layer
- **WHEN** the operator presses `m`
- **THEN** the overlay MUST hide and run `tesseract space move 2 0x3a` and then `tesseract space focus 2 0x3a`, and then close

#### Scenario: Escape cancels the carry

- **GIVEN** the overlay was on workspace 3's second layer when `m` picked up its window, and the window has since been carried to workspace 2
- **WHEN** the operator presses Escape
- **THEN** the picture MUST return to workspace 3's second layer with every window in its old place, and the footer MUST stop saying `MOVE`
- **AND** nothing MUST be dispatched
- **AND** a second Escape MUST close the overlay without focusing anything

#### Scenario: Enter while carrying does nothing

- **GIVEN** the overlay carries a window
- **WHEN** the operator presses Enter
- **THEN** the window MUST stay carried and nothing MUST be dispatched

#### Scenario: m without a window does nothing

- **GIVEN** the overlay rests on a workspace with no layers
- **WHEN** the operator presses `m`
- **THEN** no window MUST be carried and the footer MUST NOT say `MOVE`

### Requirement: The graphical navigator draws the space as an orbital system

The overlay SHALL draw a black void with the selected workspace as the nucleus at its centre: a glowing glass sphere with the workspace's name on it and a faint spherical halo behind it. Every other X-axis workspace SHALL be a smaller glass sphere with its name on it, on one equatorial orbit around the nucleus, and that orbit SHALL be the X axis: one thin ring in perspective, seen from just above its plane so that it reads as left and right rather than as a flat diagram. The workspaces SHALL sit on the orbit by id, each one fixed step further round from the front than its neighbour, lower ids to the left of the nucleus and higher ids to the right, smaller and dimmer as the orbit turns them away. No workspace SHALL turn past the back of the orbit, so a lower id is never drawn round to the right of a higher one. The selected workspace's layers SHALL be electrons on concentric shells around the nucleus, in the orbit's plane: each shell a thin ellipse with one small satellite that carries its layer's class and title. The selected layer's shell SHALL be the tight, bright shell closest to the nucleus, and each deeper layer's shell SHALL be wider and dimmer than the one inside it; at most four shells SHALL show beyond the selected one. The shells of layers in front of the selection SHALL sink onto the nucleus as faint traces whose satellites do not show. The half of a shell behind the nucleus SHALL pass behind it and the half in front SHALL pass in front of it. The overlay SHALL draw its spheres and rings with Qt Quick items and shapes through one perspective projection and MUST NOT import Qt Quick 3D. It MUST NOT draw a second orbit, a polar or tilted ring of workspaces, a column, a grid, or a floor rail, and MUST NOT place a window by its pixel position. It SHALL use white glass on a near-black void with one accent, `#ffffff`, and SHALL show every workspace name, class, and title as plain text.

#### Scenario: The system at rest

- **GIVEN** workspaces 1, 2, 3, and 4, workspace 3 active, and workspace 3 holding a terminal as its front layer and a PDF viewer behind it
- **WHEN** the overlay has finished opening and no key is pressed
- **THEN** workspace 3 MUST be the nucleus at the centre with `3` on it, workspaces 1 and 2 MUST sit on the orbit to its left, and workspace 4 to its right, each smaller and dimmer than the nucleus
- **AND** the terminal's shell MUST be the tight, bright shell around the nucleus with a satellite showing the terminal's class and title, and the PDF viewer's shell MUST be wider and dimmer, with a satellite showing its own class and title

#### Scenario: A deep workspace keeps its depth

- **GIVEN** the selected workspace has twelve layers and its front layer is selected
- **WHEN** the overlay is at rest
- **THEN** the selected shell and at most four shells beyond it MUST show, each wider and dimmer than the one inside it

#### Scenario: The orbit keeps id order

- **GIVEN** workspaces 1 through 10 and workspace 1 selected
- **WHEN** the overlay is at rest
- **THEN** workspaces 2 through 10 MUST sit in id order round the right of the orbit, from the front toward the back
- **AND** none of them MUST be drawn round to the left of the nucleus

#### Scenario: A title is plain text

- **GIVEN** a window whose title is `<b>bold</b>`
- **WHEN** its satellite is drawn
- **THEN** the satellite MUST show the literal text `<b>bold</b>`

#### Scenario: No Qt Quick 3D

- **GIVEN** the overlay files that tesseract installs
- **WHEN** they are read
- **THEN** none of them MUST use `QtQuick3D`

### Requirement: The graphical navigator moves like an orbiting system

Opening the overlay SHALL settle the system out of a slow spin onto the starting workspace while the camera falls in toward the nucleus, and SHALL then unfurl the nucleus's shells out of it; the overlay MUST NOT fade in a still layout. A move on X SHALL turn the whole system about its vertical axis with a smooth ease of about 400 ms, never a slide along a line: the newly selected workspace swings round to the front and grows into the nucleus, the workspace left behind shrinks back onto its place on the orbit, every other workspace moves along the orbit, and the old nucleus's shells fold into it before the new nucleus's shells unfurl. A move on Z SHALL ease every shell's radius for about 380 ms: Down or `j` steps the selection outward to the next shell, which eases inward until it is the tight, bright shell while the shell that was selected sinks onto the nucleus, and Up or `k` steps the selection back inward, raising the shell in front of it out of the nucleus. A move that cycles past an end SHALL move the picture one step in the direction of its key, as any other move does: from the last workspace the orbit SHALL turn one step forward onto the first, from the first one step back onto the last, and a cycle between the front layer and the deepest SHALL ease the shells one step; such a move MUST NOT turn the orbit the long way round or ease through every shell. Only the selection cycles: the workspaces at rest keep their places on the orbit, lower ids to the left and higher ids to the right. Enter SHALL carry the selected satellite out of its shell toward the eye while the rest of the system clears, and only then SHALL the overlay hide and focus. Every move SHALL start from the picture as it is, so a key pressed during a move never makes the picture jump. Once the picture is at rest, the only motion SHALL be a slow swing of the selected electron along its shell, small enough that its title stays readable, and a slow breathing of the halo.

#### Scenario: Opening settles out of a spin

- **GIVEN** workspace 3 is active
- **WHEN** the overlay opens
- **THEN** the orbit MUST turn and come to rest with workspace 3 as the nucleus while the system grows toward the eye
- **AND** the overlay MUST NOT first show the system at rest and fade it in

#### Scenario: Moving right turns the orbit

- **GIVEN** the overlay rests on workspace 2 of workspaces 1, 2, and 3
- **WHEN** the operator presses `l`
- **THEN** over about 400 ms workspace 3 MUST swing round from the right of the orbit to the front and grow into the nucleus while workspace 2 shrinks back onto the orbit to the left
- **AND** workspace 2's shells MUST fold into it and workspace 3's shells MUST unfurl out of it

#### Scenario: Going deeper steps outward

- **GIVEN** the overlay rests on a workspace with three layers and its front layer selected
- **WHEN** the operator presses `j`
- **THEN** the second layer's shell MUST ease inward until it is the tight, bright shell
- **AND** the front layer's shell MUST sink onto the nucleus and its satellite MUST fade

#### Scenario: Coming back steps inward

- **GIVEN** the overlay rests on the second layer of a workspace with three layers
- **WHEN** the operator presses `k`
- **THEN** the front layer's shell MUST rise out of the nucleus and become the tight, bright shell again
- **AND** the second and third layers' shells MUST ease back out, wider and dimmer

#### Scenario: Cycling past the last workspace turns one step

- **GIVEN** the overlay rests on workspace 3 of workspaces 1, 2, and 3
- **WHEN** the operator presses `l`
- **THEN** over about 400 ms the orbit MUST turn one step forward, as a move from workspace 2 to workspace 3 does, and workspace 1 MUST swing in from the right and grow into the nucleus while workspace 3 shrinks back to the left
- **AND** the orbit MUST NOT turn the long way round through workspace 2
- **AND** at rest workspaces 2 and 3 MUST sit in id order to the right of workspace 1

#### Scenario: Cycling past the deepest layer steps once

- **GIVEN** the overlay rests on the third and deepest layer of a workspace with three layers
- **WHEN** the operator presses `j`
- **THEN** the front layer's shell MUST ease inward one step until it is the tight, bright shell while the third layer's shell sinks onto the nucleus
- **AND** the shells MUST NOT ease through the second layer's shell on the way

#### Scenario: A still overlay stays quiet

- **GIVEN** the overlay is open and at rest
- **WHEN** no key is pressed
- **THEN** only the selected electron's swing along its shell and the halo's breathing MUST move

### Requirement: One navigator component serves every host

The navigator's state, keys, and commands SHALL live in one component, `quickshell/space-navigator/SpaceNavigator.qml`, which draws only through `SpaceStage`. The overlay's `shell.qml` SHALL host that component in its panel and MUST NOT draw a stage or keep navigator state of its own. Another Quickshell shell MAY host the same component: the host SHALL name the tesseract binary whose space commands run, show the navigator by calling `open()`, pass it the keys, hide its own window when the navigator asks before a focus or a move, and close when the navigator has focused or moved, or when Escape or `q` is pressed with no window carried. The component MUST NOT hide a window, quit, or read its binary from the environment, and MUST NOT run `hyprctl`. A host with its own backdrop MAY clear the void; every host SHALL show the same picture and take the same keys.

#### Scenario: The overlay hosts the one component

- **GIVEN** the overlay files tesseract installs
- **WHEN** they are read
- **THEN** only `SpaceNavigator.qml` MUST instantiate `SpaceStage`
- **AND** `shell.qml` MUST instantiate `SpaceNavigator`

#### Scenario: A host hides before a focus

- **GIVEN** a shell hosts the navigator and the operator presses Enter on a layer
- **WHEN** the satellite has rushed
- **THEN** the navigator MUST ask its host to hide before it runs `tesseract space focus`
- **AND** it MUST tell its host once the focus has run

#### Scenario: Escape in a host

- **GIVEN** a shell hosts the navigator and no window is carried
- **WHEN** the operator presses Escape or `q`
- **THEN** the navigator MUST ask its host to close
- **AND** it MUST NOT run `tesseract space focus` or `tesseract space move`

### Requirement: The terminal navigator is the fallback

When the graphical navigator cannot run, because its files are not installed, `quickshell` is not on `PATH`, or Quickshell exits with an error, and both stdin and stdout are terminals, `tesseract open space` SHALL open a terminal navigator in the session browser's dense keyboard style. It SHALL show the same X axis and Z layers with class and title, start at the same place, take the same keys, and focus only on Enter, through the same check and dispatches as `tesseract space focus`, after the terminal navigator exits. It SHALL take the same move mode: the first `m` picks up the selected layer, `h`, `j`, `k`, and `l` carry it, its footer says `MOVE`, Escape or `q` cancels, and Enter does nothing while a window is carried. A second `m` in a new place SHALL exit the terminal navigator and then move the window through the same check and dispatch as `tesseract space move`, followed, where the overlay would focus it, by the same check and dispatches as `tesseract space focus`. Without a terminal, `tesseract open space` SHALL exit non-zero with the graphical navigator's error.

#### Scenario: Quickshell is missing in a terminal

- **GIVEN** `quickshell` is not on `PATH`, stdin and stdout are terminals, and Hyprland reports workspaces 1, 2, and 3 with 3 active
- **WHEN** the operator runs `tesseract open space`
- **THEN** the terminal navigator MUST open with workspace 3's front layer selected

#### Scenario: Terminal keys choose, Enter focuses

- **GIVEN** the terminal navigator has workspace 3's front layer selected
- **WHEN** the operator presses `h`, then `j`, then Enter
- **THEN** no focus MUST happen before Enter
- **AND** after the navigator exits, tesseract MUST focus workspace 2 and its second layer

#### Scenario: Terminal keys cycle

- **GIVEN** the terminal navigator shows workspaces 1, 2, and 3, and workspace 2 has three layers
- **WHEN** the operator presses `h` on workspace 1, `l` on workspace 3, `k` on workspace 2's front layer, or `j` on its third layer
- **THEN** it MUST select workspace 3, workspace 1, workspace 2's third layer, or workspace 2's front layer, in that order

#### Scenario: Terminal move mode

- **GIVEN** the terminal navigator shows workspaces 1, 2, and 3 with workspace 1's front layer `0x1a` selected
- **WHEN** the operator presses `m`, then `h`, then `m`
- **THEN** nothing MUST be dispatched before the second `m`
- **AND** after the navigator exits, hyprctl MUST receive `dispatch 'hl.dsp.window.move({ workspace = "3", window = "address:0x1a" })'` and no other dispatch

#### Scenario: Terminal move cancel

- **GIVEN** the terminal navigator has workspace 3's second layer selected
- **WHEN** the operator presses `m`, `h`, `j`, and then Escape
- **THEN** workspace 3's second layer MUST be selected again with every layer in its old place
- **AND** nothing MUST be dispatched

#### Scenario: Terminal dismiss

- **GIVEN** the terminal navigator is open
- **WHEN** the operator presses `q` or Escape
- **THEN** it MUST exit without any `hyprctl dispatch`

### Requirement: The usage string lists the space forms

The tesseract usage string SHALL list `tesseract open space`, `tesseract space list`, `tesseract space focus <workspace-id> <address>`, and `tesseract space move <workspace-id> <address>` alongside the existing forms, and every existing command SHALL keep its behavior.

#### Scenario: An invalid command shows the space forms

- **GIVEN** any desktop
- **WHEN** the operator runs `tesseract open nowhere`
- **THEN** it MUST exit non-zero with a usage error that lists `tesseract open space`, `tesseract space list`, `tesseract space focus <workspace-id> <address>`, and `tesseract space move <workspace-id> <address>`
