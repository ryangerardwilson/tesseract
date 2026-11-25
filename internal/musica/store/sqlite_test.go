package store

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestIsBusy(t *testing.T) {
	locked := fmt.Errorf("sqlite3: Parse error near line 2: database is locked (5)")
	if !IsBusy(locked) {
		t.Fatal("database is locked was not busy")
	}
	if IsBusy(fmt.Errorf("sqlite3: no such table: downloads")) {
		t.Fatal("unrelated error was busy")
	}
	if IsBusy(nil) {
		t.Fatal("nil was busy")
	}
}

func TestSQLiteWaitsWhileDatabaseIsLocked(t *testing.T) {
	if _, err := exec.LookPath("python3"); err != nil {
		t.Skip("python3 is required to hold a sqlite lock")
	}
	path := filepath.Join(t.TempDir(), "musica.sqlite")
	if err := sqliteExec(path, `CREATE TABLE t(id INTEGER);`); err != nil {
		t.Fatal(err)
	}
	script := filepath.Join(t.TempDir(), "hold.py")
	body := "import sqlite3, sys\n" +
		"c = sqlite3.connect(sys.argv[1])\n" +
		"c.execute('BEGIN EXCLUSIVE')\n" +
		"print('locked', flush=True)\n" +
		"sys.stdin.readline()\n" +
		"c.commit()\n"
	if err := os.WriteFile(script, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("python3", script, path)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() {
		_ = stdin.Close()
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
		_ = cmd.Wait()
	}()
	line, err := bufio.NewReader(stdout).ReadString('\n')
	if err != nil || !strings.Contains(line, "locked") {
		t.Fatalf("lock holder = %q %v", line, err)
	}

	done := make(chan error, 1)
	go func() {
		done <- sqliteExec(path, `INSERT INTO t(id) VALUES (1);`)
	}()
	select {
	case err := <-done:
		t.Fatalf("write finished while the database was locked: %v", err)
	case <-time.After(300 * time.Millisecond):
	}
	if _, err := stdin.Write([]byte("\n")); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("write did not finish after the lock was released")
	}
}
