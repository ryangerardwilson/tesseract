## 1. Read through c3po

- [x] 1.1 List running c3po screens with `c3po __session active` and drop the raw SQL
- [x] 1.2 Parse its rows; test parsing and process start matching

## 2. Check

- [x] 2.1 Run `GOWORK=off go test ./...`
- [x] 2.2 Validate with `openspec validate read-c3po-sessions-through-c3po --strict --no-interactive`
