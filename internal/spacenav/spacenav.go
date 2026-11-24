// Package spacenav is the terminal navigator for the Hyprland space, the
// fallback when the Quickshell overlay cannot run.
package spacenav

import (
	"os/signal"
	"slices"
	"syscall"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/ryangerardwilson/tesseract/internal/components/l1"
	"github.com/ryangerardwilson/tesseract/internal/components/l3"
	"github.com/ryangerardwilson/tesseract/internal/space"
)

type model struct {
	theme  l1.Theme
	space  space.Space
	column int
	layer  int
	width  int
	target *space.Target
	// carried is the window the first s picked up, nil when none is.
	carried *carried
	// placed is where the second s put it down; it moves after the
	// navigator exits.
	placed *placement
	// created is set by n: a new workspace is added after the navigator
	// exits.
	created bool
	// closing is the workspace dd closes after the navigator exits, 0 for
	// none.
	closing int
	// armed is when the d that dd starts with was pressed, zero when no d
	// is waiting for its second.
	armed time.Time
}

// now is the clock dd is timed by. Tests replace it.
var now = time.Now

// A second d closes the selected workspace when it follows the first within
// closeWindow. A d sooner than repeatGap after the last one is a held key
// repeating, so it only arms again: a terminal cannot tell a repeat from a
// press, and Hyprland repeats every 25 ms.
const (
	closeWindow = 400 * time.Millisecond
	repeatGap   = 60 * time.Millisecond
)

// carried is a window on its way to a new place, and where the navigator
// stood when s picked it up, so a cancel can go back there.
type carried struct {
	window space.Layer
	column int
	layer  int
}

// placement is a carried window put down on a new workspace or layer.
type placement struct {
	target space.Target
	// raise focuses the window after the move. At the front of another
	// workspace the move is the whole commit. Behind another window, or back
	// on its own workspace, focus lands it on top, because no z-order
	// dispatcher's arguments are documented.
	raise bool
}

// Run lists the space, lets the operator choose a layer or carry a window to
// a new place, and focuses or moves it after the navigator exits. Moving the
// selection, carrying, cancelling, and quitting change nothing.
func Run(run space.Runner) error {
	return navigate(run, tea.WithAltScreen())
}

func navigate(run space.Runner, options ...tea.ProgramOption) error {
	current, err := space.List(run)
	if err != nil {
		return err
	}
	result, err := tea.NewProgram(newModel(current), options...).Run()
	if err != nil {
		return err
	}
	final, ok := result.(model)
	if !ok {
		return nil
	}
	if final.closing > 0 {
		// The terminal running the navigator may be one of the apps it
		// closes. Its hangup must not stop the other closes or the landing.
		signal.Ignore(syscall.SIGHUP)
	}
	return final.commit(run)
}

// commit runs what the navigator chose once it has exited: a move, a new
// workspace, a close, or a focus. Quitting chose nothing.
func (m model) commit(run space.Runner) error {
	switch {
	case m.placed != nil:
		return m.placed.apply(run)
	case m.created:
		_, err := space.New(run)
		return err
	case m.closing > 0:
		_, err := space.Close(run, m.closing)
		return err
	case m.target != nil:
		return space.Focus(run, *m.target)
	}
	return nil
}

// apply moves the window through the same check and dispatch as tesseract
// space move, then focuses it when it has to land on top.
func (p placement) apply(run space.Runner) error {
	if err := space.Move(run, p.target); err != nil {
		return err
	}
	if !p.raise {
		return nil
	}
	return space.Focus(run, p.target)
}

// newModel starts on the active workspace's front layer, or on the first
// workspace when the active one is not on the X axis.
func newModel(current space.Space) model {
	m := model{theme: l1.DefaultTheme(), space: current}
	for index, workspace := range current.Workspaces {
		if workspace.ID == current.Active {
			m.column = index
		}
	}
	return m
}

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch value := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = value.Width
		return m, nil
	case tea.KeyMsg:
		return m.handleKey(value)
	}
	return m, nil
}

func (m model) View() string {
	return l3.RenderSpace(m.theme, l3.SpaceState{
		Workspaces: m.shown(),
		Active:     m.space.Active,
		Column:     m.column,
		Layer:      m.layer,
		Width:      m.width,
		Moving:     m.carried != nil,
	})
}

