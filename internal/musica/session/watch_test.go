package session

import (
	"bufio"
	"fmt"
	"net"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func dialWatch(t *testing.T, runtimeDir string) (net.Conn, *bufio.Reader) {
	t.Helper()
	connection, err := net.DialTimeout("unix", filepath.Join(runtimeDir, "musica", controlSocketName), time.Second)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	if _, err := fmt.Fprintln(connection, watchCommand); err != nil {
		t.Fatalf("write watch: %v", err)
	}
	return connection, bufio.NewReader(connection)
}

func waitWatching(t *testing.T, server *Server, want bool) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for server.Watching() != want {
		if time.Now().After(deadline) {
			t.Fatalf("Watching() never became %v", want)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func TestWatchStreamsPublishedLinesAndPassesKeys(t *testing.T) {
	runtimeDir := t.TempDir()
	t.Setenv("XDG_RUNTIME_DIR", runtimeDir)
	server, err := ListenWatched()
	if err != nil {
		t.Fatalf("ListenWatched() error = %v", err)
	}
	defer server.Close()
	if server.Watching() {
		t.Fatal("a fresh session reports a watcher")
	}

	connection, reader := dialWatch(t, runtimeDir)
	defer connection.Close()
	waitWatching(t, server, true)

	server.Publish([]byte(`{"type":"frame"}`))
	_ = connection.SetReadDeadline(time.Now().Add(2 * time.Second))
	line, err := reader.ReadString('\n')
	if err != nil || line != "{\"type\":\"frame\"}\n" {
		t.Fatalf("read %q, %v", line, err)
	}

	if _, err := fmt.Fprintln(connection, `{"key":"j"}`); err != nil {
		t.Fatalf("write key: %v", err)
	}
	select {
	case key := <-server.Keys():
		if key != `{"key":"j"}` {
			t.Fatalf("key = %q", key)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("the key line never arrived")
	}
}

func TestTUISessionRefusesWatch(t *testing.T) {
	runtimeDir := t.TempDir()
	t.Setenv("XDG_RUNTIME_DIR", runtimeDir)
	server, err := Listen()
	if err != nil {
		t.Fatalf("Listen() error = %v", err)
	}
	defer server.Close()
	if server.Keys() != nil {
		t.Fatal("a TUI session has a key channel")
	}

	connection, reader := dialWatch(t, runtimeDir)
	defer connection.Close()
	_ = connection.SetReadDeadline(time.Now().Add(2 * time.Second))
	line, err := reader.ReadString('\n')
	if err != nil || strings.TrimSpace(line) != "unknown command" {
		t.Fatalf("read %q, %v", line, err)
	}
	if server.Watching() {
		t.Fatal("the TUI session took a watcher")
	}
}

func TestEndStillEndsAServedSession(t *testing.T) {
	t.Setenv("XDG_RUNTIME_DIR", t.TempDir())
	server, err := ListenWatched()
	if err != nil {
		t.Fatalf("ListenWatched() error = %v", err)
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

func TestCloseSendsWhatIsQueuedThenHangsUp(t *testing.T) {
	runtimeDir := t.TempDir()
	t.Setenv("XDG_RUNTIME_DIR", runtimeDir)
	server, err := ListenWatched()
	if err != nil {
		t.Fatalf("ListenWatched() error = %v", err)
	}
	connection, reader := dialWatch(t, runtimeDir)
	defer connection.Close()
	waitWatching(t, server, true)

	server.Publish([]byte(`{"type":"ended"}`))
	server.Close()

	_ = connection.SetReadDeadline(time.Now().Add(2 * time.Second))
	line, err := reader.ReadString('\n')
	if err != nil || strings.TrimSpace(line) != `{"type":"ended"}` {
		t.Fatalf("read %q, %v", line, err)
	}
	if _, err := reader.ReadString('\n'); err == nil {
		t.Fatal("the connection stayed open after Close")
	}
}

func TestAWatcherThatFallsBehindIsDropped(t *testing.T) {
	runtimeDir := t.TempDir()
	t.Setenv("XDG_RUNTIME_DIR", runtimeDir)
	server, err := ListenWatched()
	if err != nil {
		t.Fatalf("ListenWatched() error = %v", err)
	}
	defer server.Close()
	connection, _ := dialWatch(t, runtimeDir)
	defer connection.Close()
	waitWatching(t, server, true)

	// The client never reads: once the socket buffer and the queue are full,
	// Publish drops it instead of waiting.
	line := []byte(strings.Repeat("x", 64*1024))
	done := make(chan struct{})
	go func() {
		defer close(done)
		for i := 0; i < 4096 && server.Watching(); i++ {
			server.Publish(line)
		}
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Publish blocked on a stuck watcher")
	}
	if server.Watching() {
		t.Fatal("a stuck watcher was kept")
	}
}
