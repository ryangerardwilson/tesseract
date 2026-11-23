package snapshot

import (
	"os/exec"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/ryangerardwilson/tesseract/internal/tmux"
)

func isolateSQLX(t *testing.T, home string) {
	t.Helper()
	t.Setenv("SQLX_ROOT", filepath.Join(home, "sqlx"))
}

func TestRestoreCommandReturnsToShellAfterAgentExit(t *testing.T) {
	tests := []struct {
		name string
		pane PaneSnapshot
		want string
	}{
		{
			name: "codex",
			pane: PaneSnapshot{Kind: "codex", CodexThreadID: "codex-thread"},
			want: `"${SHELL:-/bin/sh}" -lc 'codex resume '"'"'codex-thread'"'"''; exec "${SHELL:-/bin/sh}"`,
		},
		{
			name: "glm through codex",
			pane: PaneSnapshot{Kind: "codex", CodexThreadID: "glm-thread", CodexProfile: "zai"},
			want: `"${SHELL:-/bin/sh}" -lc 'codex --profile '"'"'zai'"'"' resume '"'"'glm-thread'"'"''; exec "${SHELL:-/bin/sh}"`,
		},
		{
			name: "grok",
			pane: PaneSnapshot{Kind: "grok", CodexThreadID: "grok-session"},
			want: `"${SHELL:-/bin/sh}" -lc 'grok --resume '"'"'grok-session'"'"''; exec "${SHELL:-/bin/sh}"`,
		},
		{
			name: "c3po tui",
			pane: PaneSnapshot{Kind: "c3po", CodexThreadID: "01a17622-5fd9-7bc2-81ff-3e200a8c16b7", Screen: "o"},
			want: `"${SHELL:-/bin/sh}" -lc 'r2d2 '"'"'o'"'"' --resume '"'"'01a17622-5fd9-7bc2-81ff-3e200a8c16b7'"'"''; exec "${SHELL:-/bin/sh}"`,
		},
		{
			name: "r2d2 screen",
			pane: PaneSnapshot{Kind: "r2d2", CodexThreadID: "01a17622-5fd9-7bc2-81ff-3e200a8c16b7", Screen: "o"},
			want: `"${SHELL:-/bin/sh}" -lc 'r2d2 '"'"'o'"'"' --resume '"'"'01a17622-5fd9-7bc2-81ff-3e200a8c16b7'"'"''; exec "${SHELL:-/bin/sh}"`,
		},
		{
			name: "r2d2 without a screen",
			pane: PaneSnapshot{Kind: "r2d2", CodexThreadID: "01a17622-5fd9-7bc2-81ff-3e200a8c16b7"},
			want: "",
		},
		{
			name: "c3po without a screen",
			pane: PaneSnapshot{Kind: "c3po", CodexThreadID: "01a17622-5fd9-7bc2-81ff-3e200a8c16b7"},
			want: "",
		},
		{
			name: "ordinary shell pane",
			pane: PaneSnapshot{Kind: "shell"},
			want: "",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := restoreCommand(test.pane); got != test.want {
				t.Fatalf("restoreCommand() = %q, want %q", got, test.want)
			}
		})
	}
}

func TestInactiveSessionNames(t *testing.T) {
	now := time.Unix(2_000_000_000, 0)
	cutoff := now.Add(-inactiveSessionMaxAge).Unix()
	sessions := []tmux.Session{
		{Name: "stale", LastAttached: int(cutoff - 1)},
		{Name: "exact", LastAttached: int(cutoff)},
		{Name: "recent", LastAttached: int(cutoff + 1)},
		{Name: "attached", Attached: 1, LastAttached: int(cutoff - 100)},
		{Name: tmux.IndexSessionName, LastAttached: int(cutoff - 100)},
		{Name: "restored-stale", Created: int(now.Unix())},
		{Name: "restored-recent", Created: int(cutoff - 100)},
		{Name: "new-stale", Created: int(cutoff - 1)},
		{Name: "unknown"},
	}
	previous := map[string]int64{
		"restored-stale":  cutoff - 1,
		"restored-recent": cutoff + 1,
	}
	want := []string{"stale", "restored-stale", "new-stale"}
	if got := inactiveSessionNames(sessions, previous, now); !reflect.DeepEqual(got, want) {
		t.Fatalf("inactiveSessionNames() = %v, want %v", got, want)
	}
}

