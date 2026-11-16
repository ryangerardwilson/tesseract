package snapshot

import (
	"sort"

	"github.com/ryangerardwilson/tesseract/internal/tmux"
)

// SessionsInRecencyOrder puts the most recently used session first.
// Restored sessions have no tmux last-attached time after a shutdown.
// The saved last-accessed time is the order the radial had before that.
func SessionsInRecencyOrder(home string, sessions []tmux.Session) []tmux.Session {
	saved, err := LoadSavedSnapshot(home)
	if err != nil || saved == nil || len(saved.Sessions) == 0 {
		return sessions
	}
	return orderSessionsByRecency(sessions, saved)
}

func orderSessionsByRecency(sessions []tmux.Session, saved *SavedSnapshot) []tmux.Session {
	accesses := savedSessionAccesses(saved)
	positions := savedSessionPositions(saved)
	out := append([]tmux.Session{}, sessions...)
	sort.SliceStable(out, func(i, j int) bool {
		left := sessionRecency(out[i], accesses)
		right := sessionRecency(out[j], accesses)
		if left != right {
			return left > right
		}
		return savedPosition(out[i].Name, positions) < savedPosition(out[j].Name, positions)
	})
	return out
}

func savedSessionPositions(saved *SavedSnapshot) map[string]int {
	positions := map[string]int{}
	if saved == nil {
		return positions
	}
	for index, session := range saved.Sessions {
		positions[session.Name] = index
	}
	return positions
}

func savedPosition(name string, positions map[string]int) int {
	if position, ok := positions[name]; ok {
		return position
	}
	return len(positions)
}

func sessionRecency(session tmux.Session, saved map[string]int64) int64 {
	if session.LastAttached > 0 {
		return int64(session.LastAttached)
	}
	if when := saved[session.Name]; when > 0 {
		return when
	}
	return int64(session.Created)
}
