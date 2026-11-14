# Product Engineer Role

## Purpose

Own tesseract-specific facts that should not live in root generalists.

## Load Guidance

Load this file for `tesseract` implementation, CLI/TUI, installer, release, storage, configuration, or project-specific product work.

Root generalists own role behavior. This file owns only project facts and
repo-local operating constraints.

## Owns

- repo-local product and implementation facts
- CLI/TUI contract, command grammar, config, storage, and installer constraints
- release, upgrade, and verification expectations specific to this app

## Project Context

## Product Boundary
- `tesseract` owns tmux session restore and the managed keyd sticky-keys config. It stays a narrow utility.
- Keep one primary flow: `tesseract open picker` opens the graphical Quickshell radial session picker when Wayland/Quickshell is available.
- Keep the terminal browser available as `tesseract open browser` and as the fallback when the graphical picker cannot run.
- From inside the graphical picker:
  - `enter` / `ctrl+m` switches or attaches to the selected session
  - `n` creates a new named session and refreshes the ring in place
  - `x` kills the selected session and refreshes the ring in place
  - `,rn` renames the selected session and refreshes the ring in place
- From inside the terminal browser:
  - `enter` / `l` switches or attaches to the selected session
  - `n` creates a new named session
  - `r` renames the selected session
  - `d` kills the selected session
- Automatic restore may rebuild tmux layouts and reopen Codex or Grok Build panes, but it should stay focused on tmux session recovery rather than arbitrary process checkpointing.
- `tesseract km config`, `tesseract km apply`, and `tesseract km status` edit, install, and inspect `~/.config/km/keyd.config`. The `km` command is this same binary. Those actions must not start or restore tmux, and the config file stays on disk.
- Do not grow this into a broad session manager unless explicitly asked.

## Interface Rules
- Keep the default flow fast and shell-native.
- Bare `tesseract` should use the workspace default help behavior.
- `tesseract open picker` must open the Quickshell radial picker, then switch or attach to the selected session through the Go tmux boundary.
- `tesseract quickshell list` and `tesseract quickshell select <name>` are the stable API for externally configured Quickshell integrations; keep them focused on JSON session data and tmux selection, not visual styling.
- If the graphical picker is unavailable, `tesseract open picker` may fall back to the terminal browser when run from an interactive terminal or to the managed `index` session when invoked from a non-interactive tmux binding.
- `tesseract reload bindings` must remove tesseract-managed bindings from the live tmux server and then re-source `~/.config/tmux/tmux.conf`, which loads the ordered fragment directory.
- `tesseract open browser` and direct named-session flows must ensure the managed `index` session exists and that its `index` window is running `tesseract open browser`.
- Keep public command syntax declarative: `tesseract open picker`, `tesseract open browser`,
  `tesseract reload bindings`, `tesseract restore sessions`, `tesseract snapshot sessions`,
  `tesseract attach session <name>`, `tesseract quickshell list`,
  `tesseract quickshell select <name>`, `tesseract km config`,
  `tesseract km apply`, and `tesseract km status`.
- Do not reintroduce public `tesseract index`, `tesseract p`, `tesseract s`, `tesseract qs`, `tesseract reload`,
  `tesseract restore`, or `tesseract snapshot` forms.
- Terminal browser surfaces should stay dense, grayscale, and keyboard-first.
- Favor `j`/`k`, arrow keys, `n`, `r`, `d`, `/`, `?`, and `q`.
- Error text should stay short and explicit.

## Implementation Guardrails
- Keep the CLI/runtime in Go. Bubble Tea, Bubbles, and Lipgloss are allowed for the terminal browser and L1/L2/L3 component segregation.
- Keep tmux subprocess calls isolated in `internal/tmux`.
- Keep restore snapshots under `~/.tesseract/state/` and treat them as internal state, not a user-edited config surface.
- Preserve safe kill semantics by moving attached clients to another session before killing the target.
- If killing the final non-fallback session requires a temporary session, create it explicitly rather than guessing around tmux internals.
- Restored non-AI panes (or unsupported AI) should come back as shells in the saved working directory unless the app has an exact, low-risk resume primitive (`codex resume`, `grok --resume`).
- Keep tesseract narrow as an app, but let it own the managed tmux layer under `~/.config/tmux/` when the user wants tesseract-driven bindings and defaults. The entrypoint should stay a thin `tmux.conf` loader with ordered `tmux.conf.d/*.conf` fragments and helper scripts under `scripts/`.
- Preserve Omarchy compatibility by sourcing `~/.config/omarchy/current/theme/tmux.conf` from the managed tmux config instead of duplicating theme values in the repo.

## Release Contract
- Keep `tesseract` aligned with the workspace release contract.
- `install.sh` installs the tagged Go binary into `~/.tesseract/bin/tesseract`, writes embedded app assets into `~/.tesseract/app`, and publishes the user-facing command at `~/.local/bin/tesseract`. It also publishes `~/.local/bin/km` as a name for that binary when the current `km` path is missing, a symlink, or a launcher this installer wrote.
- `install.sh` owns the managed tmux config under `~/.config/tmux/` and should regenerate the entrypoint, ordered fragments, and helper scripts from embedded `tmux/` assets on every install or upgrade.
- `install.sh` should remove tesseract-managed legacy files such as `~/.config/tmux/tm.conf`, `~/.tmux/tm.conf`, and tesseract-managed `~/.tmux.conf` shims so Omarchy and fresh tmux reloads converge on the same config path.
- Do not hand-edit the managed tmux config except for short-lived local testing. After testing, move the change into `~/OpenSource/tesseract/`, then use the app install or release-upgrade path so `~/.config/tmux/` is regenerated from the app.
- Tagged builds stamp `internal/version.Version` with Go ldflags; source checkouts keep the runtime version at `0.0.0`.
