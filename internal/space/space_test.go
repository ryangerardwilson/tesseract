package space

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

// fakeHyprctl serves fixture replies and records every hyprctl call. It never
// reaches a live Hyprland.
type fakeHyprctl struct {
	replies  map[string]string
	dispatch func(expression string) (string, error)
	calls    [][]string
}

func (f *fakeHyprctl) run(args ...string) ([]byte, error) {
	f.calls = append(f.calls, append([]string(nil), args...))
	if len(args) == 2 && args[0] == "dispatch" {
		if f.dispatch != nil {
			reply, err := f.dispatch(args[1])
			return []byte(reply), err
		}
		return []byte("ok\n"), nil
	}
	if reply, ok := f.replies[args[0]]; ok && len(args) == 2 && args[1] == "-j" {
		return []byte(reply), nil
	}
	return []byte("unknown request\n"), errors.New("exit status 1")
}

func (f *fakeHyprctl) dispatches() []string {
	var expressions []string
	for _, call := range f.calls {
		if call[0] == "dispatch" {
			expressions = append(expressions, call[1])
		}
	}
	return expressions
}

func desktopFixture(t *testing.T) *fakeHyprctl {
	t.Helper()
	replies := map[string]string{}
	for _, name := range []string{"workspaces", "clients", "activeworkspace"} {
		data, err := os.ReadFile(filepath.Join("testdata", name+".json"))
		if err != nil {
			t.Fatal(err)
		}
		replies[name] = string(data)
	}
	return &fakeHyprctl{replies: replies}
}

func desktopSpace() Space {
	return Space{
		Workspaces: []Workspace{
			{ID: 1, Name: "1", Layers: []Layer{
				{Address: "0x55d0000000f6", Class: "dev.zed.Zed", Title: "notes.md - Editor"},
				{Address: "0x55d0000000b2", Class: "omarchy-tmux", Title: "TMUX"},
			}},
			{ID: 2, Name: "2", Layers: []Layer{
				{Address: "0x55d0000000e5", Class: "chromium", Title: "Docs - Chromium"},
			}},
			{ID: 3, Name: "3", Layers: []Layer{
				{Address: "0x55d0000000d4", Class: "omarchy-terminal", Title: "Terminal"},
				{Address: "0x55d0000000c3", Class: "org.pwmt.zathura", Title: "/home/operator/Books/inductive_logic.pdf"},
			}},
			{ID: 4, Name: "4", Layers: []Layer{}},
		},
		Active: 3,
	}
}

func TestListReadsTheDesktopFixture(t *testing.T) {
	fake := desktopFixture(t)

	got, err := List(fake.run)

	if err != nil {
		t.Fatal(err)
	}
	if want := desktopSpace(); !reflect.DeepEqual(got, want) {
		t.Fatalf("space mismatch\n got: %+v\nwant: %+v", got, want)
	}
	wantCalls := [][]string{{"workspaces", "-j"}, {"clients", "-j"}, {"activeworkspace", "-j"}}
	if !reflect.DeepEqual(fake.calls, wantCalls) {
		t.Fatalf("listing must only read: got %q, want %q", fake.calls, wantCalls)
	}
}

func TestListJSONShape(t *testing.T) {
	data, err := json.Marshal(desktopSpace())
	if err != nil {
		t.Fatal(err)
	}
	want := `{"workspaces":[` +
		`{"id":1,"name":"1","layers":[{"address":"0x55d0000000f6","class":"dev.zed.Zed","title":"notes.md - Editor"},{"address":"0x55d0000000b2","class":"omarchy-tmux","title":"TMUX"}]},` +
		`{"id":2,"name":"2","layers":[{"address":"0x55d0000000e5","class":"chromium","title":"Docs - Chromium"}]},` +
		`{"id":3,"name":"3","layers":[{"address":"0x55d0000000d4","class":"omarchy-terminal","title":"Terminal"},{"address":"0x55d0000000c3","class":"org.pwmt.zathura","title":"/home/operator/Books/inductive_logic.pdf"}]},` +
		`{"id":4,"name":"4","layers":[]}],"active":3}`
	if string(data) != want {
		t.Fatalf("json mismatch\n got: %s\nwant: %s", data, want)
	}

	empty, err := Build([]byte(`[]`), []byte(`[]`), []byte(`{}`))
	if err != nil {
		t.Fatal(err)
	}
	if data, _ := json.Marshal(empty); string(data) != `{"workspaces":[]}` {
		t.Fatalf("empty desktop json = %s", data)
	}
}

