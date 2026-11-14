package snapshot

import (
	"time"

	"github.com/ryangerardwilson/tesseract/internal/tmux"
)

const InactiveSessionDays = 10

const inactiveSessionMaxAge = InactiveSessionDays * 24 * time.Hour

func savedSessionAccesses(saved *SavedSnapshot) map[string]int64 {
	accesses := map[string]int64{}
	if saved == nil {
		return accesses
	}
	for _, session := range saved.Sessions {
		lastAccessed := session.LastAccessed
		if lastAccessed <= 0 {
			lastAccessed = saved.CapturedAt
		}
		if lastAccessed > 0 {
			accesses[session.Name] = lastAccessed
		}
	}
	return accesses
}

func runtimeSessionLastAccessed(session tmux.Session, previous int64, fallback int64) int64 {
	if session.LastAttached > 0 {
		return int64(session.LastAttached)
	}
	if previous > 0 {
		return previous
	}
	if session.Created > 0 {
		return int64(session.Created)
	}
	return fallback
}

func inactiveSessionNames(sessions []tmux.Session, previous map[string]int64, now time.Time) []string {
	cutoff := now.Add(-inactiveSessionMaxAge).Unix()
	inactive := make([]string, 0)
	for _, session := range sessions {
		if session.Name == tmux.IndexSessionName || session.Attached > 0 {
			continue
		}
		lastAccessed := runtimeSessionLastAccessed(session, previous[session.Name], now.Unix())
		if lastAccessed > 0 && lastAccessed < cutoff {
			inactive = append(inactive, session.Name)
		}
	}
	return inactive
}

func pruneInactiveLiveSessions(api *tmux.API, saved *SavedSnapshot, now time.Time) ([]string, error) {
	sessions, err := api.ListSessions(true)
	if err != nil {
		return nil, err
	}
	candidates := inactiveSessionNames(sessions, savedSessionAccesses(saved), now)
	pruned := make([]string, 0, len(candidates))
	for _, name := range candidates {
		killed, err := api.KillSessionIfDetached(name)
		if err != nil {
			return pruned, err
		}
		if killed {
			pruned = append(pruned, name)
		}
	}
	return pruned, nil
}

func retainedSavedSnapshot(saved *SavedSnapshot, now time.Time) (*SavedSnapshot, bool) {
	if saved == nil {
		return nil, false
	}
	retained := *saved
	retained.Sessions = make([]SessionSnapshot, 0, len(saved.Sessions))
	cutoff := now.Add(-inactiveSessionMaxAge).Unix()
	changed := false
	for _, session := range saved.Sessions {
		lastAccessed := session.LastAccessed
		if lastAccessed <= 0 {
			lastAccessed = saved.CapturedAt
			if lastAccessed <= 0 {
				lastAccessed = now.Unix()
			}
			changed = true
		}
		session.LastAccessed = lastAccessed
		if session.Name != tmux.IndexSessionName && lastAccessed < cutoff {
			changed = true
			continue
		}
		retained.Sessions = append(retained.Sessions, session)
	}
	return &retained, changed
}

func loadRetainedSavedSnapshot(api *tmux.API, now time.Time) (*SavedSnapshot, error) {
	saved, err := LoadSavedSnapshot(api.Env["HOME"])
	if err != nil || saved == nil {
		return saved, err
	}
	retained, changed := retainedSavedSnapshot(saved, now)
	if changed {
		if err := writeSavedSnapshot(api.Env["HOME"], retained); err != nil {
			return nil, err
		}
	}
	return retained, nil
}
