package l2

import (
	"fmt"
	"strings"

	"github.com/ryangerardwilson/tesseract/internal/components/l1"
	"github.com/ryangerardwilson/tesseract/internal/tmux"
)

func RenderSessionList(theme l1.Theme, sessions []tmux.Session, selected int, width int) string {
	if width < 20 {
		width = 20
	}
	if len(sessions) == 0 {
		return theme.Muted.Render("No tmux sessions")
	}
	lines := make([]string, 0, len(sessions))
	for index, session := range sessions {
		prefix := "  "
		if index == selected {
			prefix = "> "
		}
		agents := ""
		if session.AgentTotal > 0 {
			agents = fmt.Sprintf("  ai:%d/%d", session.AgentWorking, session.AgentTotal)
		}
		line := prefix + truncate(session.Name, max(8, width-18-len(agents)))
		meta := fmt.Sprintf("  windows:%d%s", session.Windows, agents)
		padding := width - visibleLen(line) - visibleLen(meta)
		if padding < 1 {
			padding = 1
		}
		line += strings.Repeat(" ", padding) + theme.Badge.Render(meta)
		if index == selected {
			line = theme.Selected.Width(width).Render(line)
		} else {
			line = theme.Normal.Width(width).Render(line)
		}
		lines = append(lines, line)
	}
	return strings.Join(lines, "\n")
}

func truncate(value string, width int) string {
	if width <= 0 {
		return ""
	}
	if len(value) <= width {
		return value
	}
	if width <= 1 {
		return value[:width]
	}
	if width <= 3 {
		return value[:width]
	}
	return value[:width-3] + "..."
}

func visibleLen(value string) int {
	return len([]rune(value))
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