// axisOf flattens a space to "id:address,address" per workspace.
func axisOf(current Space) []string {
	axis := make([]string, 0, len(current.Workspaces))
	for _, workspace := range current.Workspaces {
		addresses := make([]string, 0, len(workspace.Layers))
		for _, layer := range workspace.Layers {
			addresses = append(addresses, layer.Address)
		}
		axis = append(axis, strconv.Itoa(workspace.ID)+":"+strings.Join(addresses, ","))
	}
	return axis
}

func TestBuildAxes(t *testing.T) {
	cases := []struct {
		name       string
		workspaces string
		clients    string
		active     string
		want       []string
		wantActive int
	}{
		{
			name:       "workspaces sit left to right by id",
			workspaces: `[{"id":3,"name":"3"},{"id":-98,"name":"special:musica-hidden"},{"id":1,"name":"1"},{"id":2,"name":"2"}]`,
			clients:    `[]`,
			active:     `{"id":2}`,
			want:       []string{"1:", "2:", "3:"},
			wantActive: 2,
		},
		{
			name:       "an empty workspace keeps its place",
			workspaces: `[{"id":4,"name":"4"},{"id":1,"name":"1"}]`,
			clients:    `[{"address":"0x1","mapped":true,"workspace":{"id":1},"focusHistoryID":0}]`,
			active:     `{"id":4}`,
			want:       []string{"1:0x1", "4:"},
			wantActive: 4,
		},
		{
			name:       "a workspace hyprland does not report is not invented",
			workspaces: `[{"id":1,"name":"1"}]`,
			clients:    `[{"address":"0x7","mapped":true,"workspace":{"id":7},"focusHistoryID":0},{"address":"0x1","mapped":true,"workspace":{"id":1},"focusHistoryID":1}]`,
			active:     `{"id":7}`,
			want:       []string{"1:0x1"},
		},
		{
			name:       "the most recently focused window of a workspace is the front layer",
			workspaces: `[{"id":3,"name":"3"}]`,
			clients:    `[{"address":"0xpdf","mapped":true,"workspace":{"id":3},"focusHistoryID":2,"at":[0,26]},{"address":"0xa","mapped":true,"workspace":{"id":3},"focusHistoryID":0,"at":[640,26]}]`,
			active:     `{"id":3}`,
			want:       []string{"3:0xa,0xpdf"},
			wantActive: 3,
		},
		{
			name:       "a background workspace's front layer is relative",
			workspaces: `[{"id":1,"name":"1"},{"id":3,"name":"3"}]`,
			clients:    `[{"address":"0x5","mapped":true,"workspace":{"id":1},"focusHistoryID":5},{"address":"0x0","mapped":true,"workspace":{"id":3},"focusHistoryID":0},{"address":"0x3","mapped":true,"workspace":{"id":1},"focusHistoryID":3}]`,
			active:     `{"id":3}`,
			want:       []string{"1:0x3,0x5", "3:0x0"},
			wantActive: 3,
		},
		{
			name:       "hidden and unmapped windows are not layers",
			workspaces: `[{"id":1,"name":"1"}]`,
			clients:    `[{"address":"0xa","mapped":true,"hidden":true,"workspace":{"id":1},"focusHistoryID":0},{"address":"0xb","mapped":false,"workspace":{"id":1},"focusHistoryID":-1},{"address":"0xc","mapped":true,"hidden":false,"workspace":{"id":1},"focusHistoryID":2}]`,
			active:     `{"id":1}`,
			want:       []string{"1:0xc"},
			wantActive: 1,
		},
		{
			name:       "a pinned window stays on its own workspace",
			workspaces: `[{"id":1,"name":"1"},{"id":2,"name":"2"},{"id":3,"name":"3"}]`,
			clients:    `[{"address":"0xp","mapped":true,"pinned":true,"floating":true,"workspace":{"id":2},"focusHistoryID":0}]`,
			active:     `{"id":2}`,
			want:       []string{"1:", "2:0xp", "3:"},
			wantActive: 2,
		},
		{
			name:       "a window outside the focus history sits behind and ties keep order",
			workspaces: `[{"id":1,"name":"1"}]`,
			clients:    `[{"address":"0xn","mapped":true,"workspace":{"id":1},"focusHistoryID":-1},{"address":"0xt1","mapped":true,"workspace":{"id":1},"focusHistoryID":4},{"address":"0xt2","mapped":true,"workspace":{"id":1},"focusHistoryID":4},{"address":"0xf","mapped":true,"workspace":{"id":1},"focusHistoryID":1}]`,
			active:     `{"id":1}`,
			want:       []string{"1:0xf,0xt1,0xt2,0xn"},
			wantActive: 1,
		},
		{
			name:       "an active special workspace is not on the axis",
			workspaces: `[{"id":1,"name":"1"},{"id":-98,"name":"special:musica-hidden"}]`,
			clients:    `[]`,
			active:     `{"id":-98}`,
			want:       []string{"1:"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := Build([]byte(tc.workspaces), []byte(tc.clients), []byte(tc.active))
			if err != nil {
				t.Fatal(err)
			}
			if axis := axisOf(got); !reflect.DeepEqual(axis, tc.want) {
				t.Fatalf("axes = %q, want %q", axis, tc.want)
			}
			if got.Active != tc.wantActive {
				t.Fatalf("active = %d, want %d", got.Active, tc.wantActive)
			}
		})
	}
}

