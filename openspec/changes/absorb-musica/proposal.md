## Why

The player still lives in its own checkout, while the Super+T Musica center already belongs to this harness. One binary should answer as `musica`, the way it already answers as `km`.

## What Changes

- Move the player into this module under `internal/musica` and `player`.
- `musica` and `tesseract musica` run that player. `musica version` prints the tesseract version.
- Install `musica` as a symlink to the tesseract launcher.
- `musica upgrade` runs the tesseract installer.
- A re-exec of `serve` or `download-worker` keeps the invocation name `musica`.

## Capabilities

### Added Capabilities

- `musica`: the player command, its center, playback, and downloads.

### Modified Capabilities

- `tesseract`: `musica` is this binary. There is no separate musica product.

## Non-goals

- Changing playback, the cassette menu, or the download rules.
- Moving the Super+T launcher QML into this repository.
- Deleting the public GitHub history of the old musica repository from this change.

## Impact

Install and upgrade come from this checkout. Rollback is the previous tesseract release, which has no `musica` command.
