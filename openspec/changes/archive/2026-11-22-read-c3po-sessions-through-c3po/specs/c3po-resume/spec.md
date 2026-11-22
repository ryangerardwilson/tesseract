## ADDED Requirements

### Requirement: A c3po screen's session is read through c3po
tesseract SHALL list running c3po screens by running `c3po __session active` and SHALL NOT read c3po's database. A pane SHALL take the session id, screen, and workspace of the row whose process id is the pane's process and whose start time matches when both are known. A pane with no matching row SHALL fall back to its `--resume` argument. When the command fails or prints no rows, no row SHALL match.

#### Scenario: A running screen restores its session
- **GIVEN** `c3po __session active` prints a row for process 7 with screen `o` and workspace `/work`
- **WHEN** tesseract resolves a pane whose process is 7 and whose start time matches
- **THEN** it SHALL restore that session id with screen `o` in `/work`

#### Scenario: A stale process start does not match
- **GIVEN** a row for process 7 whose start time differs from the running process 7
- **WHEN** tesseract resolves that pane
- **THEN** no row SHALL match
