## Context

`tesseract space focus` and `tesseract space move` already check Hyprland afresh, wait for the host layer when `TESSERACT_SPACE_WAIT_LAYER=1`, and send one Lua dispatch each. The graphical navigator starts them with `setsid -f` after asking its host to hide. The host closes on `finished()`, and the Super+T launcher keeps the space reopen mark there. Hyprland 0.56 runs with the legacy parser off, so every dispatch is one `hl.dsp` expression.

## Decisions

**New is a focus.** Hyprland creates a workspace when it is focused. `space new` reads `hyprctl workspaces -j`, takes the highest positive id plus one, and sends `hl.dsp.focus({ workspace = "<id>" })`. Gaps below the highest id are not reused, because the operator asked for one past the highest. No window moves, so no move dispatch exists in this path.

**Close is the graceful request, once per app.** `space close` lists the space as `space list` does, at the moment it runs, and sends `hl.dsp.window.close({ window = "address:<address>" })` for each app on that workspace. The address selector is the one `focus` and `move` use. A failed close is reported, but the other closes still go out, and nothing is retried or killed. Hidden and unmapped windows are not apps, so they are not closed. Special workspaces and layer-shell surfaces are never on the X axis or among the clients, so they are out of reach.

**The active workspace is left only when it is empty.** If the closed workspace was the active one, Hyprland keeps an empty active workspace. `space close` reads `hyprctl clients -j` every 100 ms for up to about 1.5 s. When no mapped window is left on the workspace, it focuses the next lower id on the X axis, or else the next higher, and Hyprland drops the empty one. When a window is still mapped there (a save dialog, or an app that ignored the close), focus stays on it. A background workspace needs nothing more: Hyprland drops it once its last window goes.

**The menu is gone before anything closes.** `n` and `dd` take the confirm path: the navigator sets `confirming`, asks the host to hide, and starts `space new` or `space close <id>` with `setsid -f` and `TESSERACT_SPACE_WAIT_LAYER=1`. The host closes on `finished()`. The command waits until the host layer is unmapped, so a save dialog an app raises can take the keyboard. Before the host hides, the navigator updates its own picture in the same turn, so no frame shows a half step: `n` adds the new workspace and selects it, and `dd` drops the closed workspace and selects its neighbour.

**`dd` must be meant.** The first bare `d` arms a 400 ms window, and a second bare `d` inside it closes. Any other key disarms first and then does its own job. Ctrl+D, Alt+D, and Shift+D are not bare. An auto-repeated `d` is ignored, so holding the key never closes. `n` and `d` do nothing while an app is carried, and `dd` does nothing when only one workspace is on the X axis, which `space close` also refuses. The terminal navigator keeps the time of the first `d` and compares it with the second, with a clock that tests can set.

**Hosts need no change.** The launcher passes every key but Tab and bare `m` to the navigator only while the space shows, so `n` and `dd` cannot reach the tmux radial, Musica, or the other views, where `n` keeps its own meaning. `finished()` already keeps the reopen mark, so the next Super+T opens on the space navigator after `n` or `dd`.

## Risks / Trade-offs

- An app may take longer than 1.5 s to close a confirmed active workspace. Focus then stays on that workspace, which is the holdout rule, and the operator can move on themselves.
- A new workspace that the operator leaves empty is dropped by Hyprland as usual.
- The `hl.dsp.window.close` argument shape is not documented in `/usr/share/hypr`. It mirrors the documented window selector of `focus` and `move`, and it is checked without dispatching against the live compositor.