func TestBuildRejectsReplyThatIsNotJSON(t *testing.T) {
	_, err := Build([]byte("HYPRLAND_INSTANCE_SIGNATURE not set"), []byte(`[]`), []byte(`{}`))
	if err == nil || !strings.Contains(err.Error(), "HYPRLAND_INSTANCE_SIGNATURE not set") {
		t.Fatalf("expected the reply in the error, got %v", err)
	}
}

func TestFocusDispatchesWorkspaceThenWindow(t *testing.T) {
	fake := desktopFixture(t)

	err := Focus(fake.run, Target{Workspace: 3, Address: "0x55d0000000c3"})

	if err != nil {
		t.Fatal(err)
	}
	want := [][]string{
		{"clients", "-j"},
		{"dispatch", `hl.dsp.focus({ workspace = "3" })`},
		{"dispatch", `hl.dsp.focus({ window = "address:0x55d0000000c3" })`},
	}
	if !reflect.DeepEqual(fake.calls, want) {
		t.Fatalf("calls = %q, want %q", fake.calls, want)
	}
}

func TestFocusRefusesWithoutDispatch(t *testing.T) {
	cases := []struct {
		name   string
		target Target
	}{
		{"a layer of another workspace", Target{Workspace: 3, Address: "0x55d0000000e5"}},
		{"a window that no longer exists", Target{Workspace: 3, Address: "0x55d0000000ff"}},
		{"a hidden window", Target{Workspace: 2, Address: "0x55d000000107"}},
		{"an unmapped window", Target{Workspace: 1, Address: "0x55d000000118"}},
		{"a window on a special workspace", Target{Workspace: 1, Address: "0x55d0000000a1"}},
		{"a window on a workspace hyprland did not list", Target{Workspace: 1, Address: "0x55d00000013a"}},
		{"a workspace id below 1", Target{Workspace: 0, Address: "0x55d0000000d4"}},
		{"an address that leaves the lua string", Target{Workspace: 3, Address: `0x1" }) hl.exec_cmd("x`}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fake := desktopFixture(t)

			err := Focus(fake.run, tc.target)

			if err == nil {
				t.Fatal("expected focus to be refused")
			}
			if expressions := fake.dispatches(); len(expressions) != 0 {
				t.Fatalf("refused focus still dispatched %q", expressions)
			}
		})
	}
}

func TestFocusStopsWhenHyprlandRefuses(t *testing.T) {
	cases := []struct {
		name      string
		reply     func(expression string) (string, error)
		wantErr   string
		wantCalls int
	}{
		{
			name: "the workspace dispatch fails",
			reply: func(expression string) (string, error) {
				return "invalid workspace\n", nil
			},
			wantErr:   "invalid workspace",
			wantCalls: 1,
		},
		{
			name: "the window is not found",
			reply: func(expression string) (string, error) {
				if strings.Contains(expression, "window") {
					return "hl.focus: window not found\n", nil
				}
				return "ok\n", nil
			},
			wantErr:   "hl.focus: window not found",
			wantCalls: 2,
		},
		{
			name: "hyprctl exits non-zero",
			reply: func(expression string) (string, error) {
				return "", errors.New("exit status 1")
			},
			wantErr:   "exit status 1",
			wantCalls: 1,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fake := desktopFixture(t)
			fake.dispatch = tc.reply

			err := Focus(fake.run, Target{Workspace: 3, Address: "0x55d0000000d4"})

			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("err = %v, want it to mention %q", err, tc.wantErr)
			}
			if got := len(fake.dispatches()); got != tc.wantCalls {
				t.Fatalf("dispatches = %d, want %d", got, tc.wantCalls)
			}
		})
	}
}

