package store

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// sqliteWait bounds one sqlite3 run. Its busy timeout is 5 s.
const sqliteWait = 15 * time.Second

func sqliteBin() (string, error) {
	if path, err := exec.LookPath("sqlite3"); err == nil {
		return path, nil
	}
	return "", fmt.Errorf("sqlite3 is not installed")
}

func sqliteExec(path, sql string) error {
	_, err := sqliteRun(path, sql, false)
	return err
}

func sqliteRun(path, sql string, asJSON bool) (string, error) {
	bin, err := sqliteBin()
	if err != nil {
		return "", err
	}
	args := []string{"-batch", "-noinit", path}
	if asJSON {
		args = append(args, "-json")
	}
	ctx, cancel := context.WithTimeout(context.Background(), sqliteWait)
	defer cancel()
	cmd := exec.CommandContext(ctx, bin, args...)
	// A SQL argument runs before -cmd, so the busy timeout has to be stdin.
	// The CLI default is zero, and the TUI lists the same file twice a second
	// while the worker writes progress.
	cmd.Stdin = strings.NewReader(".timeout 5000\n" + sql + "\n")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		detail := strings.TrimSpace(stderr.String())
		if detail == "" {
			detail = err.Error()
		}
		return "", fmt.Errorf("sqlite3: %s", detail)
	}
	return stdout.String(), nil
}

// IsBusy reports that sqlite gave up while another process held the database.
func IsBusy(err error) bool {
	return err != nil && strings.Contains(err.Error(), "database is locked")
}

func queryJobs(path, sql string) ([]Job, error) {
	raw, err := sqliteRun(path, sql, true)
	if err != nil {
		return nil, err
	}
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	var jobs []Job
	if err := json.Unmarshal([]byte(raw), &jobs); err != nil {
		return nil, fmt.Errorf("sqlite json: %w", err)
	}
	return jobs, nil
}

func quote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "''") + "'"
}
