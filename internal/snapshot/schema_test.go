package snapshot

import (
	"reflect"
	"testing"
	"time"
)

func TestSnapshotSchemaRejectsInconsistentRows(t *testing.T) {
	home := t.TempDir()
	isolateSQLX(t, home)
	saved := &SavedSnapshot{Version: Version, CapturedAt: 1, Sessions: []SessionSnapshot{{
		Name:    "work",
		Windows: []WindowSnapshot{{Name: "main", Panes: []PaneSnapshot{{CWD: "/", Kind: "shell"}}}},
	}}}
	if err := writeSavedSnapshot(home, saved); err != nil {
		t.Fatal(err)
	}
	db, err := openDB(home)
	if err != nil {
		t.Fatal(err)
	}
	pane := "INSERT INTO pane_snapshot(window_id, position, cwd, kind, resume_id, screen) SELECT id, 1, '/', "
	rejected := map[string]string{
		"a second header":               "INSERT INTO saved_snapshot(id, version, captured_at) VALUES (2, 1, 1)",
		"a repeated session name":       "INSERT INTO session_snapshot(name, position, active_window_position) VALUES ('work', 1, 0)",
		"a window of no session":        "INSERT INTO window_snapshot(session_id, position, name, layout, active_pane_position) VALUES (999, 0, 'w', '', 0)",
		"a repeated window position":    "INSERT INTO window_snapshot(session_id, position, name, layout, active_pane_position) SELECT id, 0, 'w', '', 0 FROM session_snapshot",
		"a shell pane with a resume id": pane + "'shell', 'x', NULL FROM window_snapshot",
		"a grok pane with no resume id": pane + "'grok', NULL, NULL FROM window_snapshot",
		"a screen pane with no screen":  pane + "'r2d2', 'x', NULL FROM window_snapshot",
		"a grok pane with a screen":     pane + "'grok', 'x', 'o' FROM window_snapshot",
		"an unknown kind":               pane + "'vim', 'x', NULL FROM window_snapshot",
		"a codex profile on grok":       "INSERT INTO pane_snapshot(window_id, position, cwd, kind, resume_id, codex_profile) SELECT id, 1, '/', 'grok', 'x', 'zai' FROM window_snapshot",
	}
	for name, sql := range rejected {
		if err := sqliteExec(db, sql); err == nil {
			t.Errorf("the schema accepted %s", name)
		}
	}

	if _, err := RecordAgentActivity(home, []AgentActivity{{Name: "work", Total: 1, Working: 1}}, time.Unix(10, 0)); err != nil {
		t.Fatal(err)
	}
	if err := sqliteExec(db, "DELETE FROM session_snapshot WHERE name = 'work'"); err != nil {
		t.Fatal(err)
	}
	for _, table := range []string{"window_snapshot", "pane_snapshot", "session_agent"} {
		var rows []struct {
			N int `json:"n"`
		}
		if err := sqliteQueryJSON(db, "SELECT COUNT(*) AS n FROM "+table, &rows); err != nil {
			t.Fatal(err)
		}
		if len(rows) != 1 || rows[0].N != 0 {
			t.Errorf("%s kept rows of a deleted session: %+v", table, rows)
		}
	}
}

// A snapshot that cannot be saved leaves the previous one whole.
func TestFailedSnapshotWriteKeepsPreviousSnapshot(t *testing.T) {
	home := t.TempDir()
	isolateSQLX(t, home)
	previous := &SavedSnapshot{Version: Version, CapturedAt: 100, Sessions: []SessionSnapshot{{
		Name:         "keep",
		LastAccessed: 100,
		Windows:      []WindowSnapshot{{Name: "main", Panes: []PaneSnapshot{{CWD: "/", Kind: "shell"}}}},
	}}}
	if err := writeSavedSnapshot(home, previous); err != nil {
		t.Fatal(err)
	}
	broken := &SavedSnapshot{Version: Version, CapturedAt: 200, Sessions: []SessionSnapshot{{
		Name:    "new",
		Windows: []WindowSnapshot{{Name: "main", Panes: []PaneSnapshot{{CWD: "/", Kind: "shell", ResumeID: "x"}}}},
	}}}
	if err := writeSavedSnapshot(home, broken); err == nil {
		t.Fatal("an inconsistent snapshot was saved")
	}
	saved, err := LoadSavedSnapshot(home)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(saved, previous) {
		t.Fatalf("saved = %+v, want the previous snapshot %+v", saved, previous)
	}
}
