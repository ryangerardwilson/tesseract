# Report unseen agent work to the quickshell radial

## Why
An agent can finish in a tmux session the operator has not looked at since. `tesseract quickshell list` reports how many agents are working and idle, but not whether the operator saw the session after the work stopped, so the radial cannot tell a finished background run from one already read.

## What Changes
- Each session row in the tesseract sqlite snapshot keeps `agent_settled_at` and `agent_seen_at` (unix seconds) next to `last_accessed`, plus `agent_was_working`, the previous poll's working state that marks the working-to-idle edge.
- Every `quickshell list` poll sets `agent_seen_at` to now while the session is attached, sets `agent_settled_at` to now when the session goes from working to not working, and clears `agent_settled_at` when the session has no agent panes.
- Each session in the quickshell JSON gains `agent_unseen` (`0` or `1`): `1` only when `agent_total > 0`, `agent_working == 0`, and `agent_settled_at > agent_seen_at`.
- A snapshot rewrite keeps the agent columns the list poll already wrote.
- The session picker tile shows unseen work and keeps the working spin first.

## Non-goals
- Opening the radial, snapshotting, or restoring is not a visit.
- A session with no snapshot row yet is not tracked until the next snapshot writes its row.
- The list poll does not publish the database to s3. The next snapshot carries the columns.

## Impact
Capability: tesseract. Files: `internal/snapshot/agent.go`, `internal/snapshot/store.go`, `internal/snapshot/snapshot.go`, `internal/quickshell/quickshell.go`, `quickshell/session-picker/OrbitTile.qml`, `quickshell/session-picker/shell.qml`. Existing databases gain the three columns through the model migration on open. `agent_total`, `agent_working`, `agent_idle`, and `last_attached` are unchanged.
