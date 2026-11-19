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

// agentState is the part of a session row that the list poll owns.
// agent_was_working is the previous poll's answer, so the poll can see
// the moment work stops. Zero timestamps are stored as NULL.
type agentState struct {
	WasWorking bool
	SettledAt  int64
	SeenAt     int64
}

type agentRow struct {
	Name           string `json:"name"`
	WasWorking     *int64 `json:"agent_was_working"`
	AgentSettledAt *int64 `json:"agent_settled_at"`
	AgentSeenAt    *int64 `json:"agent_seen_at"`
}

const agentColumns = "agent_was_working, agent_settled_at, agent_seen_at"

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
	var rows []agentRow
	if err := queryJSON(dbPath, "SELECT name, "+agentColumns+" FROM session", &rows); err != nil {
		return nil, err
	}
	saved := make(map[string]agentState, len(rows))
	for _, row := range rows {
		saved[row.Name] = agentState{
			WasWorking: derefInt(row.WasWorking) != 0,
			SettledAt:  derefInt(row.AgentSettledAt),
			SeenAt:     derefInt(row.AgentSeenAt),
		}
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

func writeAgentUpdate(b *strings.Builder, name string, state agentState) {
	wasWorking := int64(0)
	if state.WasWorking {
		wasWorking = 1
	}
	fmt.Fprintf(b, "UPDATE session SET agent_was_working = %d, agent_settled_at = %s, agent_seen_at = %s WHERE name = %s;\n",
		wasWorking,
		sqlIntOrNull(state.SettledAt),
		sqlIntOrNull(state.SeenAt),
		sqlString(name),
	)
}

// A snapshot replaces every session row. These statements keep the agent
// columns the list poll wrote, inside the same transaction, so a snapshot
// that read the rows earlier cannot roll them back.
func writeAgentKeep(b *strings.Builder) {
	b.WriteString("DROP TABLE IF EXISTS temp.agent_keep;\n")
	b.WriteString("CREATE TEMP TABLE agent_keep AS SELECT name, " + agentColumns + " FROM session;\n")
}

func writeAgentRestore(b *strings.Builder) {
	b.WriteString("UPDATE session SET agent_was_working = k.agent_was_working, agent_settled_at = k.agent_settled_at, agent_seen_at = k.agent_seen_at FROM agent_keep AS k WHERE k.name = session.name;\n")
	b.WriteString("DROP TABLE temp.agent_keep;\n")
}

func derefInt(value *int64) int64 {
	if value == nil {
		return 0
	}
	return *value
}
