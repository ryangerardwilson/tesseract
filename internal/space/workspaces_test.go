package space

import (
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"
)

// settleQuickly makes Close's wait for an emptied workspace instant, giving
// up after a few reads, so a test never sleeps.
func settleQuickly(t *testing.T) {
	t.Helper()
	wait, giveUp := closeSettleWait, closeSettleGiveUp
	closeSettleWait, closeSettleGiveUp = 0, 20*time.Millisecond
	t.Cleanup(func() { closeSettleWait, closeSettleGiveUp = wait, giveUp })
}

// neverKills fails the test if any dispatch is a kill or a legacy
// dispatcher.
func neverKills(t *testing.T, expressions []string) {
	t.Helper()
	for _, expression := range expressions {
		for _, banned := range []string{"kill", "closewindow", "focuswindow", "movetoworkspacesilent", "keyword"} {
			if strings.Contains(expression, banned) {
				t.Fatalf("dispatched %q", expression)
			}
		}
	}
}

const closeClients = `[
{"address":"0x1a","mapped":true,"hidden":false,"workspace":{"id":1},"focusHistoryID":4},
{"address":"0x2a","mapped":true,"hidden":false,"workspace":{"id":2},"focusHistoryID":3},
{"address":"0x3a","mapped":true,"hidden":false,"workspace":{"id":3},"focusHistoryID":0},
{"address":"0x3b","mapped":true,"hidden":false,"workspace":{"id":3},"focusHistoryID":2},
{"address":"0x3c","mapped":true,"hidden":true,"workspace":{"id":3},"focusHistoryID":5},
{"address":"0x99","mapped":true,"hidden":false,"workspace":{"id":-98},"focusHistoryID":6}
]`

// closingDesktop serves workspaces 1 to 4 with active as the active one.
// After the first clients read, later reads come from after, so a test can
// say what the closes left behind.
type closingDesktop struct {
	active string
	after  string
	reads  int
	calls  [][]string
	reply  func(expression string) (string, error)
}

func (d *closingDesktop) run(args ...string) ([]byte, error) {
	d.calls = append(d.calls, append([]string(nil), args...))
	switch args[0] {
	case "workspaces":
		return []byte(`[{"id":1,"name":"1"},{"id":2,"name":"2"},{"id":-98,"name":"special:musica-hidden"},{"id":3,"name":"3"},{"id":4,"name":"4"}]`), nil
	case "activeworkspace":
		return []byte(`{"id":` + d.active + `}`), nil
	case "clients":
		d.reads++
		if d.reads > 1 {
			return []byte(d.after), nil
		}
		return []byte(closeClients), nil
	case "dispatch":
		if d.reply != nil {
			reply, err := d.reply(args[1])
			return []byte(reply), err
		}
		return []byte("ok\n"), nil
	}
	return []byte("unknown request\n"), errors.New("exit status 1")
}

func (d *closingDesktop) dispatches() []string {
	var expressions []string
	for _, call := range d.calls {
		if call[0] == "dispatch" {
			expressions = append(expressions, call[1])
		}
	}
	return expressions
}

func TestNewFocusesTheWorkspacePastTheHighest(t *testing.T) {
	fake := desktopFixture(t)

	id, err := New(fake.run)

	if err != nil {
		t.Fatal(err)
	}
	if id != 5 {
		t.Fatalf("id = %d, want 5 past workspace 4", id)
	}
	want := [][]string{
		{"workspaces", "-j"},
		{"dispatch", `hl.dsp.focus({ workspace = "5" })`},
	}
	if !reflect.DeepEqual(fake.calls, want) {
		t.Fatalf("calls = %q, want %q", fake.calls, want)
	}
}

func TestNewDoesNotReuseAGap(t *testing.T) {
	cases := []struct {
		name       string
		workspaces string
		want       int
	}{
		{"a gap below the highest", `[{"id":1,"name":"1"},{"id":7,"name":"7"}]`, 8},
		{"only special and named workspaces", `[{"id":-98,"name":"special:x"},{"id":-1337,"name":"name:y"}]`, 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fake := &fakeHyprctl{replies: map[string]string{"workspaces": tc.workspaces}}

			id, err := New(fake.run)

			if err != nil || id != tc.want {
				t.Fatalf("id = %d, err = %v, want %d", id, err, tc.want)
			}
			want := []string{focusWorkspace(tc.want)}
			if got := fake.dispatches(); !reflect.DeepEqual(got, want) {
				t.Fatalf("dispatches = %q, want %q", got, want)
			}
		})
	}
}

