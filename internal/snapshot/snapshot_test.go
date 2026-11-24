package snapshot

import (
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
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
			pane: PaneSnapshot{Kind: "codex", ResumeID: "codex-thread"},
			want: `"${SHELL:-/bin/sh}" -lc 'codex resume '"'"'codex-thread'"'"''; exec "${SHELL:-/bin/sh}"`,
		},
		{
			name: "glm through codex",
			pane: PaneSnapshot{Kind: "codex", ResumeID: "glm-thread", CodexProfile: "zai"},
			want: `"${SHELL:-/bin/sh}" -lc 'codex --profile '"'"'zai'"'"' resume '"'"'glm-thread'"'"''; exec "${SHELL:-/bin/sh}"`,
		},
		{
			name: "grok",
			pane: PaneSnapshot{Kind: "grok", ResumeID: "grok-session"},
			want: `"${SHELL:-/bin/sh}" -lc 'grok --resume '"'"'grok-session'"'"''; exec "${SHELL:-/bin/sh}"`,
		},
		{
			name: "c3po tui",
			pane: PaneSnapshot{Kind: "c3po", ResumeID: "01a17622-5fd9-7bc2-81ff-3e200a8c16b7", Screen: "o"},
			want: `"${SHELL:-/bin/sh}" -lc 'r2d2 '"'"'o'"'"' --resume '"'"'01a17622-5fd9-7bc2-81ff-3e200a8c16b7'"'"''; exec "${SHELL:-/bin/sh}"`,
		},
		{
			name: "r2d2 screen",
			pane: PaneSnapshot{Kind: "r2d2", ResumeID: "01a17622-5fd9-7bc2-81ff-3e200a8c16b7", Screen: "o"},
			want: `"${SHELL:-/bin/sh}" -lc 'r2d2 '"'"'o'"'"' --resume '"'"'01a17622-5fd9-7bc2-81ff-3e200a8c16b7'"'"''; exec "${SHELL:-/bin/sh}"`,
		},
		{
			name: "r2d2 without a screen",
			pane: PaneSnapshot{Kind: "r2d2", ResumeID: "01a17622-5fd9-7bc2-81ff-3e200a8c16b7"},
			want: "",
		},
		{
			name: "c3po without a screen",
			pane: PaneSnapshot{Kind: "c3po", ResumeID: "01a17622-5fd9-7bc2-81ff-3e200a8c16b7"},
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

// tmux gives a new pane the PATH of the client that created it. At boot that
// client can be the omarchy bar, whose status poll restores sessions with a
// bare PATH, so a restored agent must not depend on the restoring PATH.
func TestRestoreResumesAgentsWhenTmuxPathIsBare(t *testing.T) {
	if _, err := exec.LookPath("tmux"); err != nil {
		t.Skip("tmux is not installed")
	}
	bash, err := exec.LookPath("bash")
	if err != nil {
		t.Skip("bash is not installed")
	}
	const id = "01a1e190-c2f6-7332-9ceb-cd46670c350b"
	tests := []struct {
		tool string
		pane PaneSnapshot
		args []string
	}{
		{tool: "grok", pane: PaneSnapshot{Kind: "grok", ResumeID: id}, args: []string{"--resume", id}},
		{tool: "codex", pane: PaneSnapshot{Kind: "codex", ResumeID: id, CodexProfile: "zai"}, args: []string{"--profile", "zai", "resume", id}},
		{tool: "r2d2", pane: PaneSnapshot{Kind: "r2d2", ResumeID: id, Screen: "o"}, args: []string{"o", "--resume", id}},
	}

	for _, test := range tests {
		t.Run(test.tool, func(t *testing.T) {
			home := loginProfileHome(t, bash, test.tool)
			api := bareTmuxServer(t, home)
			pane := test.pane
			pane.CWD = filepath.Join(home, "workspace")
			saved := &SavedSnapshot{Sessions: []SessionSnapshot{{
				Name:    "agent",
				Windows: []WindowSnapshot{{Name: test.tool, Panes: []PaneSnapshot{pane}}},
			}}}
			if result := RestoreSnapshot(api, saved); len(result.RestoredSessions) != 1 {
				t.Fatalf("RestoreSnapshot() = %#v, want the agent session restored", result)
			}
			waitForFile(t, filepath.Join(home, "fallback"))
			data, err := os.ReadFile(filepath.Join(home, test.tool+".args"))
			if err != nil {
				t.Fatalf("%s did not run: %v", test.tool, err)
			}
			got := strings.Split(strings.TrimSuffix(string(data), "\n"), "\n")
			want := append([]string{pane.CWD}, test.args...)
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("%s ran as %q, want %q", test.tool, got, want)
			}
		})
	}
}

