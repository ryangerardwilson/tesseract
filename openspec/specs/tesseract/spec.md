# tesseract Specification

## Purpose

Define the public tesseract command: a tmux session browser whose snapshots persist through nebula.

## Requirements

### Requirement: Session snapshots persist through nebula

Tesseract SHALL store session snapshots by invoking nebula's `sql` command against the database `tesseract`. It MUST NOT open that database file itself.

#### Scenario: A snapshot is written

- **GIVEN** the `sql` command from nebula is on `PATH`
- **WHEN** tesseract saves a session snapshot
- **THEN** it MUST send that state through `sql`
- **AND** it MUST NOT write the sqlite file directly

### Requirement: km is the tesseract binary

The public command `km` SHALL invoke the tesseract binary for keyd sticky-keys actions. There MUST NOT be a separate km product.

#### Scenario: km reports the tesseract version

- **GIVEN** `km` is the installed tesseract binary
- **WHEN** the operator runs `km version`
- **THEN** it MUST print the tesseract version