func TestRetainedSavedSnapshotMigratesAndPrunes(t *testing.T) {
	now := time.Unix(2_000_000_000, 0)
	cutoff := now.Add(-inactiveSessionMaxAge).Unix()
	saved := &SavedSnapshot{
		Version:    Version,
		CapturedAt: cutoff + 100,
		Sessions: []SessionSnapshot{
			{Name: "legacy"},
			{Name: "stale", LastAccessed: cutoff - 1},
			{Name: "exact", LastAccessed: cutoff},
			{Name: tmux.IndexSessionName, LastAccessed: cutoff - 100},
		},
	}

	retained, changed := retainedSavedSnapshot(saved, now)
	if !changed {
		t.Fatal("retainedSavedSnapshot() did not report migration and pruning")
	}
	gotNames := make([]string, 0, len(retained.Sessions))
	for _, session := range retained.Sessions {
		gotNames = append(gotNames, session.Name)
	}
	wantNames := []string{"legacy", "exact", tmux.IndexSessionName}
	if !reflect.DeepEqual(gotNames, wantNames) {
		t.Fatalf("retained sessions = %v, want %v", gotNames, wantNames)
	}
	if retained.Sessions[0].LastAccessed != saved.CapturedAt {
		t.Fatalf("legacy last access = %d, want snapshot capture time %d", retained.Sessions[0].LastAccessed, saved.CapturedAt)
	}
	if saved.Sessions[0].LastAccessed != 0 {
		t.Fatal("retainedSavedSnapshot() mutated the input snapshot")
	}
}

func TestWriteRuntimeSnapshotPrunesOnlyStaleSession(t *testing.T) {
	if _, err := exec.LookPath("tmux"); err != nil {
		t.Skip("tmux is not installed")
	}
	tempHome := t.TempDir()
	isolateSQLX(t, tempHome)
	socketPath := filepath.Join(tempHome, "tmux.sock")
	start := exec.Command("tmux", "-S", socketPath, "-f", "/dev/null", "new-session", "-d", "-s", "recent", "sleep", "60")
	if output, err := start.CombinedOutput(); err != nil {
		t.Fatalf("start isolated tmux server: %v: %s", err, output)
	}
	t.Cleanup(func() {
		_ = exec.Command("tmux", "-S", socketPath, "kill-server").Run()
	})
	createStale := exec.Command("tmux", "-S", socketPath, "new-session", "-d", "-s", "stale", "sleep", "60")
	if output, err := createStale.CombinedOutput(); err != nil {
		t.Fatalf("create stale session: %v: %s", err, output)
	}

	api := tmux.NewAPI(nil)
	api.Env["HOME"] = tempHome
	api.Env["TMUX"] = socketPath + ",0,0"
	now := time.Now()
	cutoff := now.Add(-inactiveSessionMaxAge).Unix()
	previous := &SavedSnapshot{
		Version:    Version,
		CapturedAt: now.Unix(),
		Sessions: []SessionSnapshot{
			{Name: "recent", LastAccessed: cutoff + 1},
			{Name: "stale", LastAccessed: cutoff - 1},
		},
	}
	if err := writeSavedSnapshot(tempHome, previous); err != nil {
		t.Fatalf("write previous snapshot: %v", err)
	}

	got, err := writeRuntimeSnapshotAt(api, nil, now)
	if err != nil {
		t.Fatalf("writeRuntimeSnapshotAt() failed: %v", err)
	}
	if api.HasSession("stale") {
		t.Fatal("stale detached session still exists")
	}
	if !api.HasSession("recent") {
		t.Fatal("recent detached session was pruned")
	}
	if len(got.Sessions) != 1 || got.Sessions[0].Name != "recent" {
		t.Fatalf("saved sessions = %#v, want only recent", got.Sessions)
	}
	if got.Sessions[0].LastAccessed != cutoff+1 {
		t.Fatalf("recent last access = %d, want %d", got.Sessions[0].LastAccessed, cutoff+1)
	}
}

func TestLoadRetainedSavedSnapshotRemovesStaleDurably(t *testing.T) {
	tempHome := t.TempDir()
	isolateSQLX(t, tempHome)
	now := time.Unix(2_000_000_000, 0)
	cutoff := now.Add(-inactiveSessionMaxAge).Unix()
	saved := &SavedSnapshot{
		Version:    Version,
		CapturedAt: now.Unix(),
		Sessions: []SessionSnapshot{
			{Name: "stale", LastAccessed: cutoff - 1},
			{Name: "recent", LastAccessed: cutoff + 1},
		},
	}
	if err := writeSavedSnapshot(tempHome, saved); err != nil {
		t.Fatalf("write saved snapshot: %v", err)
	}
	api := tmux.NewAPI(nil)
	api.Env["HOME"] = tempHome

	retained, err := loadRetainedSavedSnapshot(api, now)
	if err != nil {
		t.Fatalf("loadRetainedSavedSnapshot() failed: %v", err)
	}
	if len(retained.Sessions) != 1 || retained.Sessions[0].Name != "recent" {
		t.Fatalf("retained sessions = %#v, want only recent", retained.Sessions)
	}
	reloaded, err := LoadSavedSnapshot(tempHome)
	if err != nil {
		t.Fatalf("reload saved snapshot: %v", err)
	}
	if len(reloaded.Sessions) != 1 || reloaded.Sessions[0].Name != "recent" {
		t.Fatalf("durable sessions = %#v, want only recent", reloaded.Sessions)
	}
}

