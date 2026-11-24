package spacenav

import (
	"errors"
	"io"
	"reflect"
	"strings"
	"testing"
	"testing/iotest"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/ryangerardwilson/tesseract/internal/space"
)

func testSpace() space.Space {
	return space.Space{
		Workspaces: []space.Workspace{
			{ID: 1, Name: "1", Layers: []space.Layer{
				{Address: "0x1a", Class: "omarchy-tmux", Title: "TMUX"},
			}},
			{ID: 2, Name: "2", Layers: []space.Layer{
				{Address: "0x2a", Class: "chromium", Title: "Docs - Chromium"},
				{Address: "0x2b", Class: "org.gnome.Nautilus", Title: "Downloads"},
			}},
			{ID: 3, Name: "3", Layers: []space.Layer{
				{Address: "0x3a", Class: "omarchy-terminal", Title: "Terminal"},
				{Address: "0x3b", Class: "org.pwmt.zathura", Title: "book.pdf"},
			}},
			{ID: 4, Name: "4", Layers: []space.Layer{}},
		},
		Active: 3,
	}
}

func key(name string) tea.KeyMsg {
	switch name {
	case "left":
		return tea.KeyMsg{Type: tea.KeyLeft}
	case "right":
		return tea.KeyMsg{Type: tea.KeyRight}
	case "up":
		return tea.KeyMsg{Type: tea.KeyUp}
	case "down":
		return tea.KeyMsg{Type: tea.KeyDown}
	case "enter":
		return tea.KeyMsg{Type: tea.KeyEnter}
	case "esc":
		return tea.KeyMsg{Type: tea.KeyEsc}
	case "ctrl+c":
		return tea.KeyMsg{Type: tea.KeyCtrlC}
	case "ctrl+s":
		return tea.KeyMsg{Type: tea.KeyCtrlS}
	}
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(name)}
}

func press(m model, names ...string) (model, tea.Cmd) {
	var cmd tea.Cmd
	for _, name := range names {
		var next tea.Model
		next, cmd = m.Update(key(name))
		m = next.(model)
	}
	return m, cmd
}

func quits(cmd tea.Cmd) bool {
	if cmd == nil {
		return false
	}
	_, ok := cmd().(tea.QuitMsg)
	return ok
}

func TestStartsOnTheActiveFrontLayer(t *testing.T) {
	m := newModel(testSpace())
	if m.column != 2 || m.layer != 0 {
		t.Fatalf("start = column %d layer %d, want workspace 3's front layer", m.column, m.layer)
	}

	current := testSpace()
	current.Active = 0
	if m := newModel(current); m.column != 0 || m.layer != 0 {
		t.Fatalf("without an active workspace start = column %d layer %d, want the first workspace", m.column, m.layer)
	}
}

func TestKeysMoveOnBothAxesWithoutFocusing(t *testing.T) {
	cases := []struct {
		keys          []string
		wantWorkspace int
		wantLayer     int
	}{
		{[]string{"h"}, 2, 0},
		{[]string{"left"}, 2, 0},
		{[]string{"h", "j"}, 2, 1},
		{[]string{"h", "down"}, 2, 1},
		{[]string{"h", "j", "j"}, 2, 0},
		{[]string{"h", "j", "k"}, 2, 0},
		{[]string{"h", "j", "up"}, 2, 0},
		{[]string{"k"}, 3, 1},
		{[]string{"h", "j", "l"}, 3, 0},
		{[]string{"l"}, 4, 0},
		{[]string{"right", "right"}, 1, 0},
		{[]string{"h", "h", "h", "h"}, 3, 0},
		{[]string{"j"}, 3, 1},
	}
	for _, tc := range cases {
		m, cmd := press(newModel(testSpace()), tc.keys...)
		workspace := m.space.Workspaces[m.column].ID
		if workspace != tc.wantWorkspace || m.layer != tc.wantLayer {
			t.Fatalf("%q: workspace %d layer %d, want workspace %d layer %d", tc.keys, workspace, m.layer, tc.wantWorkspace, tc.wantLayer)
		}
		if quits(cmd) || m.target != nil {
			t.Fatalf("%q: moving must not quit or choose a focus target", tc.keys)
		}
	}
}

