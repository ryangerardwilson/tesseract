package session

import (
	"bufio"
	"fmt"
	"net"
	"path/filepath"
	"testing"
	"time"
)

func TestResumeRoundTrip(t *testing.T) {
	runtimeDir := t.TempDir()
	t.Setenv("XDG_RUNTIME_DIR", runtimeDir)
	note := Resume{Path: "/music/flow.mp3", Offset: 90 * time.Second, Paused: false, Menu: false}
	if err := WriteResume("", note); err != nil {
		t.Fatalf("WriteResume: %v", err)
	}
	server, err := ListenWatched()
	if err != nil {
		t.Fatalf("ListenWatched: %v", err)
	}
	defer server.Close()
	got, ok := server.TakeResume()
	if !ok || got != note {
		t.Fatalf("TakeResume = %+v, %v", got, ok)
	}
	if _, again := server.TakeResume(); again {
		t.Fatal("resume was left in place")
	}
}

func TestReloadCommandSignalsServe(t *testing.T) {
	runtimeDir := t.TempDir()
	t.Setenv("XDG_RUNTIME_DIR", runtimeDir)
	server, err := ListenWatched()
	if err != nil {
		t.Fatalf("ListenWatched: %v", err)
	}
	defer server.Close()
	connection, err := net.DialTimeout("unix", filepath.Join(runtimeDir, "musica", controlSocketName), time.Second)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer connection.Close()
	if _, err := fmt.Fprintln(connection, reloadCommand); err != nil {
		t.Fatalf("write reload: %v", err)
	}
	_ = connection.SetReadDeadline(time.Now().Add(2 * time.Second))
	line, err := bufio.NewReader(connection).ReadString('\n')
	if err != nil || line != "ok\n" {
		t.Fatalf("reload response %q, %v", line, err)
	}
	select {
	case <-server.ReloadRequests():
	case <-time.After(2 * time.Second):
		t.Fatal("reload was not signaled")
	}
}

func TestTUIRefusesReload(t *testing.T) {
	runtimeDir := t.TempDir()
	t.Setenv("XDG_RUNTIME_DIR", runtimeDir)
	server, err := Listen()
	if err != nil {
		t.Fatalf("Listen: %v", err)
	}
	defer server.Close()
	connection, err := net.DialTimeout("unix", filepath.Join(runtimeDir, "musica", controlSocketName), time.Second)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer connection.Close()
	if _, err := fmt.Fprintln(connection, reloadCommand); err != nil {
		t.Fatalf("write reload: %v", err)
	}
	line, err := bufio.NewReader(connection).ReadString('\n')
	if err != nil || line != "unknown command\n" {
		t.Fatalf("reload response %q, %v", line, err)
	}
}