// loginProfileHome is a home whose login profile alone puts tool on PATH. Its
// shell records the fallback shell, the one started with no arguments.
func loginProfileHome(t *testing.T, bash, tool string) string {
	t.Helper()
	home := t.TempDir()
	for _, dir := range []string{"bare", "tools", "workspace"} {
		if err := os.Mkdir(filepath.Join(home, dir), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	files := []struct {
		name string
		mode os.FileMode
		text string
	}{
		{name: ".bash_profile", mode: 0o644, text: `PATH="$HOME/tools:$PATH"` + "\n"},
		{name: filepath.Join("tools", tool), mode: 0o755, text: "#!/bin/sh\nprintf '%s\\n' \"$PWD\" \"$@\" > \"$HOME/" + tool + ".args\"\n"},
		{name: "shell", mode: 0o755, text: "#!" + bash + "\n[ \"$#\" -gt 0 ] || : > \"$HOME/fallback\"\nexec " + bash + " \"$@\"\n"},
	}
	for _, file := range files {
		if err := os.WriteFile(filepath.Join(home, file.name), []byte(file.text), file.mode); err != nil {
			t.Fatal(err)
		}
	}
	return home
}

// bareTmuxServer starts an isolated tmux server and returns an API that, like
// the bar's status poll, reaches it with a PATH that holds none of the tools.
func bareTmuxServer(t *testing.T, home string) *tmux.API {
	t.Helper()
	sleep, err := exec.LookPath("sleep")
	if err != nil {
		t.Skip("sleep is not installed")
	}
	socketPath := filepath.Join(home, "tmux.sock")
	bare := filepath.Join(home, "bare")
	start := exec.Command("tmux", "-S", socketPath, "-f", "/dev/null", "new-session", "-d", "-s", "bar", sleep, "60")
	start.Env = []string{"HOME=" + home, "PATH=" + bare, "SHELL=" + filepath.Join(home, "shell")}
	if output, err := start.CombinedOutput(); err != nil {
		t.Fatalf("start isolated tmux server: %v: %s", err, output)
	}
	t.Cleanup(func() {
		_ = exec.Command("tmux", "-S", socketPath, "kill-server").Run()
	})
	return tmux.NewAPI(map[string]string{"HOME": home, "PATH": bare, "TMUX": socketPath + ",0,0"})
}

func waitForFile(t *testing.T, path string) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for {
		if _, err := os.Stat(path); err == nil {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("%s did not appear", path)
		}
		time.Sleep(20 * time.Millisecond)
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

func TestPaneSnapshotForResumePreservesCodexProfile(t *testing.T) {
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

			got := paneSnapshotForResume(api, pane, "agent-thread", processes, children)
			if got.Kind != test.wantKind {
				t.Fatalf("Kind = %q, want %q", got.Kind, test.wantKind)
			}
			if got.CodexProfile != test.wantProfile {
				t.Fatalf("CodexProfile = %q, want %q", got.CodexProfile, test.wantProfile)
			}
			if got.ResumeID != "agent-thread" {
				t.Fatalf("ResumeID = %q, want agent-thread", got.ResumeID)
			}
		})
	}
}

func TestApplyGrokWorkspaceUsesSessionDirectory(t *testing.T) {
	workspace := t.TempDir()
	pane := PaneSnapshot{CWD: "/home/ryan", Kind: "grok", ResumeID: "01a16844-3a1f-7331-a0f7-73684c4ac73d"}
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
	codex := applyGrokWorkspace(PaneSnapshot{CWD: "/work", Kind: "codex", ResumeID: "thread"}, workspace)
	if codex.CWD != "/work" {
		t.Fatalf("codex CWD = %q", codex.CWD)
	}
}
