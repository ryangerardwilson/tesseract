// Package spacenav is the terminal navigator for the Hyprland space, the
// fallback when the Quickshell overlay cannot run.
package spacenav

import (
	"slices"

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
	// carried is the window the first m picked up, nil when none is.
	carried *carried
	// placed is where the second m put it down; it moves after the
	// navigator exits.
	placed *placement
}

// carried is a window on its way to a new place, and where the navigator
// stood when m picked it up, so a cancel can go back there.
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
	if ok && final.placed != nil {
		return final.placed.apply(run)
	}
	if !ok || final.target == nil {
		return nil
	}
	return space.Focus(run, *final.target)
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

func (m model) handleKey(key tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch key.String() {
	case "ctrl+c":
		return m, tea.Quit
	case "q", "esc":
		if m.carried == nil {
			return m, tea.Quit
		}
		m.cancel()
	case "m":
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
	}
	return m, nil
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

// carry picks up the selected window on the first m and puts it down on the
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

// cancel puts the carried window back where m picked it up.
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
