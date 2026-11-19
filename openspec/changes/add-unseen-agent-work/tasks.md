## Tasks

- [x] Add `agent_was_working`, `agent_settled_at`, and `agent_seen_at` to the session model, snapshot INSERT, and snapshot read
- [x] Keep the agent columns across `replaceSnapshot` inside its transaction
- [x] Record agent activity on every quickshell payload and report `agent_unseen`
- [x] Cover the transitions, the snapshot rewrite, and the list JSON with table-driven tests (`GOWORK=off go test ./internal/snapshot ./internal/quickshell`)
- [x] Run the whole module (`GOWORK=off go test ./...`, `GOWORK=off go vet ./...`)
- [x] Show unseen work on the session picker tile, with the working spin first (`qmllint`)
- [x] Validate with `npm exec -- openspec validate add-unseen-agent-work --strict --no-interactive`