func TestNewWaitsUntilTheHostLayerIsGone(t *testing.T) {
	t.Setenv(waitLayerEnv, "1")
	previous := hostLayerWait
	hostLayerWait = 0
	t.Cleanup(func() { hostLayerWait = previous })
	var calls []string
	layersLeft := 2
	run := func(args ...string) ([]byte, error) {
		calls = append(calls, args[0])
		switch args[0] {
		case "workspaces":
			return []byte(`[{"id":1},{"id":2}]`), nil
		case "layers":
			if layersLeft > 0 {
				layersLeft--
				return []byte(`{"namespace":"omarchy-launcher"}`), nil
			}
			return []byte(`{}`), nil
		case "dispatch":
			return []byte("ok"), nil
		}
		return nil, errors.New("exit status 1")
	}

	if _, err := New(run); err != nil {
		t.Fatal(err)
	}
	want := []string{"workspaces", "layers", "layers", "layers", "dispatch"}
	if !reflect.DeepEqual(calls, want) {
		t.Fatalf("calls = %q, want %q", calls, want)
	}
}

func TestNewReportsHyprlandRefusal(t *testing.T) {
	fake := desktopFixture(t)
	fake.dispatch = func(string) (string, error) { return "hl.focus: no such monitor\n", nil }

	if _, err := New(fake.run); err == nil || !strings.Contains(err.Error(), "no such monitor") {
		t.Fatalf("err = %v, want the reply", err)
	}
}

func TestCloseABackgroundWorkspaceClosesItsAppsOnly(t *testing.T) {
	fake := desktopFixture(t)

	closed, err := Close(fake.run, 1)

	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		`hl.dsp.window.close({ window = "address:0x55d0000000f6" })`,
		`hl.dsp.window.close({ window = "address:0x55d0000000b2" })`,
	}
	if got := fake.dispatches(); !reflect.DeepEqual(got, want) {
		t.Fatalf("dispatches = %q, want %q", got, want)
	}
	if !reflect.DeepEqual(closed, Closed{Workspace: 1, Closed: []string{"0x55d0000000f6", "0x55d0000000b2"}}) {
		t.Fatalf("closed = %+v", closed)
	}
	reads := 0
	for _, call := range fake.calls {
		if call[0] == "clients" {
			reads++
		}
	}
	if reads != 1 {
		t.Fatalf("a background workspace was watched for emptying: %d clients reads", reads)
	}
}

func TestCloseTheActiveWorkspaceLandsOnTheNearest(t *testing.T) {
	settleQuickly(t)
	cases := []struct {
		name      string
		active    string
		workspace int
		closed    []string
		left      string
		focus     int
	}{
		{"the next lower id", "3", 3, []string{"0x3a", "0x3b"}, `[{"address":"0x1a","mapped":true,"workspace":{"id":1}},{"address":"0x3x","mapped":false,"workspace":{"id":3}}]`, 2},
		{"the next higher id from the lowest", "1", 1, []string{"0x1a"}, `[{"address":"0x2a","mapped":true,"workspace":{"id":2}}]`, 2},
		{"the next lower id from the highest", "4", 4, []string{}, `[]`, 3},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			desktop := &closingDesktop{active: tc.active, after: tc.left}

			closed, err := Close(desktop.run, tc.workspace)

			if err != nil {
				t.Fatal(err)
			}
			var want []string
			for _, address := range tc.closed {
				want = append(want, `hl.dsp.window.close({ window = "address:`+address+`" })`)
			}
			want = append(want, focusWorkspace(tc.focus))
			if got := desktop.dispatches(); !reflect.DeepEqual(got, want) {
				t.Fatalf("dispatches = %q, want %q", got, want)
			}
			if closed.Focus != tc.focus || !reflect.DeepEqual(closed.Closed, tc.closed) {
				t.Fatalf("closed = %+v", closed)
			}
		})
	}
}