func TestMovesCyclePastEitherEnd(t *testing.T) {
	cycling := space.Space{
		Workspaces: []space.Workspace{
			{ID: 1, Name: "1", Layers: []space.Layer{{Address: "0x1a"}}},
			{ID: 2, Name: "2", Layers: []space.Layer{{Address: "0x2a"}, {Address: "0x2b"}, {Address: "0x2c"}}},
			{ID: 3, Name: "3", Layers: []space.Layer{{Address: "0x3a"}}},
		},
	}
	cases := []struct {
		name          string
		start         int
		keys          []string
		wantWorkspace int
		wantLayer     int
	}{
		{"h on the first workspace selects the last", 1, []string{"h"}, 3, 0},
		{"left on the first workspace selects the last", 1, []string{"left"}, 3, 0},
		{"l on the last workspace selects the first", 3, []string{"l"}, 1, 0},
		{"right on the last workspace selects the first", 3, []string{"right"}, 1, 0},
		{"k on the front layer selects the deepest", 2, []string{"k"}, 2, 2},
		{"up on the front layer selects the deepest", 2, []string{"up"}, 2, 2},
		{"j on the deepest layer selects the front", 2, []string{"j", "j", "j"}, 2, 0},
		{"down on the deepest layer selects the front", 2, []string{"down", "down", "down"}, 2, 0},
		{"a wrap on X lands on the front layer", 2, []string{"j", "j", "l", "l"}, 1, 0},
		{"one layer stays put", 1, []string{"j", "k"}, 1, 0},
	}
	for _, tc := range cases {
		current := cycling
		current.Active = tc.start
		m, cmd := press(newModel(current), tc.keys...)
		workspace := m.space.Workspaces[m.column].ID
		if workspace != tc.wantWorkspace || m.layer != tc.wantLayer {
			t.Fatalf("%s: workspace %d layer %d, want workspace %d layer %d", tc.name, workspace, m.layer, tc.wantWorkspace, tc.wantLayer)
		}
		if quits(cmd) || m.target != nil {
			t.Fatalf("%s: cycling must not quit or choose a focus target", tc.name)
		}
	}

	alone := space.Space{Workspaces: []space.Workspace{{ID: 1, Name: "1"}}, Active: 1}
	if m, _ := press(newModel(alone), "h", "l", "j", "k"); m.column != 0 || m.layer != 0 {
		t.Fatalf("one empty workspace = column %d layer %d, want it to stay put", m.column, m.layer)
	}
}

func TestEnterChoosesTheSelectedLayer(t *testing.T) {
	m, cmd := press(newModel(testSpace()), "h", "j", "enter")

	if !quits(cmd) {
		t.Fatal("enter must close the navigator")
	}
	if m.target == nil || *m.target != (space.Target{Workspace: 2, Address: "0x2b"}) {
		t.Fatalf("target = %+v, want workspace 2 layer 0x2b", m.target)
	}
}

func TestCtrlMChoosesTheSelectedLayer(t *testing.T) {
	m, cmd := press(newModel(testSpace()), "ctrl+m")

	if !quits(cmd) {
		t.Fatal("ctrl+m must close the navigator")
	}
	if m.carried != nil || m.target == nil || *m.target != (space.Target{Workspace: 3, Address: "0x3a"}) {
		t.Fatalf("target = %+v carried = %+v, want workspace 3 layer 0x3a and no carry", m.target, m.carried)
	}
}

// Ctrl+M is Enter: while a window is carried it puts it down neither where it
// was picked up nor in a new place.
func TestCtrlMWhileCarryingDoesNothing(t *testing.T) {
	for _, keys := range [][]string{{"s"}, {"s", "h"}} {
		carrying, _ := press(newModel(testSpace()), keys...)
		m, cmd := press(carrying, "ctrl+m")
		if quits(cmd) || !reflect.DeepEqual(m, carrying) {
			t.Fatalf("ctrl+m after %q must leave the carry as it was: %+v", keys, m)
		}
	}
}

func TestEnterOnAnEmptyWorkspaceDoesNothing(t *testing.T) {
	for _, name := range []string{"enter", "ctrl+m"} {
		m, cmd := press(newModel(testSpace()), "l", name)
		if quits(cmd) || m.target != nil || m.carried != nil {
			t.Fatalf("%s on a workspace with no apps must do nothing", name)
		}
	}
}

