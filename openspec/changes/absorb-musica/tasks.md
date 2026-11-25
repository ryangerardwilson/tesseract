## 1. Player

- [x] 1.1 Move the player packages into this module and answer `musica` from the tesseract binary
- [x] 1.2 Install `musica` as a symlink to the tesseract launcher

## 2. Verification

- [x] 2.1 `c3po ast exec -- npm exec -- openspec validate absorb-musica --strict --no-interactive`
- [x] 2.2 `GOWORK=off go test ./...`
