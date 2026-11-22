## Why

tesseract restores a c3po or r2d2 screen by matching a pane's process to c3po's running-screen row. It read that row by running raw SQL against c3po's `record` table, so it depended on c3po's storage layout, and on copies of the session id, screen, and workspace that c3po kept inside the active row. c3po now keeps sessions in its own tables, keeps only the process in the active row, and exposes `c3po __session active` as the one interface for reading running screens.

## What Changes

- tesseract lists running c3po screens with `c3po __session active`. That command prints the session id, process id, process start time, screen, and workspace of each screen.
- tesseract no longer runs `sql c3po query` and no longer reads c3po tables.
- Restoring a pane does not change otherwise: the process id and start time still pick the row, and a pane with no row still falls back to its `--resume` argument.

Non-goals: no change to snapshots or to other agent resume paths.

## Capabilities

### New Capabilities

- `c3po-resume`: tesseract finds a running c3po screen's session through c3po.

### Modified Capabilities

None.

## Impact

- Code: `internal/tmux/api.go`, `internal/tmux/c3po_resume_test.go`.
- Needs a c3po with `c3po __session`. With an older c3po, the command fails, no row matches, and a pane falls back to its `--resume` argument.