func TestQuitKeysChooseNothing(t *testing.T) {
	for _, name := range []string{"q", "esc", "ctrl+c"} {
		m, cmd := press(newModel(testSpace()), "h", "j", name)
		if !quits(cmd) {
			t.Fatalf("%s must close the navigator", name)
		}
		if m.target != nil {
			t.Fatalf("%s must not choose a focus target", name)
		}
	}
}

func addresses(layers []space.Layer) []string {
	out := []string{}
	for _, layer := range layers {
		out = append(out, layer.Address)
	}
	return out
}

func TestFirstSPicksUpWithoutMovingAnything(t *testing.T) {
	m, cmd := press(newModel(testSpace()), "s")

	if m.carried == nil || m.carried.window.Address != "0x3a" {
		t.Fatalf("carried = %+v, want workspace 3's front layer 0x3a", m.carried)
	}
	if quits(cmd) || m.target != nil || m.placed != nil {
		t.Fatal("the first s must not quit, focus, or move")
	}
	if !reflect.DeepEqual(m.shown(), testSpace().Workspaces) {
		t.Fatalf("picking up must leave every layer in place: %+v", m.shown())
	}
	if view := m.View(); !strings.Contains(view, "SHIFT") {
		t.Fatalf("the footer must say SHIFT while a window is carried:\n%s", view)
	}
	if view := newModel(testSpace()).View(); strings.Contains(view, "SHIFT") {
		t.Fatalf("the footer must not say SHIFT before s:\n%s", view)
	}
}

func TestHCarriesFromTheFirstWorkspaceToTheLast(t *testing.T) {
	current := testSpace()
	current.Active = 1

	m, cmd := press(newModel(current), "s", "h")

	if quits(cmd) || m.placed != nil || m.carried == nil {
		t.Fatal("carrying must not quit or move")
	}
	if workspace := m.space.Workspaces[m.column].ID; workspace != 4 || m.layer != 0 {
		t.Fatalf("carried to workspace %d layer %d, want the last workspace's front layer", workspace, m.layer)
	}
	shown := m.shown()
	if got := addresses(shown[3].Layers); !reflect.DeepEqual(got, []string{"0x1a"}) {
		t.Fatalf("last workspace = %q, want the carried window", got)
	}
	if got := addresses(shown[0].Layers); len(got) != 0 {
		t.Fatalf("first workspace = %q, want the carried window off it", got)
	}
	if !reflect.DeepEqual(m.space, current) {
		t.Fatal("carrying must not change the listed space")
	}

	m, _ = press(m, "h")
	if got := addresses(m.shown()[2].Layers); !reflect.DeepEqual(got, []string{"0x1a", "0x3a", "0x3b"}) {
		t.Fatalf("workspace 3 = %q, want the carried window in front of its layers", got)
	}
}

func TestCarryOnZWrapsAndKeepsTheOtherLayersInOrder(t *testing.T) {
	stack := space.Space{
		Workspaces: []space.Workspace{
			{ID: 1, Name: "1", Layers: []space.Layer{{Address: "0xa"}, {Address: "0xb"}, {Address: "0xc"}}},
			{ID: 2, Name: "2", Layers: []space.Layer{{Address: "0xd"}}},
		},
		Active: 1,
	}
	m, _ := press(newModel(stack), "s")
	for _, step := range []struct {
		key  string
		want []string
	}{
		{"j", []string{"0xb", "0xa", "0xc"}},
		{"down", []string{"0xb", "0xc", "0xa"}},
		{"j", []string{"0xa", "0xb", "0xc"}},
		{"k", []string{"0xb", "0xc", "0xa"}},
		{"up", []string{"0xb", "0xa", "0xc"}},
	} {
		var cmd tea.Cmd
		m, cmd = press(m, step.key)
		if got := addresses(m.layers()); !reflect.DeepEqual(got, step.want) || got[m.layer] != "0xa" {
			t.Fatalf("after %q: layers %q with the carried window at %d, want %q", step.key, got, m.layer, step.want)
		}
		if quits(cmd) || m.placed != nil {
			t.Fatalf("after %q: carrying must not quit or move", step.key)
		}
	}
}

