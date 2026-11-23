package l2

import (
	"strconv"
	"strings"
	"unicode"

	"github.com/charmbracelet/lipgloss"
	"github.com/ryangerardwilson/tesseract/internal/components/l1"
	"github.com/ryangerardwilson/tesseract/internal/space"
)

// RenderWorkspaceRow draws the X axis as one row. The selected workspace is
// highlighted and the active workspace carries a star.
func RenderWorkspaceRow(theme l1.Theme, workspaces []space.Workspace, active int, selected int) string {
	if len(workspaces) == 0 {
		return theme.Muted.Render("No workspaces")
	}
	cells := make([]string, 0, len(workspaces))
	for index, workspace := range workspaces {
		mark := " "
		if workspace.ID == active {
			mark = "*"
		}
		cell := " " + workspaceLabel(workspace) + mark
		if index == selected {
			cell = theme.Selected.Render(cell)
		} else {
			cell = theme.Normal.Render(cell)
		}
		cells = append(cells, cell)
	}
	return strings.Join(cells, " ")
}

// RenderLayerStack draws one workspace's Z axis, front layer first, each
// layer as its class and title.
func RenderLayerStack(theme l1.Theme, layers []space.Layer, selected int, width int) string {
	if width < 20 {
		width = 20
	}
	if len(layers) == 0 {
		return theme.Muted.Render("No windows")
	}
	classWidth := 0
	for _, layer := range layers {
		classWidth = max(classWidth, lipgloss.Width(clean(layer.Class)))
	}
	classWidth = min(classWidth, width/3)
	lines := make([]string, 0, len(layers))
	for index, layer := range layers {
		prefix := "  "
		if index == selected {
			prefix = "> "
		}
		class := clip(clean(layer.Class), classWidth)
		line := prefix + class + strings.Repeat(" ", classWidth-lipgloss.Width(class)+2) + clip(clean(layer.Title), width-classWidth-4)
		if index == selected {
			line = theme.Selected.Width(width).Render(line)
		} else {
			line = theme.Normal.Width(width).Render(line)
		}
		lines = append(lines, line)
	}
	return strings.Join(lines, "\n")
}

func workspaceLabel(workspace space.Workspace) string {
	if name := clean(workspace.Name); name != "" {
		return name
	}
	return strconv.Itoa(workspace.ID)
}

// clean blanks control characters, so a window title cannot send escape
// sequences to the terminal.
func clean(value string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return ' '
		}
		return r
	}, value)
}

// clip fits value into width terminal cells and ends it with "..." when cut.
func clip(value string, width int) string {
	if lipgloss.Width(value) <= width {
		return value
	}
	if width <= 3 {
		return fitCells(value, width)
	}
	return fitCells(value, width-3) + "..."
}

// fitCells keeps the longest prefix of value that fits in width cells.
func fitCells(value string, width int) string {
	var out strings.Builder
	used := 0
	for _, r := range value {
		cell := lipgloss.Width(string(r))
		if used+cell > width {
			break
		}
		out.WriteRune(r)
		used += cell
	}
	return out.String()
}
