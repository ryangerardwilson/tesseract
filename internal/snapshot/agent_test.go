package snapshot

import (
	"testing"
	"time"
)

type agentPoll struct {
	at       int64
	attached bool
	total    int
	working  int
}

func TestNextAgentStateMarksUnseenWork(t *testing.T) {
	tests := []struct {
		name        string
		start       agentState
		polls       []agentPoll
		wantSettled int64
		wantSeen    int64
		wantUnseen  bool
	}{
		{
			name: "working then idle while detached",
			polls: []agentPoll{
				{at: 100, total: 1, working: 1},
				{at: 110, total: 1},
			},
			wantSettled: 110,
			wantUnseen:  true,
		},
		{
			name: "a later idle poll keeps the settled time",
			polls: []agentPoll{
				{at: 100, total: 1, working: 1},
				{at: 110, total: 1},
				{at: 120, total: 1},
				{at: 130, total: 2},
			},
			wantSettled: 110,
			wantUnseen:  true,
		},
		{
			name: "attached while it settles",
			polls: []agentPoll{
				{at: 100, attached: true, total: 1, working: 1},
				{at: 110, attached: true, total: 1},
			},
			wantSettled: 110,
			wantSeen:    110,
		},
		{
			name: "left before it finished",
			polls: []agentPoll{
				{at: 100, attached: true, total: 1, working: 1},
				{at: 105, total: 1, working: 1},
				{at: 110, total: 1},
			},
			wantSettled: 110,
			wantSeen:    100,
			wantUnseen:  true,
		},
		{
			name: "attach after it settled",
			polls: []agentPoll{
				{at: 100, total: 1, working: 1},
				{at: 110, total: 1},
				{at: 120, attached: true, total: 1},
			},
			wantSettled: 110,
			wantSeen:    120,
		},
		{
			name:  "working with an older settled time",
			start: agentState{SettledAt: 50, SeenAt: 10},
			polls: []agentPoll{
				{at: 100, total: 2, working: 1},
			},
			wantSettled: 50,
			wantSeen:    10,
		},
		{
			name:  "no agent panes clears the settled time",
			start: agentState{WasWorking: true, SettledAt: 50, SeenAt: 10},
			polls: []agentPoll{
				{at: 100},
			},
			wantSeen: 10,
		},
		{
			name: "an agent that never worked is not unseen",
			polls: []agentPoll{
				{at: 100, total: 1},
				{at: 110, total: 1},
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			state := test.start
			var last AgentActivity
			for _, poll := range test.polls {
				last = AgentActivity{Name: "alpha", Attached: poll.attached, Total: poll.total, Working: poll.working}
				state = nextAgentState(state, last, poll.at)
			}
			if state.SettledAt != test.wantSettled {
				t.Fatalf("settled at = %d, want %d", state.SettledAt, test.wantSettled)
			}
			if state.SeenAt != test.wantSeen {
				t.Fatalf("seen at = %d, want %d", state.SeenAt, test.wantSeen)
			}
			if got := agentUnseen(state, last); got != test.wantUnseen {
				t.Fatalf("agentUnseen() = %v, want %v", got, test.wantUnseen)
			}
		})
	}
}

func TestSnapshotRewriteKeepsAgentState(t *testing.T) {
	tests := []struct {
		name    string
		rewrite func(t *testing.T, home string)
	}{
		{
			name: "runtime snapshot without agent fields",
			rewrite: func(t *testing.T, home string) {
				rewritten := &SavedSnapshot{
					Version:    Version,
					CapturedAt: 2_000_000_200,
					Sessions: []SessionSnapshot{
						{Name: "beta", LastAccessed: 2_000_000_200},
						{Name: "alpha", LastAccessed: 2_000_000_200},
					},
				}
				if err := writeSavedSnapshot(home, rewritten); err != nil {
					t.Fatalf("rewrite snapshot: %v", err)
				}
			},
		},
		{
			name: "snapshot read before the poll wrote",
			rewrite: func(t *testing.T, home string) {
				stale := agentTestSnapshot()
				stale.Sessions[0].AgentSettledAt = 1
				stale.Sessions[0].AgentSeenAt = 1
				if err := writeSavedSnapshot(home, stale); err != nil {
					t.Fatalf("rewrite snapshot: %v", err)
				}
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			home := t.TempDir()
			isolateSQLX(t, home)
			if err := writeSavedSnapshot(home, agentTestSnapshot()); err != nil {
				t.Fatalf("write snapshot: %v", err)
			}
			polls := []struct {
				at       int64
				attached bool
				working  int
			}{
				{at: 2_000_000_000, attached: true, working: 1},
				{at: 2_000_000_050, working: 1},
				{at: 2_000_000_100},
			}
			var unseen map[string]bool
			for _, poll := range polls {
				activity := []AgentActivity{
					{Name: "alpha", Attached: poll.attached, Total: 1, Working: poll.working},
					{Name: "missing", Total: 1},
				}
				var err error
				unseen, err = RecordAgentActivity(home, activity, time.Unix(poll.at, 0))
				if err != nil {
					t.Fatalf("RecordAgentActivity() failed: %v", err)
				}
			}
			if !unseen["alpha"] || unseen["missing"] {
				t.Fatalf("unseen = %v, want only alpha", unseen)
			}

			test.rewrite(t, home)

			saved, err := LoadSavedSnapshot(home)
			if err != nil {
				t.Fatalf("reload snapshot: %v", err)
			}
			alpha := agentTestSession(t, saved, "alpha")
			if alpha.AgentSettledAt != 2_000_000_100 || alpha.AgentSeenAt != 2_000_000_000 {
				t.Fatalf("alpha agent times = %d/%d, want 2000000100/2000000000", alpha.AgentSettledAt, alpha.AgentSeenAt)
			}
			unseen, err = RecordAgentActivity(home, []AgentActivity{{Name: "alpha", Total: 1}}, time.Unix(2_000_000_300, 0))
			if err != nil {
				t.Fatalf("RecordAgentActivity() after rewrite failed: %v", err)
			}
			if !unseen["alpha"] {
				t.Fatal("alpha lost its unseen work across the rewrite")
			}
			saved, err = LoadSavedSnapshot(home)
			if err != nil {
				t.Fatalf("reload snapshot: %v", err)
			}
			if got := agentTestSession(t, saved, "alpha").AgentSettledAt; got != 2_000_000_100 {
				t.Fatalf("idle poll after rewrite moved settled at to %d", got)
			}
		})
	}
}

func agentTestSnapshot() *SavedSnapshot {
	return &SavedSnapshot{
		Version:    Version,
		CapturedAt: 2_000_000_000,
		Sessions: []SessionSnapshot{
			{Name: "alpha", LastAccessed: 2_000_000_000},
		},
	}
}

func agentTestSession(t *testing.T, saved *SavedSnapshot, name string) SessionSnapshot {
	t.Helper()
	if saved != nil {
		for _, session := range saved.Sessions {
			if session.Name == name {
				return session
			}
		}
	}
	t.Fatalf("saved snapshot has no session %q", name)
	return SessionSnapshot{}
}
