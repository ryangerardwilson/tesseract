package snapshot

import (
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"testing"
)

// A flat tm.sqlite from before schema version 2 becomes tesseract.sqlite with
// its rows linked by id.
func TestLegacyTMDatabaseBecomesTesseract(t *testing.T) {
	root := t.TempDir()
	t.Setenv("SQLX_ROOT", root)
	data := filepath.Join(root, "data")
	if err := os.MkdirAll(data, 0o700); err != nil {
		t.Fatal(err)
	}
	legacy := filepath.Join(data, "tm.sqlite")
	// The oldest flat tables had no agent columns and no pane screen.
	sql := `CREATE TABLE meta (id INTEGER PRIMARY KEY, key TEXT NOT NULL, value TEXT);
CREATE TABLE session (id INTEGER PRIMARY KEY, name TEXT NOT NULL, last_accessed INTEGER,
  active_window_position INTEGER NOT NULL, position INTEGER NOT NULL);
CREATE TABLE "window" (id INTEGER PRIMARY KEY, session_name TEXT NOT NULL, position INTEGER NOT NULL,
  name TEXT NOT NULL, layout TEXT NOT NULL, active_pane_position INTEGER NOT NULL);
CREATE TABLE pane (id INTEGER PRIMARY KEY, session_name TEXT NOT NULL, window_position INTEGER NOT NULL,
  position INTEGER NOT NULL, cwd TEXT NOT NULL, kind TEXT NOT NULL, codex_thread_id TEXT, codex_profile TEXT);
INSERT INTO meta(key, value) VALUES ('version', '1'), ('captured_at', '1700000000'), ('imported_snapshot', '1');
INSERT INTO session(name, last_accessed, active_window_position, position) VALUES ('tm', 1700000000, 0, 0);
INSERT INTO "window"(session_name, position, name, layout, active_pane_position) VALUES ('tm', 0, 'work', 'tiled', 1);
INSERT INTO pane(session_name, window_position, position, cwd, kind, codex_thread_id, codex_profile) VALUES
  ('tm', 0, 0, '/home/ryan', 'grok', '01a17622-5fd9-7bc2-81ff-3e200a8c16b7', NULL),
  ('tm', 0, 1, '/home/ryan', 'codex', '', 'zai'),
  ('gone', 0, 0, '/home/ryan', 'shell', NULL, NULL);
`
	cmd := exec.Command("sqlite3", legacy, sql)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("seed: %v %s", err, out)
	}
	db, err := openDB("")
	if err != nil {
		t.Fatal(err)
	}
	if db != filepath.Join(data, "tesseract.sqlite") {
		t.Fatalf("database = %s", db)
	}
	if _, err := os.Stat(legacy); !os.IsNotExist(err) {
		t.Fatalf("legacy still present: %v", err)
	}
	saved, err := readSavedSnapshot(db)
	if err != nil {
		t.Fatal(err)
	}
	want := &SavedSnapshot{
		Version:    1,
		CapturedAt: 1_700_000_000,
		Sessions: []SessionSnapshot{{
			Name:         "tm",
			LastAccessed: 1_700_000_000,
			Windows: []WindowSnapshot{{
				Name:               "work",
				Layout:             "tiled",
				ActivePanePosition: 1,
				Panes: []PaneSnapshot{
					{CWD: "/home/ryan", Kind: "grok", ResumeID: "01a17622-5fd9-7bc2-81ff-3e200a8c16b7"},
					// A codex pane with no thread restored as a shell, so it is one.
					{CWD: "/home/ryan", Kind: "shell"},
				},
			}},
		}},
	}
	if !reflect.DeepEqual(saved, want) {
		t.Fatalf("saved = %+v, want %+v", saved, want)
	}
	var flat []struct {
		Name string `json:"name"`
	}
	if err := sqliteQueryJSON(db, "SELECT name FROM sqlite_master WHERE type = 'table' AND name IN ('meta', 'session', 'window', 'pane')", &flat); err != nil {
		t.Fatal(err)
	}
	if len(flat) != 0 {
		t.Fatalf("flat tables left: %+v", flat)
	}
	if version, err := userVersion(db); err != nil || version != schemaVersion {
		t.Fatalf("user_version = %d, %v; want %d", version, err, schemaVersion)
	}
}
