## ADDED Requirements

### Requirement: A host can bring the navigator back to the place it left

The navigator SHALL report where its selection rests through `place()`. The result SHALL be the selected workspace's id and the lit app's address. The address SHALL be empty on a workspace with no apps, and the workspace SHALL be 0 before the space is read. A host SHALL read `place()` only after `leave()` has returned true. A carried app is then back where it was picked up, and the place is that one.

A host MAY set `restoreWorkspace` and `restoreAddress` before `open()`. When the host does, the navigator SHALL open on that workspace and that app. When that workspace is gone, the navigator SHALL open on the nearest remaining workspace on the X axis: the next lower id, or else the next higher. When that address is not an app of the workspace it opens on, it SHALL light that workspace's front app. The navigator SHALL set the column, the app, `columnSteps`, and `layerSteps` to that place before the picture arrives, so the orbit settles onto it and does not turn in from the active workspace.

With no place set, the navigator SHALL open on the active workspace's front app, or on the first workspace when the active one is not on the X axis. Moves after opening SHALL start from the place it opened on, with their usual wrap.

#### Scenario: Back to a remembered app

- **GIVEN** a host set `restoreWorkspace` 2 and `restoreAddress` `0x2b`, and workspace 2 lists `0x2a` and then `0x2b`
- **WHEN** the navigator opens on workspaces 1, 2, 3, and 4 with workspace 3 active
- **THEN** workspace 2 and its app `0x2b` MUST be lit
- **AND** `columnSteps` MUST be 1 and `layerSteps` 1

#### Scenario: A remembered workspace that is gone

- **GIVEN** a host set `restoreWorkspace` 3
- **WHEN** the navigator opens on workspaces 1, 2, and 4, or on workspaces 4 and 7
- **THEN** it MUST light workspace 2's front app, or workspace 4's front app

#### Scenario: A remembered app that is gone

- **GIVEN** a host set `restoreWorkspace` 2 and `restoreAddress` `0x2c`, and workspace 2 lists `0x2a` and `0x2b`
- **WHEN** the navigator opens
- **THEN** workspace 2's front app `0x2a` MUST be lit

#### Scenario: Nothing remembered

- **GIVEN** no host set a place
- **WHEN** the navigator opens with workspace 3 active
- **THEN** workspace 3's front app MUST be lit

#### Scenario: The place after a carry

- **GIVEN** the navigator carries `0x3a`, picked up from workspace 3's front, to workspace 2
- **WHEN** the host calls `leave()` and then `place()`
- **THEN** the place MUST be workspace 3 and `0x3a`

#### Scenario: Moving from a remembered place

- **GIVEN** the navigator opened on workspace 2's app `0x2b` of workspaces 1, 2, 3, and 4
- **WHEN** the operator presses `l`, and then `h` and `k`
- **THEN** `l` MUST light workspace 3's front app
- **AND** `h` and `k` MUST light workspace 2 and then its deepest app
