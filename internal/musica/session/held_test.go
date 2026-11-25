package session

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

// startPausedFFplay starts a child of this process named ffplay, with track as
// its last argument, and stops it as musica stops a paused ffplay.
func startPausedFFplay(t *testing.T, track string) {
	t.Helper()
	script := filepath.Join(t.TempDir(), "ffplay")
	if err := os.WriteFile(script, []byte("#!/bin/sh\nread line\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(script, "-nodisp", "-autoexit", track)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = cmd.Process.Kill()
		_ = stdin.Close()
		_ = cmd.Wait()
	})
	if err := cmd.Process.Signal(syscall.SIGSTOP); err != nil {
		t.Fatal(err)
	}
}

// letGoWhenAsked closes server after delay once it is asked to end, as a
// session does once its ffplay and ffmpeg have stopped.
func letGoWhenAsked(server *Server, delay time.Duration) {
	go func() {
		<-server.EndRequests()
		time.Sleep(delay)
		server.Close()
	}()
}

func TestEndNotesTheTrackAPausedFFplayHasLoaded(t *testing.T) {
	t.Setenv("XDG_RUNTIME_DIR", t.TempDir())
	server, err := Listen()
	if err != nil {
		t.Fatalf("Listen() error = %v", err)
	}
	defer server.Close()
	track := "/music/focus/Flow.mp3"
	startPausedFFplay(t, track)
	letGoWhenAsked(server, 0)

	if err := End(); err != nil {
		t.Fatalf("End() error = %v", err)
	}
	next, err := ListenWatched()
	if err != nil {
		t.Fatalf("musica serve could not hold the session after End: %v", err)
	}
	defer next.Close()
	if got := next.TakeHeld(); got != track {
		t.Fatalf("TakeHeld() = %q, want %q", got, track)
	}
	if got := next.TakeHeld(); got != "" {
		t.Fatalf("a second TakeHeld() = %q, want nothing", got)
	}
}

func TestEndForgetsAnOlderNoteWhenNothingIsLoaded(t *testing.T) {
	runtimeDir := t.TempDir()
	t.Setenv("XDG_RUNTIME_DIR", runtimeDir)
	server, err := Listen()
	if err != nil {
		t.Fatalf("Listen() error = %v", err)
	}
	defer server.Close()
	note := filepath.Join(runtimeDir, "musica", heldName)
	if err := os.WriteFile(note, []byte("/music/old.mp3\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	letGoWhenAsked(server, 0)

	if err := End(); err != nil {
		t.Fatalf("End() error = %v", err)
	}
	if _, err := os.Stat(note); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("the older note stayed: %v", err)
	}
}

func TestEndReturnsOnlyOnceTheSessionLetsGo(t *testing.T) {
	t.Setenv("XDG_RUNTIME_DIR", t.TempDir())
	server, err := Listen()
	if err != nil {
		t.Fatalf("Listen() error = %v", err)
	}
	defer server.Close()
	letGoWhenAsked(server, 300*time.Millisecond)

	started := time.Now()
	if err := End(); err != nil {
		t.Fatalf("End() error = %v", err)
	}
	if waited := time.Since(started); waited < 300*time.Millisecond {
		t.Fatalf("End() returned after %v, before the session let go", waited)
	}
	next, err := ListenWatched()
	if err != nil {
		t.Fatalf("musica serve could not hold the session after End: %v", err)
	}
	next.Close()
}

func TestEndReportsASessionThatDoesNotLetGo(t *testing.T) {
	t.Setenv("XDG_RUNTIME_DIR", t.TempDir())
	previous := endWait
	endWait = 200 * time.Millisecond
	defer func() { endWait = previous }()
	server, err := Listen()
	if err != nil {
		t.Fatalf("Listen() error = %v", err)
	}
	defer server.Close()

	if err := End(); err == nil || err.Error() != "musica did not end" {
		t.Fatalf("End() error = %v, want musica did not end", err)
	}
}
