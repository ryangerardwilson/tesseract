package snapshot

import (
	"fmt"
	"strings"
)

// The saved snapshot is one row: its format version and when it was taken.
const savedSnapshotModel = "model saved_snapshot\n" +
	"  id integer primary key\n" +
	"  version integer not null\n" +
	"  captured_at integer not null\n" +
	"  check (id = 1)\n"

// A saved session keeps its row, and so its id, across snapshots while tmux
// keeps its name. last_accessed is null when tmux never reported a use, and
// retention then counts from captured_at.
const sessionSnapshotModel = "model session_snapshot\n" +
	"  id integer primary key\n" +
	"  name text not null\n" +
	"  position integer not null\n" +
	"  active_window_position integer not null\n" +
	"  last_accessed integer\n" +
	"  index session_snapshot_name unique name\n"

// Only the quickshell list poll writes session_agent (agent.go). settled_at is
// null until agent work stops, and seen_at is null until the session is
// attached.
const sessionAgentModel = "model session_agent\n" +
	"  session_id integer primary key references session_snapshot(id) on delete cascade\n" +
	"  was_working integer not null\n" +
	"  settled_at integer\n" +
	"  seen_at integer\n" +
	"  check (was_working in (0, 1))\n"

const windowSnapshotModel = "model window_snapshot\n" +
	"  id integer primary key\n" +
	"  session_id integer not null references session_snapshot(id) on delete cascade\n" +
	"  position integer not null\n" +
	"  name text not null\n" +
	"  layout text not null\n" +
	"  active_pane_position integer not null\n" +
	"  index window_snapshot_position unique session_id position\n"

// resume_id is the id the pane's agent resumes, and is null exactly for a
// shell pane. codex_profile is null unless codex ran with --profile. screen is
// null unless the pane is a c3po or r2d2 screen.
const paneSnapshotModel = "model pane_snapshot\n" +
	"  id integer primary key\n" +
	"  window_id integer not null references window_snapshot(id) on delete cascade\n" +
	"  position integer not null\n" +
	"  cwd text not null\n" +
	"  kind text not null\n" +
	"  resume_id text\n" +
	"  codex_profile text\n" +
	"  screen text\n" +
	"  check (kind in ('shell', 'codex', 'grok', 'c3po', 'r2d2'))\n" +
	"  check ((kind = 'shell') = (resume_id is null))\n" +
	"  check (codex_profile is null or kind = 'codex')\n" +
	"  check ((screen is not null) = (kind in ('c3po', 'r2d2')))\n" +
	"  index pane_snapshot_position unique window_id position\n"

func ormText() string {
	return savedSnapshotModel + "\n" + sessionSnapshotModel + "\n" + sessionAgentModel + "\n" +
		windowSnapshotModel + "\n" + paneSnapshotModel
}

// schemaVersion is the PRAGMA user_version of the schema above. Version 2
// replaced the flat tables, which linked windows and panes to a session by its
// name and kept the header as text in meta, with tables linked by id.
const schemaVersion = 2

// flatModels is the schema before version 2, as the upgrade reads it.
const flatModels = "model meta\n" +
	"  id integer primary key\n" +
	"  key text not null\n" +
	"  value text\n" +
	"\n" +
	"model session\n" +
	"  id integer primary key\n" +
	"  name text not null\n" +
	"  last_accessed integer\n" +
	"  agent_was_working integer\n" +
	"  agent_settled_at integer\n" +
	"  agent_seen_at integer\n" +
	"  active_window_position integer not null\n" +
	"  position integer not null\n" +
	"\n" +
	"model window\n" +
	"  id integer primary key\n" +
	"  session_name text not null\n" +
	"  position integer not null\n" +
	"  name text not null\n" +
	"  layout text not null\n" +
	"  active_pane_position integer not null\n" +
	"\n" +
	"model pane\n" +
	"  id integer primary key\n" +
	"  session_name text not null\n" +
	"  window_position integer not null\n" +
	"  position integer not null\n" +
	"  cwd text not null\n" +
	"  kind text not null\n" +
	"  codex_thread_id text\n" +
	"  codex_profile text\n" +
	"  screen text\n"

