package browser

import "github.com/charmbracelet/lipgloss"

type Theme struct {
	Title     lipgloss.Style
	Muted     lipgloss.Style
	Selected  lipgloss.Style
	Normal    lipgloss.Style
	Status    lipgloss.Style
	Error     lipgloss.Style
	Badge     lipgloss.Style
	Prompt    lipgloss.Style
	HelpTitle lipgloss.Style
}

func DefaultTheme() Theme {
	return Theme{
		Title:     lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("15")),
		Muted:     lipgloss.NewStyle().Foreground(lipgloss.Color("245")),
		Selected:  lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("15")).Background(lipgloss.Color("240")),
		Normal:    lipgloss.NewStyle().Foreground(lipgloss.Color("252")),
		Status:    lipgloss.NewStyle().Foreground(lipgloss.Color("245")),
		Error:     lipgloss.NewStyle().Foreground(lipgloss.Color("203")),
		Badge:     lipgloss.NewStyle().Foreground(lipgloss.Color("245")),
		Prompt:    lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("15")),
		HelpTitle: lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("15")),
	}
}