func TestSecondSPutsTheWindowDown(t *testing.T) {
	current := testSpace()
	current.Active = 1

	m, cmd := press(newModel(current), "s", "h", "s")

	if !quits(cmd) {
		t.Fatal("putting a window in a new place must close the navigator")
	}
	if m.placed == nil || *m.placed != (placement{target: space.Target{Workspace: 4, Address: "0x1a"}}) {
		t.Fatalf("placed = %+v, want 0x1a on workspace 4's front layer", m.placed)
	}

	m, _ = press(newModel(testSpace()), "s", "h", "j", "s")
	if m.placed == nil || *m.placed != (placement{target: space.Target{Workspace: 2, Address: "0x3a"}, raise: true}) {
		t.Fatalf("placed = %+v, want 0x3a behind workspace 2's front layer, raised after the move", m.placed)
	}
}

func TestPuttingDownInPlaceMovesNothing(t *testing.T) {
	m, cmd := press(newModel(testSpace()), "s", "j", "k", "s")

	if quits(cmd) || m.placed != nil || m.carried != nil {
		t.Fatal("a window put down where it was picked up must not move or close the navigator")
	}
}

func TestEscapeCancelsTheCarry(t *testing.T) {
	m, cmd := press(newModel(testSpace()), "j", "s", "h", "j", "esc")

	if quits(cmd) || m.carried != nil || m.placed != nil || m.target != nil {
		t.Fatal("escape while carrying must cancel without closing, moving, or focusing")
	}
	if workspace := m.space.Workspaces[m.column].ID; workspace != 3 || m.layer != 1 {
		t.Fatalf("after cancel = workspace %d layer %d, want workspace 3's second layer", workspace, m.layer)
	}
	if !reflect.DeepEqual(m.shown(), testSpace().Workspaces) {
		t.Fatalf("after cancel every layer must be back in place: %+v", m.shown())
	}
	if view := m.View(); strings.Contains(view, "SHIFT") || !strings.Contains(view, "> org.pwmt.z") {
		t.Fatalf("after cancel the view must mark the old place without SHIFT:\n%s", view)
	}

	m, cmd = press(m, "esc")
	if !quits(cmd) || m.target != nil || m.placed != nil {
		t.Fatal("escape with nothing carried must close the navigator without focusing")
	}
}

func TestSWithoutAWindowDoesNothing(t *testing.T) {
	m, cmd := press(newModel(testSpace()), "l", "s")

	if quits(cmd) || m.carried != nil {
		t.Fatal("s on a workspace with no apps must do nothing")
	}
}

// Bare m and Ctrl+S are not navigator keys. They neither pick up nor put
// down a window, so a host may bind bare m.
func TestBareMAndCtrlSLeaveTheNavigatorAsItWas(t *testing.T) {
	rest := newModel(testSpace())
	carrying, _ := press(rest, "s", "h")
	for _, name := range []string{"m", "ctrl+s"} {
		for _, before := range []model{rest, carrying} {
			after, cmd := press(before, name)
			if quits(cmd) || !reflect.DeepEqual(after, before) {
				t.Fatalf("%s changed the navigator: %+v, want %+v", name, after, before)
			}
		}
	}
}

func TestEnterWhileCarryingDoesNothing(t *testing.T) {
	m, cmd := press(newModel(testSpace()), "s", "h", "enter")

	if quits(cmd) || m.target != nil || m.placed != nil || m.carried == nil {
		t.Fatal("enter while carrying must do nothing")
	}
}

func TestViewShowsTheRowAndTheStack(t *testing.T) {
	sized, _ := newModel(testSpace()).Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	view := sized.View()

	for _, want := range []string{"tesseract space", " 1 ", " 2 ", " 3*", " 4 ", "> omarchy-terminal", "Terminal", "org.pwmt.zathura", "book.pdf"} {
		if !strings.Contains(view, want) {
			t.Fatalf("view missing %q:\n%s", want, view)
		}
	}
	if strings.Contains(view, "chromium") {
		t.Fatalf("view must show only the selected workspace's layers:\n%s", view)
	}
}