func TestCloseLeavesAWorkspaceAWindowStaysOn(t *testing.T) {
	settleQuickly(t)
	cases := []struct {
		name string
		left string
	}{
		{"a save dialog", `[{"address":"0x3d","mapped":true,"hidden":false,"workspace":{"id":3}}]`},
		{"a hidden window that was never listed", `[{"address":"0x3c","mapped":true,"hidden":true,"workspace":{"id":3}}]`},
		{"a clients reply that is not JSON", `not json`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			desktop := &closingDesktop{active: "3", after: tc.left}

			closed, err := Close(desktop.run, 3)

			if err != nil {
				t.Fatal(err)
			}
			want := []string{
				`hl.dsp.window.close({ window = "address:0x3a" })`,
				`hl.dsp.window.close({ window = "address:0x3b" })`,
			}
			got := desktop.dispatches()
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("dispatches = %q, want one close per app and no focus", got)
			}
			neverKills(t, got)
			if closed.Focus != 0 {
				t.Fatalf("focus left a workspace a window stays on: %+v", closed)
			}
		})
	}
}

func TestCloseRefusesWithoutDispatch(t *testing.T) {
	cases := []struct {
		name      string
		run       Runner
		workspace int
	}{
		{"a workspace Hyprland does not report", desktopFixture(t).run, 7},
		{"a special workspace", desktopFixture(t).run, -98},
		{"workspace 0", desktopFixture(t).run, 0},
		{"the only workspace", (&fakeHyprctl{replies: map[string]string{
			"workspaces":      `[{"id":1,"name":"1"},{"id":-98,"name":"special:x"}]`,
			"clients":         `[{"address":"0x1a","mapped":true,"workspace":{"id":1}}]`,
			"activeworkspace": `{"id":1}`,
		}}).run, 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var dispatched []string
			run := func(args ...string) ([]byte, error) {
				if args[0] == "dispatch" {
					dispatched = append(dispatched, args[1])
				}
				return tc.run(args...)
			}

			if _, err := Close(run, tc.workspace); err == nil {
				t.Fatal("expected the close to be refused")
			}
			if len(dispatched) != 0 {
				t.Fatalf("refused close still dispatched %q", dispatched)
			}
		})
	}
}

func TestCloseWaitsUntilTheHostLayerIsGone(t *testing.T) {
	t.Setenv(waitLayerEnv, "1")
	previous := hostLayerWait
	hostLayerWait = 0
	t.Cleanup(func() { hostLayerWait = previous })
	desktop := &closingDesktop{active: "1"}
	layersLeft := 1
	run := func(args ...string) ([]byte, error) {
		if args[0] == "layers" {
			desktop.calls = append(desktop.calls, args)
			if layersLeft > 0 {
				layersLeft--
				return []byte(`{"namespace":"omarchy-launcher"}`), nil
			}
			return []byte(`{}`), nil
		}
		return desktop.run(args...)
	}

	if _, err := Close(run, 2); err != nil {
		t.Fatal(err)
	}
	var order []string
	for _, call := range desktop.calls {
		order = append(order, call[0])
	}
	want := []string{"workspaces", "clients", "activeworkspace", "layers", "layers", "dispatch"}
	if !reflect.DeepEqual(order, want) {
		t.Fatalf("calls = %q, want %q", order, want)
	}
}

func TestCloseSendsEveryCloseWhenOneFails(t *testing.T) {
	desktop := &closingDesktop{active: "1"}
	desktop.reply = func(expression string) (string, error) {
		if strings.Contains(expression, "0x3a") {
			return "hl.window.close: window not found\n", nil
		}
		return "ok\n", nil
	}

	closed, err := Close(desktop.run, 3)

	if err == nil || !strings.Contains(err.Error(), "window not found") {
		t.Fatalf("err = %v, want the failed close reported", err)
	}
	if got := desktop.dispatches(); len(got) != 2 {
		t.Fatalf("dispatches = %q, want both closes sent once", got)
	}
	if !reflect.DeepEqual(closed.Closed, []string{"0x3b"}) {
		t.Fatalf("closed = %+v", closed)
	}
}

func TestParseWorkspace(t *testing.T) {
	if id, err := ParseWorkspace("4"); err != nil || id != 4 {
		t.Fatalf("ParseWorkspace(4) = %d, %v", id, err)
	}
	for _, bad := range []string{"0", "-1", "special", "4 ", "0x4", ""} {
		if _, err := ParseWorkspace(bad); err == nil {
			t.Fatalf("ParseWorkspace(%q) succeeded", bad)
		}
	}
}