// handleKey takes the navigator's keys: s picks up and puts down, enter or
// ctrl+m confirms and never shifts, n adds a workspace, and dd closes the
// selected one. Bare m is not a key. Any key but d forgets a waiting d first.
func (m model) handleKey(key tea.KeyMsg) (tea.Model, tea.Cmd) {
	name := key.String()
	if name != "d" {
		m.armed = time.Time{}
	}
	switch name {
	case "ctrl+c":
		return m, tea.Quit
	case "q", "esc":
		if m.carried == nil {
			return m, tea.Quit
		}
		m.cancel()
	case "s":
		if m.carry() {
			return m, tea.Quit
		}
	case "h", "left":
		m.moveColumn(-1)
	case "l", "right":
		m.moveColumn(1)
	case "k", "up":
		m.moveLayer(-1)
	case "j", "down":
		m.moveLayer(1)
	case "enter", "ctrl+m":
		if target, ok := m.selected(); ok && m.carried == nil {
			m.target = &target
			return m, tea.Quit
		}
	case "n":
		if m.carried == nil {
			m.created = true
			return m, tea.Quit
		}
	case "d":
		if m.pressClose() {
			return m, tea.Quit
		}
	}
	return m, nil
}

// pressClose arms dd on a d, and reports whether this d is the second one,
// which closes the selected workspace. While a window is carried, or with one
// workspace, d does nothing.
func (m *model) pressClose() bool {
	if m.carried != nil || len(m.space.Workspaces) < 2 {
		return false
	}
	pressed := now()
	if gap := pressed.Sub(m.armed); !m.armed.IsZero() && gap >= repeatGap && gap <= closeWindow {
		m.armed = time.Time{}
		m.closing = m.space.Workspaces[m.column].ID
		return true
	}
	m.armed = pressed
	return false
}

// moveColumn steps on X, cycles past either end, and starts the new
// workspace at its front layer.
func (m *model) moveColumn(delta int) {
	next := cycle(m.column+delta, len(m.space.Workspaces))
	if next != m.column {
		m.column = next
		m.layer = 0
	}
}

// moveLayer steps on Z: -1 toward the front, +1 deeper, cycling between
// the front layer and the deepest.
func (m *model) moveLayer(delta int) {
	m.layer = cycle(m.layer+delta, len(m.layers()))
}

// carry picks up the selected window on the first s and puts it down on the
// second. It reports whether the window was put down in a new place.
func (m *model) carry() bool {
	if m.carried != nil {
		return m.putDown()
	}
	if layers := m.layers(); m.layer < len(layers) {
		m.carried = &carried{window: layers[m.layer], column: m.column, layer: m.layer}
	}
	return false
}

// putDown ends the carry where h, j, k, and l took the window. Put down where
// it was picked up, nothing moves.
func (m *model) putDown() bool {
	from := *m.carried
	m.carried = nil
	if m.column == from.column && m.layer == from.layer {
		return false
	}
	m.placed = &placement{
		target: space.Target{Workspace: m.space.Workspaces[m.column].ID, Address: from.window.Address},
		raise:  m.layer > 0 || m.column == from.column,
	}
	return true
}

// cancel puts the carried window back where s picked it up.
func (m *model) cancel() {
	m.column, m.layer = m.carried.column, m.carried.layer
	m.carried = nil
}

// shown is the space as the navigator draws it. With a window carried, it is
// the space as if that window were put down at the selected workspace and
// layer: off its old workspace, with the other layers in their order.
func (m model) shown() []space.Workspace {
	if m.carried == nil {
		return m.space.Workspaces
	}
	shown := make([]space.Workspace, len(m.space.Workspaces))
	for index, workspace := range m.space.Workspaces {
		layers := make([]space.Layer, 0, len(workspace.Layers)+1)
		for _, layer := range workspace.Layers {
			if layer.Address != m.carried.window.Address {
				layers = append(layers, layer)
			}
		}
		if index == m.column {
			layers = slices.Insert(layers, min(m.layer, len(layers)), m.carried.window)
		}
		workspace.Layers = layers
		shown[index] = workspace
	}
	return shown
}

func (m model) layers() []space.Layer {
	workspaces := m.shown()
	if m.column >= len(workspaces) {
		return nil
	}
	return workspaces[m.column].Layers
}

func (m model) selected() (space.Target, bool) {
	layers := m.layers()
	if m.layer >= len(layers) {
		return space.Target{}, false
	}
	return space.Target{Workspace: m.space.Workspaces[m.column].ID, Address: layers[m.layer].Address}, true
}

// cycle wraps index into [0, count), and is 0 when count is 0.
func cycle(index, count int) int {
	if count <= 0 {
		return 0
	}
	return ((index % count) + count) % count
}