func TestEmptyRuntimeDoesNotOverwriteSavedSnapshot(t *testing.T) {
	tempHome := t.TempDir()
	isolateSQLX(t, tempHome)
	saved := &SavedSnapshot{
		Version:    Version,
		CapturedAt: time.Now().Unix(),
		Sessions:   []SessionSnapshot{{Name: "preserve", LastAccessed: time.Now().Unix()}},
	}
	if err := writeSavedSnapshot(tempHome, saved); err != nil {
		t.Fatalf("write saved snapshot: %v", err)
	}
	before, err := LoadSavedSnapshot(tempHome)
	if err != nil {
		t.Fatalf("read saved snapshot: %v", err)
	}
	if before == nil {
		t.Fatal("saved snapshot was not stored")
	}
	api := tmux.NewAPI(nil)
	api.Env["HOME"] = tempHome
	api.Env["TMUX"] = filepath.Join(tempHome, "missing.sock") + ",0,0"

	got, err := WriteRuntimeSnapshotIfNonempty(api, nil)
	if err != nil {
		t.Fatalf("WriteRuntimeSnapshotIfNonempty() failed: %v", err)
	}
	if got != nil {
		t.Fatalf("WriteRuntimeSnapshotIfNonempty() = %#v, want nil", got)
	}
	after, err := LoadSavedSnapshot(tempHome)
	if err != nil {
		t.Fatalf("read preserved snapshot: %v", err)
	}
	if !reflect.DeepEqual(before, after) {
		t.Fatalf("empty runtime overwrote the saved snapshot\nbefore %#v\nafter %#v", before, after)
	}
}

func TestPaneSnapshotForThreadPreservesCodexProfile(t *testing.T) {
	tests := []struct {
		name        string
		process     tmux.ProcessInfo
		wantKind    string
		wantProfile string
	}{
		{
			name:        "glm through codex",
			process:     tmux.ProcessInfo{PID: 11, PPID: 10, Comm: "codex", Args: "codex --profile zai"},
			wantKind:    "codex",
			wantProfile: "zai",
		},
		{
			name:        "default codex",
			process:     tmux.ProcessInfo{PID: 11, PPID: 10, Comm: "codex", Args: "codex"},
			wantKind:    "codex",
			wantProfile: "",
		},
		{
			name:        "grok",
			process:     tmux.ProcessInfo{PID: 11, PPID: 10, Comm: "grok", Args: "grok"},
			wantKind:    "grok",
			wantProfile: "",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			api := tmux.NewAPI(nil)
			api.Env["HOME"] = t.TempDir()
			pane := tmux.Pane{PanePID: 10, CurrentPath: "/work/project"}
			processes := map[int]tmux.ProcessInfo{
				10: {PID: 10, PPID: 9, Comm: "bash", Args: "bash"},
				11: test.process,
			}
			children := map[int][]int{10: {11}}

			got := paneSnapshotForThread(api, pane, "agent-thread", processes, children)
			if got.Kind != test.wantKind {
				t.Fatalf("Kind = %q, want %q", got.Kind, test.wantKind)
			}
			if got.CodexProfile != test.wantProfile {
				t.Fatalf("CodexProfile = %q, want %q", got.CodexProfile, test.wantProfile)
			}
			if got.CodexThreadID != "agent-thread" {
				t.Fatalf("CodexThreadID = %q, want agent-thread", got.CodexThreadID)
			}
		})
	}
}

func TestApplyGrokWorkspaceUsesSessionDirectory(t *testing.T) {
	workspace := t.TempDir()
	pane := PaneSnapshot{CWD: "/home/ryan", Kind: "grok", CodexThreadID: "01a16844-3a1f-7331-a0f7-73684c4ac73d"}
	got := applyGrokWorkspace(pane, workspace)
	if got.CWD != workspace {
		t.Fatalf("CWD = %q, want session workspace %q", got.CWD, workspace)
	}

	missing := applyGrokWorkspace(pane, filepath.Join(workspace, "gone"))
	if missing.CWD != "/home/ryan" {
		t.Fatalf("missing workspace CWD = %q", missing.CWD)
	}

	shell := applyGrokWorkspace(PaneSnapshot{CWD: "/home/ryan", Kind: "shell"}, workspace)
	if shell.CWD != "/home/ryan" {
		t.Fatalf("shell CWD = %q", shell.CWD)
	}
	codex := applyGrokWorkspace(PaneSnapshot{CWD: "/work", Kind: "codex", CodexThreadID: "thread"}, workspace)
	if codex.CWD != "/work" {
		t.Fatalf("codex CWD = %q", codex.CWD)
	}
}
