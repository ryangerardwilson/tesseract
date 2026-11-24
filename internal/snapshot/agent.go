package snapshot

import (
	"fmt"
	"strings"
	"time"
)

// AgentActivity is what one quickshell poll saw in a session.
type AgentActivity struct {
	Name     string
	Attached bool
	Total    int
	Working  int
}

// agentState is a saved session's session_agent row, which only this poll
// writes. WasWorking is the previous poll's answer, so the poll can see the
// moment work stops. Zero timestamps are stored as NULL.
type agentState struct {
	WasWorking bool
	SettledAt  int64
	SeenAt     int64
}

type agentRow struct {
	Name       string `json:"name"`
	WasWorking *int64 `json:"was_working"`
	SettledAt  *int64 `json:"settled_at"`
	SeenAt     *int64 `json:"seen_at"`
}

func nextAgentState(prev agentState, activity AgentActivity, now int64) agentState {
	next := prev
	working := activity.Total > 0 && activity.Working > 0
	if activity.Total == 0 {
		next.SettledAt = 0
	} else if prev.WasWorking && !working {
		next.SettledAt = now
	}
	next.WasWorking = working
	if activity.Attached {
		next.SeenAt = now
	}
	return next
}

func agentUnseen(state agentState, activity AgentActivity) bool {
	return activity.Total > 0 && activity.Working == 0 && state.SettledAt > state.SeenAt
}

// RecordAgentActivity advances the saved agent state of each session that
// has a snapshot row and reports which of them have unseen work. It does
// not publish the database: the next snapshot carries these columns.
func RecordAgentActivity(home string, activity []AgentActivity, now time.Time) (map[string]bool, error) {
	if len(activity) == 0 {
		return nil, nil
	}
	dbPath, err := openDB(home)
	if err != nil {
		return nil, err
	}
	saved, err := readAgentStates(dbPath)
	if err != nil {
		return nil, err
	}
	unseen := map[string]bool{}
	var updates strings.Builder
	for _, current := range activity {
		prev, ok := saved[current.Name]
		if !ok {
			continue
		}
		next := nextAgentState(prev, current, now.Unix())
		unseen[current.Name] = agentUnseen(next, current)
		if next != prev {
			writeAgentUpdate(&updates, current.Name, next)
		}
	}
	if updates.Len() == 0 {
		return unseen, nil
	}
	if err := sqliteExec(dbPath, "BEGIN IMMEDIATE;\n"+updates.String()+"COMMIT;\n"); err != nil {
		return nil, err
	}
	return unseen, nil
}

// readAgentStates returns the agent state of every saved session. A session
// the poll has not written yet has the zero state.
func readAgentStates(dbPath string) (map[string]agentState, error) {
	var rows []agentRow
	if err := sqliteQueryJSON(dbPath, "SELECT s.name, a.was_working, a.settled_at, a.seen_at FROM session_snapshot AS s LEFT JOIN session_agent AS a ON a.session_id = s.id", &rows); err != nil {
		return nil, err
	}
	saved := make(map[string]agentState, len(rows))
	for _, row := range rows {
		saved[row.Name] = agentState{
			WasWorking: derefInt(row.WasWorking) != 0,
			SettledAt:  derefInt(row.SettledAt),
			SeenAt:     derefInt(row.SeenAt),
		}
	}
	return saved, nil
}

func writeAgentUpdate(b *strings.Builder, name string, state agentState) {
	wasWorking := int64(0)
	if state.WasWorking {
		wasWorking = 1
	}
	fmt.Fprintf(b, "INSERT INTO session_agent(session_id, was_working, settled_at, seen_at) SELECT id, %d, %s, %s FROM session_snapshot WHERE name = %s\n"+
		"ON CONFLICT(session_id) DO UPDATE SET was_working = excluded.was_working, settled_at = excluded.settled_at, seen_at = excluded.seen_at;\n",
		wasWorking,
		sqlIntOrNull(state.SettledAt),
		sqlIntOrNull(state.SeenAt),
		sqlString(name),
	)
}

func derefInt(value *int64) int64 {
	if value == nil {
		return 0
	}
	return *value
}
