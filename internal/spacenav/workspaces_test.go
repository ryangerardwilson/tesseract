package spacenav

import (
	"reflect"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// stepClock makes each press of d read a clock that has moved on by step.
func stepClock(t *testing.T, step time.Duration) {
	t.Helper()
	previous := now
	at := time.Date(2026, 11, 24, 0, 0, 0, 0, time.UTC)
	now = func() time.Time {
		at = at.Add(step)
		return at
	}
	t.Cleanup(func() { now = previous })
}

func TestNAddsAWorkspaceAndQuits(t *testing.T) {
	m, cmd := press(newModel(testSpace()), "n")

	if !quits(cmd) || !m.created {
		t.Fatalf("n: created = %v, quits = %v", m.created, quits(cmd))
	}
	if m.target != nil || m.placed != nil || m.closing != 0 {
		t.Fatalf("n chose more than a new workspace: %+v", m)
	}
}

func TestNAndDDWhileCarryingDoNothing(t *testing.T) {
	stepClock(t, 150*time.Millisecond)

	m, cmd := press(newModel(testSpace()), "s", "n", "d", "d")

	if quits(cmd) || m.created || m.closing != 0 {
		t.Fatalf("while carrying: created = %v, closing = %d", m.created, m.closing)
	}
	if m.carried == nil {
		t.Fatal("n or d dropped the carried window")
	}
}

func TestDDClosesTheSelectedWorkspace(t *testing.T) {
	stepClock(t, 150*time.Millisecond)

	m, cmd := press(newModel(testSpace()), "h", "d", "d")

	if !quits(cmd) || m.closing != 2 {
		t.Fatalf("dd on workspace 2: closing = %d, quits = %v", m.closing, quits(cmd))
	}
}

func TestDDNeedsBothPressesInsideTheWindow(t *testing.T) {
	cases := []struct {
		name string
		step time.Duration
		keys []string
	}{
		{"a d after the window has passed", 500 * time.Millisecond, []string{"d", "d"}},
		{"a held d repeating", 25 * time.Millisecond, []string{"d", "d", "d", "d", "d"}},
		{"a d before another key", 100 * time.Millisecond, []string{"d", "l", "d"}},
		{"a d before Escape", 100 * time.Millisecond, []string{"d", "esc"}},
		{"shift and d", 100 * time.Millisecond, []string{"D", "D"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			stepClock(t, tc.step)

			m, cmd := press(newModel(testSpace()), tc.keys...)

			if m.closing != 0 {
				t.Fatalf("%q closed workspace %d", tc.keys, m.closing)
			}
			if tc.keys[len(tc.keys)-1] != "esc" && quits(cmd) {
				t.Fatalf("%q quit the navigator", tc.keys)
			}
		})
	}
}

func TestADBeforeAnotherKeyLetsThatKeyWork(t *testing.T) {
	stepClock(t, 100*time.Millisecond)

	m, _ := press(newModel(testSpace()), "d", "l")

	if m.column != 3 || m.closing != 0 || !m.armed.IsZero() {
		t.Fatalf("after d then l: column %d closing %d armed %v, want workspace 4 selected and nothing armed", m.column, m.closing, m.armed)
	}
}

func TestCtrlDClosesNothing(t *testing.T) {
	stepClock(t, 100*time.Millisecond)
	m := newModel(testSpace())
	var cmd tea.Cmd
	for i := 0; i < 2; i++ {
		var next tea.Model
		next, cmd = m.Update(tea.KeyMsg{Type: tea.KeyCtrlD})
		m = next.(model)
	}

	if quits(cmd) || m.closing != 0 {
		t.Fatalf("ctrl+d twice: closing = %d", m.closing)
	}
}

func TestDDWithOneWorkspaceDoesNothing(t *testing.T) {
	stepClock(t, 150*time.Millisecond)
	one := testSpace()
	one.Workspaces = one.Workspaces[:1]
	one.Active = 1

	m, cmd := press(newModel(one), "d", "d")

	if quits(cmd) || m.closing != 0 {
		t.Fatalf("dd on the only workspace: closing = %d", m.closing)
	}
}

func TestCommitAddsOrCloses(t *testing.T) {
	added := &fakeHyprctl{}
	if err := (model{created: true}).commit(added.run); err != nil {
		t.Fatal(err)
	}
	wantAdded := [][]string{{"workspaces", "-j"}, {"dispatch", `hl.dsp.focus({ workspace = "4" })`}}
	if !reflect.DeepEqual(added.calls, wantAdded) {
		t.Fatalf("new: calls = %q, want %q", added.calls, wantAdded)
	}

	closed := &fakeHyprctl{}
	if err := (model{closing: 2}).commit(closed.run); err != nil {
		t.Fatal(err)
	}
	wantClosed := [][]string{
		{"workspaces", "-j"},
		{"clients", "-j"},
		{"activeworkspace", "-j"},
		{"dispatch", `hl.dsp.window.close({ window = "address:0x2a" })`},
		{"dispatch", `hl.dsp.window.close({ window = "address:0x2b" })`},
	}
	if !reflect.DeepEqual(closed.calls, wantClosed) {
		t.Fatalf("close: calls = %q, want %q", closed.calls, wantClosed)
	}
}

func TestNavigatorNAddsAWorkspaceAfterItExits(t *testing.T) {
	fake := &fakeHyprctl{}

	if err := navigateWith(t, fake, "n"); err != nil {
		t.Fatal(err)
	}

	want := [][]string{
		{"workspaces", "-j"},
		{"clients", "-j"},
		{"activeworkspace", "-j"},
		{"workspaces", "-j"},
		{"dispatch", `hl.dsp.focus({ workspace = "4" })`},
	}
	if !reflect.DeepEqual(fake.calls, want) {
		t.Fatalf("calls = %q, want %q", fake.calls, want)
	}
}

func TestNavigatorDDClosesAfterItExits(t *testing.T) {
	stepClock(t, 150*time.Millisecond)
	fake := &fakeHyprctl{}

	if err := navigateWith(t, fake, "hdd"); err != nil {
		t.Fatal(err)
	}

	var dispatched []string
	for _, call := range fake.calls {
		if call[0] == "dispatch" {
			dispatched = append(dispatched, call[1])
		}
	}
	want := []string{
		`hl.dsp.window.close({ window = "address:0x2a" })`,
		`hl.dsp.window.close({ window = "address:0x2b" })`,
	}
	if !reflect.DeepEqual(dispatched, want) {
		t.Fatalf("dispatches = %q, want %q", dispatched, want)
	}
	for _, expression := range dispatched {
		if strings.Contains(expression, "kill") {
			t.Fatalf("dispatched %q", expression)
		}
	}
}

func TestNavigatorALoneDThenQuitChangesNothing(t *testing.T) {
	stepClock(t, 150*time.Millisecond)
	fake := &fakeHyprctl{}

	if err := navigateWith(t, fake, "dq"); err != nil {
		t.Fatal(err)
	}

	want := [][]string{{"workspaces", "-j"}, {"clients", "-j"}, {"activeworkspace", "-j"}}
	if !reflect.DeepEqual(fake.calls, want) {
		t.Fatalf("a lone d must only have read: calls = %q", fake.calls)
	}
}
