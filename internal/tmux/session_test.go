package tmux

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestParseSessionsIncludesCreationTime(t *testing.T) {
	sessions := parseSessions("older\t0\t1\t100\t50\nnewer\t1\t2\t200\t150\n")
	if len(sessions) != 2 {
		t.Fatalf("parseSessions() returned %d sessions, want 2", len(sessions))
	}
	if sessions[0].Name != "newer" || sessions[0].Created != 150 {
		t.Fatalf("first session = %#v, want newer with creation time 150", sessions[0])
	}
	if sessions[1].Name != "older" || sessions[1].Created != 50 {
		t.Fatalf("second session = %#v, want older with creation time 50", sessions[1])
	}
}

func TestKillSessionIfDetached(t *testing.T) {
	if _, err := exec.LookPath("tmux"); err != nil {
		t.Skip("tmux is not installed")
	}
	socketPath := filepath.Join(t.TempDir(), "tmux.sock")
	start := exec.Command("tmux", "-S", socketPath, "-f", "/dev/null", "new-session", "-d", "-s", "stale", "sleep", "60")
	if output, err := start.CombinedOutput(); err != nil {
		t.Fatalf("start isolated tmux server: %v: %s", err, output)
	}
	t.Cleanup(func() {
		_ = exec.Command("tmux", "-S", socketPath, "kill-server").Run()
	})

	env := envMap(os.Environ())
	env["TMUX"] = socketPath + ",0,0"
	api := NewAPI(env)
	killed, err := api.KillSessionIfDetached("stale")
	if err != nil {
		t.Fatalf("KillSessionIfDetached() failed: %v", err)
	}
	if !killed {
		t.Fatal("KillSessionIfDetached() did not kill detached session")
	}
	if api.HasSession("stale") {
		t.Fatal("detached session still exists after KillSessionIfDetached()")
	}
}
