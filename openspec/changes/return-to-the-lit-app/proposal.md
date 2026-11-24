## Why

A host makes the navigator afresh each time it shows it. `applyList` then puts the selection on the active workspace's front app every time. In the Super+T launcher, Tab out of the space and Tab back lands there instead of on the workspace and app the operator had lit.

## What Changes

- `place()` reports where the selection rests: the workspace id and the lit app's address, empty on a workspace with no apps.
- A host may set `restoreWorkspace` and `restoreAddress` before `open()`. The navigator then opens on that place, or on the nearest remaining workspace (the next lower id, or else the next higher) and that workspace's front app when the place is gone. It sets `columnSteps` and `layerSteps` to the place before the picture arrives.
- With no place set, the navigator opens on the active workspace's front app as before, so the `tesseract open space` overlay is unchanged.

## Non-goals

- The navigator does not remember anything itself and writes no file. The host keeps the place for as long as it lives.
- No change to h, j, k, l and their wrap, `s`, `n`, `dd`, Enter, Ctrl+M, or bare `m`.

## Capabilities

### Modified Capabilities

- `space`: a host can bring the navigator back to the place it left. `space` is not a living spec yet, so this change ADDs one requirement.

## Impact

`quickshell/space-navigator/SpaceNavigator.qml`. The launcher that uses it lives in the home repository.
