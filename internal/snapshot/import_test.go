package snapshot

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestLegacySnapshotImportsOnce(t *testing.T) {
	home := t.TempDir()
	root := t.TempDir()
	t.Setenv("SQLX_ROOT", root)
	saved := &SavedSnapshot{
		Version:    Version,
		CapturedAt: 42,
		Sessions: []SessionSnapshot{
			{
				Name:                 "kept",
				LastAccessed:         99,
				ActiveWindowPosition: 1,
				Windows: []WindowSnapshot{
					{
						Name:               "main",
						Layout:             "even-horizontal, 'x'",
						ActivePanePosition: 1,
						Panes: []PaneSnapshot{
							{CWD: "/tmp", Kind: "codex", ResumeID: "thread", CodexProfile: "zai"},
							{CWD: "/work", Kind: "shell"},
						},
					},
				},
			},
			{Name: "other", LastAccessed: 10},
		},
	}
	path := Path(home)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	data, err := json.MarshalIndent(saved, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, append(data, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := LoadSavedSnapshot(home)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, saved) {
		t.Fatalf("imported snapshot = %#v, want %#v", got, saved)
	}
	dbPath := filepath.Join(root, "data", "tesseract.sqlite")
	info, err := os.Stat(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("db mode = %o", info.Mode().Perm())
	}
	for _, dir := range []string{root, filepath.Join(root, "data")} {
		dirInfo, err := os.Stat(dir)
		if err != nil {
			t.Fatal(err)
		}
		if dirInfo.Mode().Perm() != 0o700 {
			t.Fatalf("%s mode = %o", dir, dirInfo.Mode().Perm())
		}
	}
	if _, err := os.Stat(filepath.Join(home, ".sqlx")); !os.IsNotExist(err) {
		t.Fatal("SQLX_ROOT was ignored")
	}

	if err := os.WriteFile(path, []byte("{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	again, err := LoadSavedSnapshot(home)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(again, saved) {
		t.Fatal("legacy snapshot was imported again")
	}
	if err := writeSavedSnapshot(home, &SavedSnapshot{Version: Version, CapturedAt: 7}); err != nil {
		t.Fatal(err)
	}
	empty, err := LoadSavedSnapshot(home)
	if err != nil {
		t.Fatal(err)
	}
	if empty == nil || len(empty.Sessions) != 0 || empty.CapturedAt != 7 {
		t.Fatalf("empty save = %#v", empty)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatal(err)
	}
}
