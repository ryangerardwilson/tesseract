## ADDED Requirements

### Requirement: Quickshell list reports unseen agent work

`tesseract quickshell list` SHALL report `agent_unseen` (`0` or `1`) on every session object. It MUST be `1` only when `agent_total` is above 0, `agent_working` is 0, and the session's saved `agent_settled_at` is later than its saved `agent_seen_at`. Each list poll MUST set `agent_seen_at` to the current unix second while the session is attached and leave it unchanged while detached. It MUST set `agent_settled_at` to the current unix second when the session goes from working to not working, MUST NOT move it on later idle polls, and MUST clear it when the session has no agent panes. Both values MUST live on the session row of the tesseract sqlite snapshot, and a snapshot rewrite MUST keep the values the list poll already recorded.

#### Scenario: Work finishes while detached

- **GIVEN** a detached session whose agent was working on the previous list poll
- **WHEN** a list poll sees no agent working in that session
- **THEN** `agent_settled_at` MUST become the poll time
- **AND** that session MUST report `agent_unseen` 1

#### Scenario: Work finishes while attached

- **GIVEN** an attached session whose agent was working on the previous list poll
- **WHEN** a list poll sees no agent working in that session
- **THEN** `agent_seen_at` MUST be at least `agent_settled_at`
- **AND** that session MUST report `agent_unseen` 0

#### Scenario: Attaching after the work settled

- **GIVEN** a session that reports `agent_unseen` 1
- **WHEN** the operator attaches it and the next list poll runs
- **THEN** that session MUST report `agent_unseen` 0

#### Scenario: Working wins

- **GIVEN** a session with an earlier `agent_settled_at` later than its `agent_seen_at`
- **WHEN** a list poll sees an agent working in that session
- **THEN** that session MUST report `agent_unseen` 0

#### Scenario: A snapshot rewrites the session rows

- **GIVEN** a list poll has recorded `agent_settled_at` and `agent_seen_at` for a session
- **WHEN** a snapshot rewrites the session table
- **THEN** both values MUST be unchanged afterwards
