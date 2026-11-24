package l3

import (
	"strings"

	"github.com/ryangerardwilson/tesseract/internal/components/l1"
	"github.com/ryangerardwilson/tesseract/internal/components/l2"
	"github.com/ryangerardwilson/tesseract/internal/space"
)

type SpaceState struct {
	Workspaces []space.Workspace
	Active     int
	Column     int
	Layer      int
	Width      int
	// Moving is set while a window is carried; the footer then says SHIFT.
	Moving bool
}

// restFooter is the footer with no window carried, on one line, or broken
// before n new when the terminal is narrower than that line.
func restFooter(width int) string {
	const keys, more = "enter focus  h/l workspace  k/j app", "n new  dd close  q quit"
	if width >= len(keys)+2+len(more) {
		return keys + "  " + more
	}
	return keys + "\n" + more
}

func RenderSpace(theme l1.Theme, state SpaceState) string {
	width := state.Width
	if width < 40 {
		width = 40
	}
	var layers []space.Layer
	if state.Column >= 0 && state.Column < len(state.Workspaces) {
		layers = state.Workspaces[state.Column].Layers
	}
	footer := theme.Muted.Render(restFooter(width))
	if state.Moving {
		footer = theme.Title.Render("SHIFT  h/l k/j carry  s put down  q cancel")
	}
	lines := []string{
		theme.Title.Render("tesseract space"),
		l2.RenderWorkspaceRow(theme, state.Workspaces, state.Active, state.Column),
		l2.RenderLayerStack(theme, layers, state.Layer, width),
		footer,
	}
	return strings.Join(lines, "\n")
}
