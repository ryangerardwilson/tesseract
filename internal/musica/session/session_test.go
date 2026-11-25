package session

import (
	"errors"
	"path/filepath"
	"testing"
	"time"
)

func TestEndRequestsRunningSessionToExit(t *testing.T) {
	t.Setenv("XDG_RUNTIME_DIR", t.TempDir())
	server, err := Listen()
	if err != nil {
		t.Fatalf("Listen() error = %v", err)
	}
	defer server.Close()

	result := make(chan error, 1)
	go func() { result <- End() }()

	select {
	case <-server.EndRequests():
		server.Close()
	case <-time.After(time.Second):
		t.Fatal("server did not receive end request")
	}
	if err := <-result; err != nil {
		t.Fatalf("End() error = %v", err)
	}
}

func TestListenRejectsSecondRunningSession(t *testing.T) {
	t.Setenv("XDG_RUNTIME_DIR", t.TempDir())
	server, err := Listen()
	if err != nil {
		t.Fatalf("first Listen() error = %v", err)
	}
	defer server.Close()

	second, err := Listen()
	if second != nil {
		second.Close()
		t.Fatal("second Listen() returned a server")
	}
	if !errors.Is(err, ErrAlreadyRunning) {
		t.Fatalf("second Listen() error = %v, want ErrAlreadyRunning", err)
	}
}

func TestCloseRemovesControlSocket(t *testing.T) {
	runtimeDir := t.TempDir()
	t.Setenv("XDG_RUNTIME_DIR", runtimeDir)
	server, err := Listen()
	if err != nil {
		t.Fatalf("Listen() error = %v", err)
	}
	server.Close()

	if err := End(); err == nil {
		t.Fatal("End() succeeded after server.Close()")
	}
	if _, err := filepath.Abs(runtimeDir); err != nil {
		t.Fatalf("runtime directory was invalid: %v", err)
	}
}
