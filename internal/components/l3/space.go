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
	// Moving is set while a window is carried; the footer then says MOVE.
	Moving bool
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
	footer := theme.Muted.Render("enter focus  h/l workspace  k/j layer  q quit")
	if state.Moving {
		footer = theme.Title.Render("MOVE  h/l k/j carry  m put down  q cancel")
	}
	lines := []string{
		theme.Title.Render("tesseract space"),
		l2.RenderWorkspaceRow(theme, state.Workspaces, state.Active, state.Column),
		l2.RenderLayerStack(theme, layers, state.Layer, width),
		footer,
	}
	return strings.Join(lines, "\n")
}
