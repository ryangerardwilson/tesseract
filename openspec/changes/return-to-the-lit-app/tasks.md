## 1. Spec

- [x] 1.1 `c3po ast exec -- openspec validate return-to-the-lit-app --strict --no-interactive`

## 2. Navigator

- [x] 2.1 `place()` after `leave()`; `restoreWorkspace` and `restoreAddress` read by `applyList`, with the nearest-workspace and front-app fallbacks and both step counts set before `arrive()`
- [x] 2.2 qmllint, and the restore driven offscreen with `qmltestrunner`, alone and through the launcher as host

## 3. Install

- [x] 3.1 Install from `/tmp` with `TM_INSTALL_ACTIVATE_SYSTEMD=0`; `tesseract-restore.service` stays inactive and the installed `SpaceNavigator.qml` matches the worktree
