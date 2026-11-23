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

func TestEnterOnAnEmptyWorkspaceDoesNothing(t *testing.T) {
	m, cmd := press(newModel(testSpace()), "l", "enter")

	if quits(cmd) || m.target != nil {
		t.Fatal("enter on a workspace with no layers must do nothing")
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

func TestFirstMPicksUpWithoutMovingAnything(t *testing.T) {
	m, cmd := press(newModel(testSpace()), "m")

	if m.carried == nil || m.carried.window.Address != "0x3a" {
		t.Fatalf("carried = %+v, want workspace 3's front layer 0x3a", m.carried)
	}
	if quits(cmd) || m.target != nil || m.placed != nil {
		t.Fatal("the first m must not quit, focus, or move")
	}
	if !reflect.DeepEqual(m.shown(), testSpace().Workspaces) {
		t.Fatalf("picking up must leave every layer in place: %+v", m.shown())
	}
	if view := m.View(); !strings.Contains(view, "MOVE") {
		t.Fatalf("the footer must say MOVE while a window is carried:\n%s", view)
	}
	if view := newModel(testSpace()).View(); strings.Contains(view, "MOVE") {
		t.Fatalf("the footer must not say MOVE before m:\n%s", view)
	}
}

func TestHCarriesFromTheFirstWorkspaceToTheLast(t *testing.T) {
	current := testSpace()
	current.Active = 1

	m, cmd := press(newModel(current), "m", "h")

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
	m, _ := press(newModel(stack), "m")
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

func TestSecondMPutsTheWindowDown(t *testing.T) {
	current := testSpace()
	current.Active = 1

	m, cmd := press(newModel(current), "m", "h", "m")

	if !quits(cmd) {
		t.Fatal("putting a window in a new place must close the navigator")
	}
	if m.placed == nil || *m.placed != (placement{target: space.Target{Workspace: 4, Address: "0x1a"}}) {
		t.Fatalf("placed = %+v, want 0x1a on workspace 4's front layer", m.placed)
	}

	m, _ = press(newModel(testSpace()), "m", "h", "j", "m")
	if m.placed == nil || *m.placed != (placement{target: space.Target{Workspace: 2, Address: "0x3a"}, raise: true}) {
		t.Fatalf("placed = %+v, want 0x3a behind workspace 2's front layer, raised after the move", m.placed)
	}
}

func TestPuttingDownInPlaceMovesNothing(t *testing.T) {
	m, cmd := press(newModel(testSpace()), "m", "j", "k", "m")

	if quits(cmd) || m.placed != nil || m.carried != nil {
		t.Fatal("a window put down where it was picked up must not move or close the navigator")
	}
}

func TestEscapeCancelsTheCarry(t *testing.T) {
	m, cmd := press(newModel(testSpace()), "j", "m", "h", "j", "esc")

	if quits(cmd) || m.carried != nil || m.placed != nil || m.target != nil {
		t.Fatal("escape while carrying must cancel without closing, moving, or focusing")
	}
	if workspace := m.space.Workspaces[m.column].ID; workspace != 3 || m.layer != 1 {
		t.Fatalf("after cancel = workspace %d layer %d, want workspace 3's second layer", workspace, m.layer)
	}
	if !reflect.DeepEqual(m.shown(), testSpace().Workspaces) {
		t.Fatalf("after cancel every layer must be back in place: %+v", m.shown())
	}
	if view := m.View(); strings.Contains(view, "MOVE") || !strings.Contains(view, "> org.pwmt.z") {
		t.Fatalf("after cancel the view must mark the old place without MOVE:\n%s", view)
	}

	m, cmd = press(m, "esc")
	if !quits(cmd) || m.target != nil || m.placed != nil {
		t.Fatal("escape with nothing carried must close the navigator without focusing")
	}
}

func TestMWithoutAWindowDoesNothing(t *testing.T) {
	m, cmd := press(newModel(testSpace()), "l", "m")

	if quits(cmd) || m.carried != nil {
		t.Fatal("m on a workspace with no layers must do nothing")
	}
}

func TestEnterWhileCarryingDoesNothing(t *testing.T) {
	m, cmd := press(newModel(testSpace()), "m", "h", "enter")

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

func TestNavigatorFirstMAndCancelDispatchNothing(t *testing.T) {
	fake := &fakeHyprctl{}

	if err := navigateWith(t, fake, "mhjqq"); err != nil {
		t.Fatal(err)
	}

	want := [][]string{{"workspaces", "-j"}, {"clients", "-j"}, {"activeworkspace", "-j"}}
	if !reflect.DeepEqual(fake.calls, want) {
		t.Fatalf("picking up and cancelling must only have read: calls = %q", fake.calls)
	}
}

// From workspace 1, h carries the window round to workspace 3, the last, and
// the second m moves it there with one dispatch.
func TestNavigatorSecondMMovesTheWindow(t *testing.T) {
	fake := &fakeHyprctl{}

	if err := navigateWith(t, fake, "lmhm"); err != nil {
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

	if err := navigateWith(t, fake, "hjmkm"); err != nil {
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
