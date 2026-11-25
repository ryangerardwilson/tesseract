package store

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// fakeNebula is a NEBULA_BIN that logs its arguments. FAKE_KEEP picks what
// keep restore does: fresh makes an empty file, restored copies
// FAKE_KEEP_SRC, fail exits 1 and makes nothing. markfail fails keep mark.
const fakeNebula = `#!/bin/sh
printf '%s\n' "$*" >> "$FAKE_KEEP_LOG"
if [ "$1 $2" = "keep restore" ]; then
  case "$FAKE_KEEP" in
  fail) echo "restore s3://backups/musica.sqlite: connection timed out" >&2; exit 1 ;;
  restored) cp "$FAKE_KEEP_SRC" "$4"; echo "restored $4" ;;
  *) : > "$4"; echo "fresh $4" ;;
  esac
fi
if [ "$1 $2" = "keep mark" ] && [ "$FAKE_KEEP" = markfail ]; then
  echo "keep is locked" >&2
  exit 1
fi
`

// runtimeKeep points the store at a runtime file under a temporary
// XDG_RUNTIME_DIR and at a fake nebula. It returns the file and the log.
func runtimeKeep(t *testing.T, mode string) (path, log string) {
	t.Helper()
	dir := t.TempDir()
	bin := filepath.Join(dir, "nebula")
	if err := os.WriteFile(bin, []byte(fakeNebula), 0o755); err != nil {
		t.Fatal(err)
	}
	log = filepath.Join(dir, "keep.log")
	t.Setenv("MUSICA_DB", "")
	t.Setenv("SQLX_ROOT", "")
	t.Setenv("XDG_RUNTIME_DIR", filepath.Join(dir, "run"))
	t.Setenv("NEBULA_BIN", bin)
	t.Setenv("FAKE_KEEP", mode)
	t.Setenv("FAKE_KEEP_LOG", log)
	return filepath.Join(dir, "run", "musica", "musica.sqlite"), log
}

func keepCalls(t *testing.T, log string) []string {
	t.Helper()
	body, err := os.ReadFile(log)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		t.Fatal(err)
	}
	return strings.Split(strings.TrimSpace(string(body)), "\n")
}

func TestOpenRestoresAMissingFileThroughKeep(t *testing.T) {
	backup := filepath.Join(t.TempDir(), "backup.sqlite")
	if err := sqliteExec(backup, createSQL+`INSERT INTO settings (key, value) VALUES ('music_dir', '/music/kept');`); err != nil {
		t.Fatal(err)
	}
	path, log := runtimeKeep(t, "restored")
	t.Setenv("FAKE_KEEP_SRC", backup)
	if FilePath() != path {
		t.Fatalf("FilePath = %q, want %q", FilePath(), path)
	}
	db, err := Open()
	if err != nil {
		t.Fatal(err)
	}
	dir, ok, err := db.Setting("music_dir")
	if err != nil || !ok || dir != "/music/kept" {
		t.Fatalf("restored setting = %q %v %v", dir, ok, err)
	}
	calls := keepCalls(t, log)
	if len(calls) != 1 || calls[0] != "keep restore musica "+path {
		t.Fatalf("calls %q", calls)
	}
	if _, err := Open(); err != nil {
		t.Fatal(err)
	}
	if calls := keepCalls(t, log); len(calls) != 1 {
		t.Fatalf("an existing file was restored again: %q", calls)
	}
}

func TestAFreshFileFromKeepGetsTheSchema(t *testing.T) {
	path, _ := runtimeKeep(t, "fresh")
	db, err := Open()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.InsertJob("https://youtu.be/a", "A", "/tmp", "A.mp3"); err != nil {
		t.Fatal(err)
	}
	if info, err := os.Stat(path); err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("file %v %v", info, err)
	}
}

func TestAnUnreachableBackupLeavesNoFile(t *testing.T) {
	path, _ := runtimeKeep(t, "fail")
	_, err := Open()
	if err == nil || !strings.Contains(err.Error(), "nebula keep restore musica") || !strings.Contains(err.Error(), "timed out") {
		t.Fatalf("err %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("musica made the file itself: %v", err)
	}
}

func TestCommitsMarkTheFileAndProgressDoesNot(t *testing.T) {
	path, log := runtimeKeep(t, "fresh")
	db, err := Open()
	if err != nil {
		t.Fatal(err)
	}
	job, err := db.InsertJob("https://youtu.be/a", "A", "/tmp", "A.mp3")
	if err != nil {
		t.Fatal(err)
	}
	if err := db.SetPID(job.ID, 7); err != nil {
		t.Fatal(err)
	}
	if err := db.MarkProgress(job.ID, 50, "1MiB/s", "00:01"); err != nil {
		t.Fatal(err)
	}
	if err := db.MarkStatus(job.ID, "done", ""); err != nil {
		t.Fatal(err)
	}
	if err := db.PutSetting("music_dir", "/music"); err != nil {
		t.Fatal(err)
	}
	mark := "keep mark musica " + path
	want := []string{"keep restore musica " + path, mark, mark, mark}
	if got := keepCalls(t, log); strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("calls %q, want %q", got, want)
	}
}

func TestAFailedMarkKeepsTheCommit(t *testing.T) {
	runtimeKeep(t, "fresh")
	db, err := Open()
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("FAKE_KEEP", "markfail")
	job, err := db.InsertJob("https://youtu.be/a", "A", "/tmp", "A.mp3")
	if err != nil {
		t.Fatalf("the write failed after it committed: %v", err)
	}
	if _, ok, err := db.Get(job.ID); err != nil || !ok {
		t.Fatalf("job %v %v", ok, err)
	}
}

func TestAnIsolatedFileNeverReachesKeep(t *testing.T) {
	_, log := runtimeKeep(t, "fail")
	t.Setenv("MUSICA_DB", filepath.Join(t.TempDir(), "musica.sqlite"))
	db, err := Open()
	if err != nil {
		t.Fatal(err)
	}
	if err := db.PutSetting("music_dir", "/music"); err != nil {
		t.Fatal(err)
	}
	if calls := keepCalls(t, log); len(calls) != 0 {
		t.Fatalf("calls %q", calls)
	}
}

func TestATestBinaryWithoutNebulaBinSkipsKeep(t *testing.T) {
	path, log := runtimeKeep(t, "fail")
	t.Setenv("NEBULA_BIN", "")
	t.Setenv("PATH", filepath.Dir(log)+string(os.PathListSeparator)+os.Getenv("PATH"))
	db, err := Open()
	if err != nil {
		t.Fatal(err)
	}
	if err := db.PutSetting("music_dir", "/music"); err != nil {
		t.Fatal(err)
	}
	if calls := keepCalls(t, log); len(calls) != 0 {
		t.Fatalf("the nebula on PATH ran: %q", calls)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatal(err)
	}
}
