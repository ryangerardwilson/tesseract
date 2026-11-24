## Context

The navigator calls a window on a shell a layer. The overlay's headline says `LAYER 1 / 2` or `NO WINDOWS`, its footers say `K J  LAYER`, and the terminal navigator's footer says `k/j layer`. `m` picks up a window, a second `m` puts it down, and the footer says `MOVE` in between. Hyprland calls a layer-shell surface a layer, and tesseract waits through `hyprctl layers -j` for the navigator's own surface to go before it focuses. The Super+T launcher in the home repository hosts `SpaceNavigator.qml`, passes it its keys, and now opens Musica on bare `m`. The `space` capability exists only in the open change `add-space-navigator`; `openspec/specs/` has no `space` spec.

## Goals / Non-Goals

**Goals:**

- One word, app, for a window on a shell wherever the operator reads a navigator.
- `s` shifts a window in both navigators by the rules `m` had.
- Bare `m` left to the host.

**Non-Goals:**

- Renaming identifiers, the JSON field `layers`, the error messages, or anything about compositor layers.
- Binding Musica. That is the host's code, in the home repository.

## Decisions

**The delta is ADDED, not MODIFIED.** `space` is not a living spec: the open change `add-space-navigator` adds it and has not been archived. A MODIFIED block replaces a requirement in `openspec/specs/space/spec.md`, and there is none to replace. This change adds one requirement for the words and the key, and edits `add-space-navigator` in place so the two changes do not contradict each other. Alternatives rejected:
- MODIFIED blocks against `add-space-navigator`'s requirements: archive has no living requirement to apply them to.
- Waiting for `add-space-navigator` to be archived and then modifying it: the code and the open change would disagree until then, and archive refuses a MODIFIED block that renames a scenario, so each renamed scenario title would need a patch to the living spec afterwards.

**`s` shifts, and Ctrl+M stays Enter.** The overlay's key handler keeps its order: Escape or `q` cancels, then Return, Enter, or Ctrl+M confirms, then `s` without Control carries. Ctrl+M is how Enter arrives, so it never reaches the carry. Bare `m` matches no key, so the navigator leaves the event unaccepted and the host can take it. Ctrl+S is not `s`: the overlay's check excludes Control, and Bubble Tea reports it as `ctrl+s`. The terminal navigator takes `s` where it took `m` and keeps `enter` and `ctrl+m` as confirm. The mode word follows the key: `SHIFT` for `s`, as `MOVE` was for `m`. `tesseract space move` keeps its name, because it is a command, not a key. Alternatives rejected:
- Keeping `m` and letting the host take it first: the footer would say `M  PUT DOWN` while the host opens Musica on the same key.
- A chord such as Ctrl+S: every other navigator key is one letter.

**App is the operator's word; layer stays the compositor's.** The headline, the footers, the empty stack, the README, `tesseract help`, and the specs say app. The headline counts apps, so an empty workspace says `NO APPS`, not `NO WINDOWS`. Identifiers keep layer (`chosenLayer`, `layerSteps`, `space.Layer`, `RenderLayerStack`), as do the JSON field `layers`, the error `no layer <address> on workspace <id>`, `TESSERACT_SPACE_WAIT_LAYER`, and every mention of a layer surface. Alternatives rejected:
- Renaming the JSON field to `apps`: it breaks every reader of `tesseract space list` and changes nothing in the picture.
- Renaming the identifiers: a wide diff the operator never sees.

## Risks / Trade-offs

- [The operator presses `m` from habit] → in the overlay nothing happens; in the launcher Musica opens. The README lists `s`, and the carrying footer says `S  PUT DOWN`.
- [Two open changes add to `space`] → their requirement names differ, so archive takes them in either order. Archiving `add-space-navigator` first keeps its requirements ahead of this one.
- [Bubble Tea reports Ctrl+M in a terminal as `enter`] → the terminal navigator confirms on both and carries on neither, so it cannot tell them apart and does not need to.