func TestFocusAcceptsOkAndSilentReplies(t *testing.T) {
	for _, reply := range []string{"ok\n", "OK", ""} {
		fake := desktopFixture(t)
		fake.dispatch = func(string) (string, error) { return reply, nil }

		if err := Focus(fake.run, Target{Workspace: 3, Address: "0x55d0000000d4"}); err != nil {
			t.Fatalf("reply %q: %v", reply, err)
		}
	}
}

func TestMoveDispatchesTheWindowMove(t *testing.T) {
	fake := desktopFixture(t)

	err := Move(fake.run, Target{Workspace: 2, Address: "0x55d0000000c3"})

	if err != nil {
		t.Fatal(err)
	}
	want := [][]string{
		{"clients", "-j"},
		{"dispatch", `hl.dsp.window.move({ workspace = "2", window = "address:0x55d0000000c3" })`},
	}
	if !reflect.DeepEqual(fake.calls, want) {
		t.Fatalf("calls = %q, want %q", fake.calls, want)
	}
}

func TestMoveLeavesAWindowOnItsOwnWorkspace(t *testing.T) {
	fake := desktopFixture(t)

	if err := Move(fake.run, Target{Workspace: 3, Address: "0x55d0000000c3"}); err != nil {
		t.Fatal(err)
	}
	if expressions := fake.dispatches(); len(expressions) != 0 {
		t.Fatalf("a window already on that workspace was dispatched %q", expressions)
	}
}

func TestMoveRefusesWithoutDispatch(t *testing.T) {
	cases := []struct {
		name   string
		target Target
	}{
		{"a window that no longer exists", Target{Workspace: 2, Address: "0x55d0000000ff"}},
		{"a hidden window", Target{Workspace: 3, Address: "0x55d000000107"}},
		{"an unmapped window", Target{Workspace: 3, Address: "0x55d000000118"}},
		{"a window on a special workspace", Target{Workspace: 3, Address: "0x55d0000000a1"}},
		{"a workspace id below 1", Target{Workspace: 0, Address: "0x55d0000000d4"}},
		{"an address that leaves the lua string", Target{Workspace: 3, Address: `0x1" }) hl.exec_cmd("x`}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fake := desktopFixture(t)

			err := Move(fake.run, tc.target)

			if err == nil {
				t.Fatal("expected the move to be refused")
			}
			if expressions := fake.dispatches(); len(expressions) != 0 {
				t.Fatalf("refused move still dispatched %q", expressions)
			}
		})
	}
}

func TestMoveReportsHyprlandRefusal(t *testing.T) {
	fake := desktopFixture(t)
	fake.dispatch = func(string) (string, error) { return "hl.window.move: window not found\n", nil }

	err := Move(fake.run, Target{Workspace: 2, Address: "0x55d0000000c3"})

	if err == nil || !strings.Contains(err.Error(), "hl.window.move: window not found") {
		t.Fatalf("err = %v, want Hyprland's reply", err)
	}
}

func TestParseTarget(t *testing.T) {
	got, err := ParseTarget("3", "0x55D0000000C3")
	if err != nil {
		t.Fatal(err)
	}
	if want := (Target{Workspace: 3, Address: "0x55d0000000c3"}); got != want {
		t.Fatalf("target = %+v, want %+v", got, want)
	}

	bad := [][2]string{
		{"0", "0x1"},
		{"-1", "0x1"},
		{"x", "0x1"},
		{"3.0", "0x1"},
		{"", "0x1"},
		{"special:musica-hidden", "0x1"},
		{"3", "special"},
		{"3", "0x"},
		{"3", "address:0x3a"},
		{"3", "0x3g"},
		{"3", `0x3a" })`},
		{"3", "0x11111111111111111"},
		{"3", ""},
	}
	for _, args := range bad {
		if _, err := ParseTarget(args[0], args[1]); err == nil {
			t.Fatalf("ParseTarget(%q, %q) accepted a malformed target", args[0], args[1])
		}
	}
}