// copyFlatTables moves the flat rows into the linked tables. A repeated
// session name or position keeps its first row, rows whose session or window
// is gone are dropped, and a pane that restore would open as a plain shell
// becomes a shell pane.
const copyFlatTables = `INSERT INTO saved_snapshot(id, version, captured_at)
SELECT 1, CAST(value AS INTEGER),
  COALESCE((SELECT CAST(value AS INTEGER) FROM meta WHERE key = 'captured_at' AND value IS NOT NULL ORDER BY id LIMIT 1), 0)
FROM meta WHERE key = 'version' AND value IS NOT NULL ORDER BY id LIMIT 1;
INSERT INTO session_snapshot(id, name, position, active_window_position, last_accessed)
SELECT id, name, position, active_window_position, last_accessed FROM session
WHERE id IN (SELECT MIN(id) FROM session GROUP BY name);
INSERT INTO session_agent(session_id, was_working, settled_at, seen_at)
SELECT id, COALESCE(agent_was_working, 0) != 0, agent_settled_at, agent_seen_at FROM session
WHERE id IN (SELECT id FROM session_snapshot)
  AND (agent_was_working IS NOT NULL OR agent_settled_at IS NOT NULL OR agent_seen_at IS NOT NULL);
INSERT INTO window_snapshot(id, session_id, position, name, layout, active_pane_position)
SELECT w.id, s.id, w.position, w.name, w.layout, w.active_pane_position
FROM "window" AS w JOIN session_snapshot AS s ON s.name = w.session_name
WHERE w.id IN (SELECT MIN(id) FROM "window" GROUP BY session_name, position);
INSERT INTO pane_snapshot(window_id, position, cwd, kind, resume_id, codex_profile, screen)
SELECT window_id, position, cwd, kind,
  CASE WHEN kind != 'shell' THEN resume_id END,
  CASE WHEN kind = 'codex' THEN codex_profile END,
  CASE WHEN kind IN ('c3po', 'r2d2') THEN screen END
FROM (
  SELECT w.id AS window_id, p.position, p.cwd,
    CASE
      WHEN NULLIF(p.codex_thread_id, '') IS NULL THEN 'shell'
      WHEN p.kind IN ('codex', 'grok') THEN p.kind
      WHEN p.kind IN ('c3po', 'r2d2') AND NULLIF(p.screen, '') IS NOT NULL THEN p.kind
      ELSE 'shell'
    END AS kind,
    NULLIF(p.codex_thread_id, '') AS resume_id,
    NULLIF(p.codex_profile, '') AS codex_profile,
    NULLIF(p.screen, '') AS screen
  FROM pane AS p
  JOIN session_snapshot AS s ON s.name = p.session_name
  JOIN window_snapshot AS w ON w.session_id = s.id AND w.position = p.window_position
  WHERE p.id IN (SELECT MIN(id) FROM pane GROUP BY session_name, window_position, position)
);
DROP TABLE pane;
DROP TABLE "window";
DROP TABLE session;
DROP TABLE meta;
`

// linkTablesByID upgrades a flat database to version 2.
//
// Backfill: copyFlatTables carries every consistent flat row over, keeping
// session and window ids.
//
// Rollback: the copy, the drops, and the version bump are one transaction, so
// a failure leaves the flat tables as they were. A finished upgrade is not
// reversed: an older tesseract finds no flat rows and saves a new snapshot on
// its next run.
func linkTablesByID(dbPath string) error {
	version, err := userVersion(dbPath)
	if err != nil || version >= schemaVersion {
		return err
	}
	// An older flat file may lack columns the copy reads.
	if err := applyModels(dbPath, flatModels); err != nil {
		return err
	}
	var b strings.Builder
	b.WriteString("BEGIN IMMEDIATE;\n")
	// A concurrent upgrade may already have dropped the flat tables.
	for _, model := range parseModels(flatModels) {
		b.WriteString(createSQL(model) + ";\n")
	}
	b.WriteString(copyFlatTables)
	fmt.Fprintf(&b, "PRAGMA user_version = %d;\nCOMMIT;\n", schemaVersion)
	return sqliteExec(dbPath, b.String())
}
