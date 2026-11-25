## ADDED Requirements

### Requirement: musica is the tesseract binary

The public command `musica` SHALL invoke the tesseract binary for the player. `tesseract musica` SHALL run that same player. There MUST NOT be a separate musica product or checkout.

#### Scenario: musica reports the tesseract version

- **GIVEN** `musica` is the installed tesseract binary
- **WHEN** the operator runs `musica version`
- **THEN** it MUST print the tesseract version

#### Scenario: The player stays in this module

- **GIVEN** the operator runs `musica serve` or `tesseract musica serve`
- **WHEN** the player command runs
- **THEN** it MUST run from the tesseract binary
- **AND** `github.com/ryangerardwilson/tesseract/player` MUST be the public `Run` package
