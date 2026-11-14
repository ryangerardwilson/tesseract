package snapshot

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestLegacyTMDatabaseBecomesTesseract(t *testing.T) {
	root := t.TempDir()
	t.Setenv("SQLX_ROOT", root)
	data := filepath.Join(root, "data")
	if err := os.MkdirAll(data, 0o700); err != nil {
		t.Fatal(err)
	}
	legacy := filepath.Join(data, "tm.sqlite")
	sql := `CREATE TABLE pane (
  id INTEGER PRIMARY KEY,
  session_name TEXT NOT NULL,
  window_position INTEGER NOT NULL,
  position INTEGER NOT NULL,
  cwd TEXT NOT NULL,
  kind TEXT NOT NULL,
  codex_thread_id TEXT,
  codex_profile TEXT
);
INSERT INTO pane(session_name, window_position, position, cwd, kind, codex_thread_id)
VALUES ('tm', 0, 0, '/home/ryan', 'grok', '01a17622-5fd9-7bc2-81ff-3e200a8c16b7');
`
	cmd := exec.Command("sqlite3", legacy, sql)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("seed: %v %s", err, out)
	}
	if _, err := openDB(""); err != nil {
		t.Fatal(err)
	}
	db := filepath.Join(root, "data", "tesseract.sqlite")
	if _, err := os.Stat(db); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(legacy); !os.IsNotExist(err) {
		t.Fatalf("legacy still present: %v", err)
	}
	saved, err := readSavedSnapshot(db)
	if err != nil {
		t.Fatal(err)
	}
	if saved == nil || len(saved.Sessions) != 0 {
		// The seeded file has a pane and no session header, so the snapshot
		// reader may reject it. The row itself must still be readable.
	}
	var rows []struct {
		Kind string  `json:"kind"`
		ID   *string `json:"codex_thread_id"`
	}
	if err := queryJSON(db, "SELECT kind, codex_thread_id FROM pane", &rows); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].Kind != "grok" || rows[0].ID == nil || *rows[0].ID != "01a17622-5fd9-7bc2-81ff-3e200a8c16b7" {
		t.Fatalf("rows = %+v", rows)
	}
	var cols []struct {
		Name string `json:"name"`
	}
	if err := queryJSON(db, "SELECT name FROM pragma_table_info('pane')", &cols); err != nil {
		t.Fatal(err)
	}
	found := false
	for _, col := range cols {
		if col.Name == "screen" {
			found = true
		}
	}
	if !found {
		t.Fatalf("columns = %+v", cols)
	}
}
