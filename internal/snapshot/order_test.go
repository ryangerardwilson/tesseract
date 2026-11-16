package snapshot

import (
	"testing"

	"github.com/ryangerardwilson/tesseract/internal/tmux"
)

func TestOrderSessionsByRecencyKeepsSavedOrderWhenTmuxForgotIt(t *testing.T) {
	restoredAt := 1_800_000_000
	saved := &SavedSnapshot{
		CapturedAt: 1_700_000_000,
		Sessions: []SessionSnapshot{
			{Name: "alpha", LastAccessed: 100},
			{Name: "zebra", LastAccessed: 300},
			{Name: "middle", LastAccessed: 200},
		},
	}
	live := []tmux.Session{
		{Name: "alpha", Created: restoredAt},
		{Name: "middle", Created: restoredAt},
		{Name: "zebra", Created: restoredAt},
	}

	got := sessionNamesForTest(orderSessionsByRecency(live, saved))
	want := []string{"zebra", "middle", "alpha"}
	if !sameNames(got, want) {
		t.Fatalf("order = %v, want %v", got, want)
	}
}

func TestOrderSessionsByRecencyPrefersLiveAttach(t *testing.T) {
	saved := &SavedSnapshot{
		Sessions: []SessionSnapshot{
			{Name: "zebra", LastAccessed: 300},
			{Name: "alpha", LastAccessed: 100},
		},
	}
	live := []tmux.Session{
		{Name: "alpha", LastAttached: 1_900_000_000, Created: 1_800_000_000},
		{Name: "zebra", Created: 1_800_000_000},
	}

	got := sessionNamesForTest(orderSessionsByRecency(live, saved))
	want := []string{"alpha", "zebra"}
	if !sameNames(got, want) {
		t.Fatalf("order = %v, want %v", got, want)
	}
}

func TestOrderSessionsByRecencyUsesSnapshotOrderWhenAccessTies(t *testing.T) {
	saved := &SavedSnapshot{
		CapturedAt: 50,
		Sessions: []SessionSnapshot{
			{Name: "newest"},
			{Name: "older"},
		},
	}
	live := []tmux.Session{
		{Name: "older", Created: 1_800_000_000},
		{Name: "newest", Created: 1_800_000_000},
	}

	got := sessionNamesForTest(orderSessionsByRecency(live, saved))
	want := []string{"newest", "older"}
	if !sameNames(got, want) {
		t.Fatalf("order = %v, want %v", got, want)
	}
}

func TestOrderSessionsByRecencyPutsNewSessionFirst(t *testing.T) {
	saved := &SavedSnapshot{
		Sessions: []SessionSnapshot{
			{Name: "zebra", LastAccessed: 300},
		},
	}
	live := []tmux.Session{
		{Name: "zebra", Created: 1_800_000_000},
		{Name: "fresh", Created: 1_900_000_000},
	}

	got := sessionNamesForTest(orderSessionsByRecency(live, saved))
	want := []string{"fresh", "zebra"}
	if !sameNames(got, want) {
		t.Fatalf("order = %v, want %v", got, want)
	}
}

func TestSessionsInRecencyOrderReadsSavedAccess(t *testing.T) {
	tempHome := t.TempDir()
	isolateSQLX(t, tempHome)
	saved := &SavedSnapshot{
		Version:    Version,
		CapturedAt: 1_700_000_000,
		Sessions: []SessionSnapshot{
			{Name: "alpha", LastAccessed: 100},
			{Name: "zebra", LastAccessed: 300},
			{Name: "middle", LastAccessed: 200},
		},
	}
	if err := writeSavedSnapshot(tempHome, saved); err != nil {
		t.Fatalf("write saved snapshot: %v", err)
	}
	live := []tmux.Session{
		{Name: "alpha", Created: 1_800_000_000},
		{Name: "middle", Created: 1_800_000_000},
		{Name: "zebra", Created: 1_800_000_000},
	}

	got := sessionNamesForTest(SessionsInRecencyOrder(tempHome, live))
	want := []string{"zebra", "middle", "alpha"}
	if !sameNames(got, want) {
		t.Fatalf("order = %v, want %v", got, want)
	}
}

func TestSessionsInRecencyOrderKeepsLiveOrderWithoutSnapshot(t *testing.T) {
	tempHome := t.TempDir()
	isolateSQLX(t, tempHome)
	live := []tmux.Session{{Name: "alpha"}, {Name: "zebra"}}

	got := sessionNamesForTest(SessionsInRecencyOrder(tempHome, live))
	if !sameNames(got, []string{"alpha", "zebra"}) {
		t.Fatalf("order = %v, want live order", got)
	}
}

func sessionNamesForTest(sessions []tmux.Session) []string {
	names := make([]string, 0, len(sessions))
	for _, session := range sessions {
		names = append(names, session.Name)
	}
	return names
}

func sameNames(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}
