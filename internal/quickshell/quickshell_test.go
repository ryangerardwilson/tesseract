package quickshell

import (
	"encoding/json"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/ryangerardwilson/tesseract/internal/tmux"
)

func TestValidateSessionName(t *testing.T) {
	tests := []struct {
		name    string
		want    string
		wantErr bool
	}{
		{name: " root ", want: "root"},
		{name: "", wantErr: true},
		{name: "index", wantErr: true},
		{name: "bad:name", wantErr: true},
		{name: "bad\nname", wantErr: true},
	}
	for _, test := range tests {
		got, err := ValidateSessionName(test.name)
		if test.wantErr {
			if err == nil {
				t.Fatalf("ValidateSessionName(%q) succeeded", test.name)
			}
			continue
		}
		if err != nil {
			t.Fatalf("ValidateSessionName(%q) failed: %v", test.name, err)
		}
		if got != test.want {
			t.Fatalf("ValidateSessionName(%q) = %q, want %q", test.name, got, test.want)
		}
	}
}

// Whether work is unseen is snapshot's rule (agentUnseen, which also covers
// working and agentless sessions). The payload only carries its answer.
func TestPayloadItemAgentUnseen(t *testing.T) {
	tests := []struct {
		name   string
		status tmux.AgentStatus
		unseen bool
		want   int
	}{
		{name: "idle with unseen work", status: tmux.AgentStatus{Total: 2, Idle: 2}, unseen: true, want: 1},
		{name: "idle and seen", status: tmux.AgentStatus{Total: 1, Idle: 1}, want: 0},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			item := payloadItem(tmux.Session{Name: "alpha"}, test.status, test.unseen)
			data, err := json.Marshal(item)
			if err != nil {
				t.Fatalf("marshal payload item: %v", err)
			}
			var fields map[string]any
			if err := json.Unmarshal(data, &fields); err != nil {
				t.Fatalf("unmarshal payload item: %v", err)
			}
			if got := fields["agent_unseen"]; got != float64(test.want) {
				t.Fatalf("agent_unseen = %v, want %d in %s", got, test.want, data)
			}
		})
	}
}

func TestQuickshellListIncludesAgentUnseen(t *testing.T) {
	if _, err := exec.LookPath("tmux"); err != nil {
		t.Skip("tmux is not installed")
	}
	home := t.TempDir()
	t.Setenv("SQLX_ROOT", filepath.Join(home, "sqlx"))
	socketPath := filepath.Join(home, "tmux.sock")
	start := exec.Command("tmux", "-S", socketPath, "-f", "/dev/null", "new-session", "-d", "-s", "alpha", "sleep", "60")
	if output, err := start.CombinedOutput(); err != nil {
		t.Fatalf("start isolated tmux server: %v: %s", err, output)
	}
	t.Cleanup(func() {
		_ = exec.Command("tmux", "-S", socketPath, "kill-server").Run()
	})
	api := tmux.NewAPI(nil)
	api.Env["HOME"] = home
	api.Env["TMUX"] = socketPath + ",0,0"

	encoded, err := APIPayload(api, []string{"list"})
	if err != nil {
		t.Fatalf("APIPayload(list) failed: %v", err)
	}
	data, err := json.Marshal(encoded)
	if err != nil {
		t.Fatal(err)
	}
	var payload map[string]any
	if err := json.Unmarshal(data, &payload); err != nil {
		t.Fatal(err)
	}
	sessions, _ := payload["sessions"].([]any)
	if len(sessions) != 1 {
		t.Fatalf("sessions = %#v, want one", payload["sessions"])
	}
	session, _ := sessions[0].(map[string]any)
	for _, key := range []string{"agent_total", "agent_working", "agent_idle", "last_attached", "agent_unseen"} {
		if _, ok := session[key]; !ok {
			t.Fatalf("session %#v has no %s", session, key)
		}
	}
	if session["agent_unseen"] != float64(0) {
		t.Fatalf("agent_unseen = %v, want 0 without agent panes", session["agent_unseen"])
	}
}
