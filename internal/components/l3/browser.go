package l3

import (
	"strings"

	"github.com/ryangerardwilson/tesseract/internal/components/l1"
	"github.com/ryangerardwilson/tesseract/internal/components/l2"
	"github.com/ryangerardwilson/tesseract/internal/tmux"
)

type BrowserState struct {
	Sessions []tmux.Session
	Selected int
	Width    int
	Height   int
	Mode     string
	Input    string
	Status   string
	Error    string
	Help     bool
}

func RenderBrowser(theme l1.Theme, state BrowserState) string {
	width := state.Width
	if width < 40 {
		width = 40
	}
	if state.Help {
		return renderHelp(theme, width)
	}
	lines := []string{theme.Title.Render("tesseract")}
	if state.Mode != "" && state.Mode != "select" {
		lines = append(lines, theme.Prompt.Render(promptLabel(state.Mode)+" "+state.Input))
	}
	lines = append(lines, l2.RenderSessionList(theme, state.Sessions, state.Selected, width))
	if state.Error != "" {
		lines = append(lines, theme.Error.Render(state.Error))
	} else if state.Status != "" {
		lines = append(lines, theme.Status.Render(state.Status))
	} else {
		lines = append(lines, theme.Muted.Render("enter attach  n new  r rename  d delete  / filter  ? help  q quit"))
	}
	return strings.Join(lines, "\n")
}

func renderHelp(theme l1.Theme, width int) string {
	lines := []string{
		theme.HelpTitle.Render("tesseract help"),
		"",
		"enter / l      attach selected session",
		"j / down       next session",
		"k / up         previous session",
		"n             create session",
		"r             rename selected session",
		"d             delete selected session",
		"/             filter by prefix",
		"esc           clear mode",
		"q             quit",
	}
	return theme.Normal.Width(width).Render(strings.Join(lines, "\n"))
}

func promptLabel(mode string) string {
	switch mode {
	case "create":
		return "new:"
	case "rename":
		return "rename:"
	case "filter":
		return "filter:"
	default:
		return mode + ":"
	}
}
