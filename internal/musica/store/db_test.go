package store

import (
	"path/filepath"
	"testing"
)

func TestDownloadRoundTrip(t *testing.T) {
	t.Setenv("MUSICA_DB", filepath.Join(t.TempDir(), "musica.sqlite"))

	db, err := Open()
	if err != nil {
		t.Fatal(err)
	}
	job, err := db.InsertJob("https://youtu.be/example", "Blue Monday", "/music/cassette", "Blue Monday.mp3")
	if err != nil {
		t.Fatal(err)
	}
	if job.Status != "queued" {
		t.Fatalf("status = %q", job.Status)
	}
	if err := db.SetPID(job.ID, 4242); err != nil {
		t.Fatal(err)
	}
	if err := db.MarkProgress(job.ID, 40, "1MiB/s", "00:02"); err != nil {
		t.Fatal(err)
	}
	got, ok, err := db.Get(job.ID)
	if err != nil || !ok {
		t.Fatalf("get = %v %v", err, ok)
	}
	if got.Percent != 40 || got.PID != 4242 || got.Speed != "1MiB/s" {
		t.Fatalf("progress job = %+v", got)
	}
	if err := db.PutSetting("music_dir", "/music"); err != nil {
		t.Fatal(err)
	}
	dir, ok, err := db.Setting("music_dir")
	if err != nil || !ok || dir != "/music" {
		t.Fatalf("setting = %q %v %v", dir, ok, err)
	}
}

func TestClaimNextSkipsRunningJobs(t *testing.T) {
	t.Setenv("MUSICA_DB", filepath.Join(t.TempDir(), "musica.sqlite"))
	db, err := Open()
	if err != nil {
		t.Fatal(err)
	}
	first, err := db.InsertJob("https://youtu.be/a", "A", "/tmp", "A.mp3")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.InsertJob("https://youtu.be/b", "B", "/tmp", "B.mp3"); err != nil {
		t.Fatal(err)
	}
	claimed, ok, err := db.ClaimNext()
	if err != nil || !ok || claimed.ID != first.ID || claimed.Status != "running" {
		t.Fatalf("claimed = %+v ok=%v err=%v", claimed, ok, err)
	}
	if err := db.SetPID(claimed.ID, 7); err != nil {
		t.Fatal(err)
	}
	next, ok, err := db.ClaimNext()
	if err != nil || !ok || next.Title != "B" {
		t.Fatalf("second claim = %+v ok=%v err=%v", next, ok, err)
	}
}

func TestIsolatedDBDoesNotUseDefaultPath(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nested", "musica.sqlite")
	t.Setenv("MUSICA_DB", path)
	if got := FilePath(); got != path {
		t.Fatalf("FilePath = %q", got)
	}
	if _, err := Open(); err != nil {
		t.Fatal(err)
	}
}