// The terminal navigator lists one workspace's layers as a stack, front layer
// first, and moving through them only moves the marker: the layers keep their
// order.
func TestViewListsTheStackFrontFirst(t *testing.T) {
	m, _ := press(newModel(testSpace()), "h")
	for _, step := range []struct {
		key  string
		mark string
	}{
		{"", "> chromium"},
		{"j", "> org.gnome.Nautilus"},
		{"j", "> chromium"},
		{"k", "> org.gnome.Nautilus"},
	} {
		if step.key != "" {
			m, _ = press(m, step.key)
		}
		sized, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
		view := sized.View()
		front := strings.Index(view, "chromium")
		deeper := strings.Index(view, "org.gnome.Nautilus")
		if front < 0 || deeper < 0 || front > deeper {
			t.Fatalf("after %q workspace 2's front layer must be listed above the layer under it:\n%s", step.key, view)
		}
		if !strings.Contains(view, step.mark) {
			t.Fatalf("after %q the view must mark %q:\n%s", step.key, step.mark, view)
		}
	}
}

// The terminal navigator calls a window an app, and its footer names s
// while a window is carried.
func TestViewSaysAppAndShift(t *testing.T) {
	carrying, _ := press(newModel(testSpace()), "s")
	empty, _ := press(newModel(testSpace()), "l")
	wide := newModel(testSpace())
	wide.width = 100
	for _, tc := range []struct {
		view string
		want string
	}{
		{wide.View(), "enter focus  h/l workspace  k/j app  n new  dd close  q quit"},
		{newModel(testSpace()).View(), "enter focus  h/l workspace  k/j app\nn new  dd close  q quit"},
		{carrying.View(), "SHIFT  h/l k/j carry  s put down  q cancel"},
		{empty.View(), "No apps"},
	} {
		if !strings.Contains(tc.view, tc.want) {
			t.Fatalf("view missing %q:\n%s", tc.want, tc.view)
		}
		if strings.Contains(strings.ToLower(tc.view), "layer") || strings.Contains(tc.view, "MOVE") {
			t.Fatalf("the view must say app and SHIFT, not layer or MOVE:\n%s", tc.view)
		}
	}
}

func TestViewFitsBeforeTheTerminalSizeIsKnown(t *testing.T) {
	view := newModel(testSpace()).View()

	for _, line := range strings.Split(view, "\n") {
		if width := len([]rune(line)); width > 48 {
			t.Fatalf("line is %d cells wide before any size message: %q", width, line)
		}
	}
	if !strings.Contains(view, "> omarchy-te...") || !strings.Contains(view, "Terminal") {
		t.Fatalf("narrow view lost the class or title:\n%s", view)
	}
}

func TestViewBlanksControlCharactersInTitles(t *testing.T) {
	current := testSpace()
	current.Workspaces[2].Layers[0].Title = "evil\x1b]0;owned\x07\nnext"

	view := newModel(current).View()

	if strings.Contains(view, "\x1b]0;") || strings.Contains(view, "\x07") {
		t.Fatalf("a window title reached the terminal raw: %q", view)
	}
}

// fakeHyprctl serves one desktop and records every hyprctl call.
type fakeHyprctl struct {
	calls [][]string
}

func (f *fakeHyprctl) run(args ...string) ([]byte, error) {
	f.calls = append(f.calls, append([]string(nil), args...))
	replies := map[string]string{
		"workspaces":      `[{"id":3,"name":"3"},{"id":1,"name":"1"},{"id":2,"name":"2"}]`,
		"clients":         `[{"address":"0x1a","mapped":true,"workspace":{"id":1},"class":"omarchy-tmux","title":"TMUX","focusHistoryID":3},{"address":"0x2b","mapped":true,"workspace":{"id":2},"class":"org.gnome.Nautilus","title":"Downloads","focusHistoryID":4},{"address":"0x2a","mapped":true,"workspace":{"id":2},"class":"chromium","title":"Docs - Chromium","focusHistoryID":1},{"address":"0x3a","mapped":true,"workspace":{"id":3},"class":"omarchy-terminal","title":"Terminal","focusHistoryID":0}]`,
		"activeworkspace": `{"id":3,"name":"3"}`,
	}
	if args[0] == "dispatch" {
		return []byte("ok\n"), nil
	}
	if reply, ok := replies[args[0]]; ok {
		return []byte(reply), nil
	}
	return nil, errors.New("exit status 1")
}

func navigateWith(t *testing.T, fake *fakeHyprctl, input string) error {
	t.Helper()
	return navigate(fake.run,
		tea.WithInput(iotest.OneByteReader(strings.NewReader(input))),
		tea.WithOutput(io.Discard),
		tea.WithoutSignalHandler(),
	)
}

