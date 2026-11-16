# tesseract

`tesseract` is a small tmux session browser. Session snapshots live in its own sqlite. [nebula](https://github.com/ryangerardwilson/nebula) `s3` backs that file up.

This replaces shell-function glue and custom tmux chooser bindings with a real terminal app.

## Install

Install the latest tagged release with:

```bash
curl -fsSL https://raw.githubusercontent.com/ryangerardwilson/tesseract/main/install.sh | bash
```

The installer keeps app state under `~/.tesseract`, publishes the user-facing command
at `~/.local/bin/tesseract`, and regenerates the repo-managed tmux config under
`~/.config/tmux/` on every install or upgrade.

If `~/.local/bin` is not already on your `PATH`, add it once to `~/.bashrc`
and reload your shell:

```bash
export PATH="$HOME/.local/bin:$PATH"
source ~/.bashrc
```

The public launcher is `~/.local/bin/tesseract`. The internal runtime still lives
under `~/.tesseract/bin/tesseract`, with the release bundle in `~/.tesseract/app`.

`tesseract` also owns the keyd sticky-keys config. The editable file stays
`~/.config/km/keyd.config`. `tesseract km config`, `tesseract km apply`, and
`tesseract km status` edit, install, and inspect it. The `km` command is the
same installed binary.

Installer shortcuts:

- `./install.sh help`: installer help
- `./install.sh version`: print the latest release version
- `./install.sh version 0.1.0`: install a specific release
- `./install.sh upgrade`: upgrade to the latest release if newer
- `./install.sh upgrade tmux key F8`: override the tmux index shortcut if you want a different key

The installer never edits shell startup files automatically. If `~/.local/bin`
is already on your `PATH`, no shell change is needed.

`tesseract` owns the managed tmux layer under `~/.config/tmux/`: a thin
`tmux.conf` loader, ordered `tmux.conf.d/*.conf` fragments, and helper scripts
under `scripts/`. Persistent tmux config changes should be made in the `tesseract`
repo and then installed or upgraded through `tesseract`, not hand-edited in
`~/.config/tmux/`.

`tesseract` does not depend on `~/.tmux.conf`. During install or upgrade it removes
tesseract-managed legacy shims such as `~/.tmux.conf`, `~/.tmux/tm.conf`, and
`~/.config/tmux/tm.conf` so Omarchy and tmux both converge on the same
`~/.config/tmux/tmux.conf` entrypoint.

The managed config keeps the session-picker shortcut repo-owned instead of
hand-maintained. That shortcut runs `tesseract open picker`, which opens the Quickshell
radial session picker when a graphical Wayland session is available. The
default shortcut key is `M-i`. You can still override it with
`tmux key <key>` if you want a different tmux key name.

The managed prefix-`q` reload now runs `tesseract reload bindings`, which first unbinds the
tesseract-managed key set from the live tmux server and then re-sources
`~/.config/tmux/tmux.conf`. That entrypoint then sources the ordered fragments
under `~/.config/tmux/tmux.conf.d/`. Use prefix-`q` after upgrading `tesseract` if
your current tmux server was started with an older binding set.

Alt+Enter equalizes the splits already open in every window of the current
session. A window that is already one horizontal row keeps that arrangement
and its panes are spread evenly. Any other layout becomes one even horizontal
row, without adding or removing panes. A single pane stays as it is.
Alt+2 through Alt+9 still add panes up to that count and lay them out in one
even row.

The managed tmux config keeps the session-picker shortcut and the pane, window,
session, copy-mode, and status-bar setup repo-owned in separate ordered
fragments while still sourcing the active Omarchy theme from
`~/.config/omarchy/current/theme/tmux.conf`.

The installer writes and enables `tesseract-restore.service` and
`tesseract-snapshot.timer` under `~/.config/systemd/user/`. Login restore runs once
from the service, while the timer invokes the one-shot `tesseract snapshot sessions`
command every 15 minutes. Looptab remains available for general automation but
does not schedule `tesseract` snapshots.

The snapshot command does not overwrite the saved state when there are no live
non-index tmux sessions, so a fresh login cannot erase the last useful restore
snapshot before restore has a chance to run.

## Usage

```bash
tesseract
tesseract help
tesseract open picker
tesseract open browser
tesseract quickshell list
tesseract quickshell select root
tesseract reload bindings
tesseract restore sessions
tesseract snapshot sessions
tesseract attach session root
tesseract version
tesseract upgrade
```

`tesseract version` prints the installed runtime version. Source checkouts keep the
Go runtime version at `0.0.0`; tagged release artifacts stamp the real release
version during the release workflow.

### Radial Session Picker

```bash
tesseract open picker
```

`tesseract open picker` opens a transparent Quickshell radial picker for live tmux sessions.
Choosing a session switches your current tmux client to it from inside tmux or
attaches to it from outside tmux. Bare `tesseract` shows help.
The picker shows up to 30 matching sessions; at higher counts it switches to
smaller tiles and hides the outer labels so the ring stays readable.

Core actions:

- `j` / `k`, `h` / `l`, tab, or arrow keys: rotate selection
- `enter`, `ctrl+m`, or space: switch to the selected session
- `1` through `9`: switch directly to a visible session
- `/`: type a starts-with filter, then `enter` or `ctrl+m` to apply it
- `n`: create a named session
- `x`: kill the selected session
- `,rn`: rename the selected session
- `esc`: clear the filter or close the picker

Sessions with working AI panes show an animated orbit on their tile. Sessions
with idle AI panes show a quieter static marker.
Sessions are placed clockwise in tmux's current last-opened/last-attached
order.

If Quickshell is unavailable, `tesseract open picker` falls back to the terminal browser
from an interactive terminal or to the managed `index` session from a
non-interactive tmux binding.

### External Quickshell API

```bash
tesseract quickshell list
tesseract quickshell select writing
```

`tesseract quickshell` is the stable integration point for user-owned Quickshell
config. It exposes session data and session selection only; create, delete, and
rename remain hidden picker internals so the public API stays small.
`select` switches existing tmux clients to the chosen session and creates that
session first if it is missing.

Payload shape:

```json
{
  "sessions": [
    {
      "name": "work",
      "attached": 1,
      "windows": 2,
      "last_attached": 1710000000,
      "agent_total": 1,
      "agent_working": 1,
      "agent_idle": 0
    }
  ],
  "preferred": "work"
}
```

`preferred` is present when an action should keep or move focus to a specific
session.

### Terminal Browser

```bash
tesseract open browser
```

`tesseract open browser` opens the persistent terminal session browser.

The installed systemd timer writes an automatic restore snapshot every 15 minutes.
The persistent terminal browser also writes a snapshot in the background.
Direct named-session flows and `tesseract open browser` also ensure the managed
`index` session exists and that its `index` window is running `tesseract open browser`.

Core actions:

- `j` / `k` or arrow keys: move
- `l`: switch to the selected session
- `m`: mark or unmark the current session
- `v`: toggle visual selection for a contiguous range
- `x`: kill the current session, marked sessions, or the visual selection
- `n`: create a new named session
- `,rn`: rename the current session
- `?`: toggle help
- `q`: quit the picker

Outside tmux, `l` attaches to the selected session. Inside tmux, it switches the current client to that session.
In the managed `index` session, the picker stays running in its original tmux pane after a switch and refreshes the session list, so ordering is updated when you jump back to it later.
Sessions with actively working AI panes (Codex or Grok Build) show the same compact animated pulse used in `loc`.

## Automatic Restore

If a saved snapshot exists and there are no live non-index tmux sessions, `tesseract` restores the snapshot automatically on the next `tesseract` startup.
The installed `tesseract-restore.service` runs the same restore path after the
graphical session environment is ready, before the first delayed snapshot.

Manual commands:

```bash
tesseract restore sessions
tesseract snapshot sessions
```

`tesseract restore sessions` restores missing saved sessions and skips names that
already exist. `tesseract snapshot sessions` saves the current non-index tmux sessions
only when at least one exists. By default, snapshot and restore close or skip
detached non-index sessions that have not been accessed for more than 10 days.
Attached sessions are always protected, and last-access time persists across
tmux server restarts so restoring a session does not reset its age.

If a saved AI pane (Codex or Grok) cannot be resumed, restore falls back to a
shell in the saved working directory and continues restoring the remaining
sessions.

Restore behavior:

- recreates tmux sessions, windows, and pane splits from the latest snapshot
- resumes ordinary Codex panes with `codex resume <thread_id>`
- resumes GLM-via-Codex panes with `codex --profile zai resume <thread_id>`
- resumes Grok panes with `grok --resume <session-id>` (Grok Build TUI sessions in `~/.grok/sessions/`)
- reopens non-AI panes as shells in the saved working directory

This restores tmux layout and AI conversations (Codex + Grok Build). It does not
checkpoint arbitrary in-memory process state mid-flight. Status detection,
Quickshell picker orbits, and the omarchy bar now include both.

### Direct Session

```bash
tesseract attach session root
```

If `root` already exists, `tesseract` attaches to it outside tmux or switches the current client to it inside tmux. If it does not exist yet, `tesseract` creates `root` first and then takes you there.

When killing sessions, `tesseract` moves attached clients off the target session(s) before killing them. If there is no other session available, it creates a temporary fallback session so the kill can complete safely.
