package download

import (
	"errors"
	"testing"
)

func TestWorkerPathIsMusicaOnPath(t *testing.T) {
	old := lookPath
	defer func() { lookPath = old }()
	lookPath = func(name string) (string, error) {
		if name != "musica" {
			t.Fatalf("looked up %q", name)
		}
		return "/opt/bin/musica", nil
	}
	path, err := workerPath()
	if err != nil || path != "/opt/bin/musica" {
		t.Fatalf("%q %v", path, err)
	}
	cmd := workerCommand(path)
	if cmd.Path != "/opt/bin/musica" || len(cmd.Args) != 2 || cmd.Args[1] != "download-worker" || !cmd.SysProcAttr.Setsid {
		t.Fatalf("%q %#v", cmd.Path, cmd.Args)
	}
}

func TestSpawnFailsWithoutMusicaOnPath(t *testing.T) {
	old := lookPath
	defer func() { lookPath = old }()
	missing := errors.New(`exec: "musica": executable file not found in $PATH`)
	lookPath = func(string) (string, error) { return "", missing }
	if err := spawnDetached(); !errors.Is(err, missing) {
		t.Fatalf("spawn %v", err)
	}
}