func TestNavigatorFocusesOnlyAfterEnter(t *testing.T) {
	fake := &fakeHyprctl{}

	if err := navigateWith(t, fake, "hj\r"); err != nil {
		t.Fatal(err)
	}

	want := [][]string{
		{"workspaces", "-j"},
		{"clients", "-j"},
		{"activeworkspace", "-j"},
		{"clients", "-j"},
		{"dispatch", `hl.dsp.focus({ workspace = "2" })`},
		{"dispatch", `hl.dsp.focus({ window = "address:0x2b" })`},
	}
	if !reflect.DeepEqual(fake.calls, want) {
		t.Fatalf("calls = %q, want %q", fake.calls, want)
	}
}

func TestNavigatorFirstSAndCancelDispatchNothing(t *testing.T) {
	fake := &fakeHyprctl{}

	if err := navigateWith(t, fake, "shjqq"); err != nil {
		t.Fatal(err)
	}

	want := [][]string{{"workspaces", "-j"}, {"clients", "-j"}, {"activeworkspace", "-j"}}
	if !reflect.DeepEqual(fake.calls, want) {
		t.Fatalf("picking up and cancelling must only have read: calls = %q", fake.calls)
	}
}

// From workspace 1, h carries the window round to workspace 3, the last, and
// the second s moves it there with one dispatch.
func TestNavigatorSecondSMovesTheWindow(t *testing.T) {
	fake := &fakeHyprctl{}

	if err := navigateWith(t, fake, "lshs"); err != nil {
		t.Fatal(err)
	}

	want := [][]string{
		{"workspaces", "-j"},
		{"clients", "-j"},
		{"activeworkspace", "-j"},
		{"clients", "-j"},
		{"dispatch", `hl.dsp.window.move({ workspace = "3", window = "address:0x1a" })`},
	}
	if !reflect.DeepEqual(fake.calls, want) {
		t.Fatalf("calls = %q, want %q", fake.calls, want)
	}
}

// Brought to the front of its own workspace, the window does not change
// workspace, so the move dispatches nothing and focus lands it on top.
func TestNavigatorPutDownOnItsOwnWorkspaceFocuses(t *testing.T) {
	fake := &fakeHyprctl{}

	if err := navigateWith(t, fake, "hjsks"); err != nil {
		t.Fatal(err)
	}

	want := [][]string{
		{"workspaces", "-j"},
		{"clients", "-j"},
		{"activeworkspace", "-j"},
		{"clients", "-j"},
		{"clients", "-j"},
		{"dispatch", `hl.dsp.focus({ workspace = "2" })`},
		{"dispatch", `hl.dsp.focus({ window = "address:0x2b" })`},
	}
	if !reflect.DeepEqual(fake.calls, want) {
		t.Fatalf("calls = %q, want %q", fake.calls, want)
	}
}

// Bare m picks nothing up and puts nothing down, and Enter, which is how
// Ctrl+M arrives, does nothing while a window is carried: only the second s
// moves the window. The trailing q keys close the navigator if either one
// ever shifts.
func TestNavigatorBareMAndEnterNeverShift(t *testing.T) {
	fake := &fakeHyprctl{}

	if err := navigateWith(t, fake, "lmshm\rsqq"); err != nil {
		t.Fatal(err)
	}

	want := [][]string{
		{"workspaces", "-j"},
		{"clients", "-j"},
		{"activeworkspace", "-j"},
		{"clients", "-j"},
		{"dispatch", `hl.dsp.window.move({ workspace = "3", window = "address:0x1a" })`},
	}
	if !reflect.DeepEqual(fake.calls, want) {
		t.Fatalf("calls = %q, want %q", fake.calls, want)
	}
}

func TestNavigatorQuitLeavesTheDesktop(t *testing.T) {
	fake := &fakeHyprctl{}

	if err := navigateWith(t, fake, "hjq"); err != nil {
		t.Fatal(err)
	}

	want := [][]string{{"workspaces", "-j"}, {"clients", "-j"}, {"activeworkspace", "-j"}}
	if !reflect.DeepEqual(fake.calls, want) {
		t.Fatalf("quitting must only have read: calls = %q", fake.calls)
	}
}
