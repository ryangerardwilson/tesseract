package store

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

// nebula keep owns the durable copy of this file at s3://backups/musica.sqlite.
// musica says "the file changed" after each commit and asks for a missing file
// at startup; the upload, the snapshot, and the rule that an empty file never
// replaces the backup all live in nebula. Nebula sql is the Postgres interface
// and is not called.

const keepName = "musica"

const (
	markWait    = 5 * time.Second  // local only: a record and a wake-up
	restoreWait = 90 * time.Second // one download
)

// keepMark records that the file changed. It returns once nebula has noted
// the change; nebula's detached worker uploads it later.
func keepMark(path string) error {
	return runKeep(markWait, "mark", path)
}

// keepRestore brings a missing file back. nebula prints "restored <path>",
// or makes an empty file and prints "fresh <path>" when there is no backup.
// It fails and makes no file when the backup is unreachable.
func keepRestore(path string) error {
	return runKeep(restoreWait, "restore", path)
}

// keepSkipped keeps a test binary off the operator's nebula: under test only
// the nebula that NEBULA_BIN names is run.
func keepSkipped() bool {
	return testing.Testing() && os.Getenv("NEBULA_BIN") == ""
}

func runKeep(wait time.Duration, verb, path string) error {
	if keepSkipped() {
		return nil
	}
	bin, err := nebulaBin()
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), wait)
	defer cancel()
	out, err := exec.CommandContext(ctx, bin, "keep", verb, keepName, path).CombinedOutput()
	if err == nil {
		return nil
	}
	text := strings.TrimSpace(string(out))
	if text == "" {
		text = err.Error()
	}
	return fmt.Errorf("nebula keep %s %s: %s", verb, keepName, text)
}

func nebulaBin() (string, error) {
	if bin := os.Getenv("NEBULA_BIN"); bin != "" {
		return bin, nil
	}
	bin, err := exec.LookPath("nebula")
	if err != nil {
		return "", errors.New("nebula is not on PATH")
	}
	return bin, nil
}
