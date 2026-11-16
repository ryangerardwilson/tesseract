package browser

import (
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/ryangerardwilson/tesseract/internal/components/l1"
	"github.com/ryangerardwilson/tesseract/internal/components/l3"
	"github.com/ryangerardwilson/tesseract/internal/quickshell"
	"github.com/ryangerardwilson/tesseract/internal/snapshot"
	"github.com/ryangerardwilson/tesseract/internal/tmux"
)

type model struct {
	api          *tmux.API
	theme        l1.Theme
	allSessions  []tmux.Session
	sessions     []tmux.Session
	selected     int
	mode         string
	filter       string
	input        textinput.Model
	width        int
	height       int
	status       string
	errText      string
	attachTarget string
	lastSnapshot time.Time
}

type refreshMsg struct {
	sessions []tmux.Session
	err      error
}

type snapshotTickMsg struct{}

func Run(api *tmux.API, persistent bool) int {
	initialStatus := ""
	if persistent {
		result, err := snapshot.RestoreSavedSessionsIfNeeded(api)
		if err != nil {
			initialStatus = err.Error()
		} else if result != nil {
			switch len(result.RestoredSessions) {
			case 0:
			case 1:
				initialStatus = "Restored 1 session"
			default:
				initialStatus = "Restored " + itoa(len(result.RestoredSessions)) + " sessions"
			}
		}
	}
	input := textinput.New()
	input.Prompt = ""
	input.CharLimit = 80
	m := model{
		api:          api,
		theme:        l1.DefaultTheme(),
		mode:         "select",
		input:        input,
		status:       initialStatus,
		lastSnapshot: time.Now(),
	}
	program := tea.NewProgram(m, tea.WithAltScreen())
	result, err := program.Run()
	if err != nil {
		return 1
	}
	finalModel, ok := result.(model)
	if !ok {
		return 1
	}
	if finalModel.attachTarget != "" {
		return tmux.AttachOrCreateSession(api, finalModel.attachTarget)
	}
	return 0
}

func (m model) Init() tea.Cmd {
	return tea.Batch(m.refresh(), snapshotTick())
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch value := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = value.Width
		m.height = value.Height
		return m, nil
	case refreshMsg:
		if value.err != nil {
			m.errText = value.err.Error()
			return m, nil
		}
		m.errText = ""
		m.allSessions = value.sessions
		m.applyFilter()
		return m, nil
	case snapshotTickMsg:
		if time.Since(m.lastSnapshot) >= snapshot.IntervalSeconds*time.Second {
			_, _ = snapshot.WriteRuntimeSnapshot(m.api, sessionNames(m.allSessions))
			m.lastSnapshot = time.Now()
		}
		return m, snapshotTick()
	case tea.KeyMsg:
		return m.handleKey(value)
	}
	return m, nil
}

func (m model) View() string {
	return l3.RenderBrowser(m.theme, l3.BrowserState{
		Sessions: m.sessions,
		Selected: m.selected,
		Width:    m.width,
		Height:   m.height,
		Mode:     m.mode,
		Input:    m.input.Value(),
		Status:   m.status,
		Error:    m.errText,
		Help:     m.mode == "help",
	})
}

func (m model) handleKey(key tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.mode == "help" {
		switch key.String() {
		case "esc", "q", "?":
			m.mode = "select"
		}
		return m, nil
	}
	if m.mode != "select" {
		return m.handleInputKey(key)
	}
	switch key.String() {
	case "q", "ctrl+c":
		return m, tea.Quit
	case "?", "h":
		m.mode = "help"
		return m, nil
	case "j", "down":
		m.move(1)
		return m, nil
	case "k", "up":
		m.move(-1)
		return m, nil
	case "enter", "l":
		if session := m.selectedSession(); session != "" {
			m.attachTarget = session
			return m, tea.Quit
		}
	case "n":
		return m.beginInput("create", "")
	case "r":
		return m.beginInput("rename", m.selectedSession())
	case "/":
		return m.beginInput("filter", m.filter)
	case "esc":
		m.filter = ""
		m.applyFilter()
		return m, nil
	case "d":
		if session := m.selectedSession(); session != "" {
			if _, err := quickshell.DeleteSessionPayload(m.api, session); err != nil {
				m.errText = err.Error()
				return m, nil
			}
			m.status = "Deleted " + session
			return m, m.refresh()
		}
	}
	return m, nil
}

func (m model) handleInputKey(key tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch key.String() {
	case "esc", "ctrl+c":
		m.mode = "select"
		m.input.Blur()
		return m, nil
	case "enter":
		value := strings.TrimSpace(m.input.Value())
		switch m.mode {
		case "filter":
			m.filter = strings.ToLower(value)
			m.mode = "select"
			m.input.Blur()
			m.applyFilter()
			return m, nil
		case "create":
			if value == "" {
				return m, nil
			}
			if _, err := quickshell.CreateSessionPayload(m.api, value); err != nil {
				m.errText = err.Error()
				return m, nil
			}
			m.status = "Created " + value
			m.mode = "select"
			m.input.Blur()
			return m, m.refresh()
		case "rename":
			current := m.selectedSession()
			if current == "" || value == "" {
				return m, nil
			}
			if _, err := quickshell.RenameSessionPayload(m.api, current, value); err != nil {
				m.errText = err.Error()
				return m, nil
			}
			m.status = "Renamed " + current + " to " + value
			m.mode = "select"
			m.input.Blur()
			return m, m.refresh()
		}
	}
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(key)
	return m, cmd
}

func (m model) beginInput(mode, value string) (tea.Model, tea.Cmd) {
	if mode == "rename" && value == "" {
		return m, nil
	}
	m.mode = mode
	m.input.SetValue(value)
	m.input.CursorEnd()
	m.input.Focus()
	return m, textinput.Blink
}

func (m *model) move(delta int) {
	if len(m.sessions) == 0 {
		m.selected = 0
		return
	}
	m.selected = (m.selected + delta + len(m.sessions)) % len(m.sessions)
}

func (m *model) applyFilter() {
	filter := strings.TrimSpace(strings.ToLower(m.filter))
	if filter == "" {
		m.sessions = append([]tmux.Session{}, m.allSessions...)
	} else {
		m.sessions = m.sessions[:0]
		for _, session := range m.allSessions {
			if strings.HasPrefix(strings.ToLower(session.Name), filter) {
				m.sessions = append(m.sessions, session)
			}
		}
	}
	if m.selected >= len(m.sessions) {
		m.selected = len(m.sessions) - 1
	}
	if m.selected < 0 {
		m.selected = 0
	}
}

func (m model) selectedSession() string {
	if len(m.sessions) == 0 || m.selected < 0 || m.selected >= len(m.sessions) {
		return ""
	}
	return m.sessions[m.selected].Name
}

func (m model) refresh() tea.Cmd {
	api := m.api
	return func() tea.Msg {
		sessions, err := api.ListSessions(true)
		if err != nil {
			return refreshMsg{err: err}
		}
		statuses, _ := api.ListSessionAgentStatuses()
		visible := make([]tmux.Session, 0, len(sessions))
		for _, session := range sessions {
			if session.Name == tmux.IndexSessionName {
				continue
			}
			status := statuses[session.Name]
			session.AgentTotal = status.Total
			session.AgentWorking = status.Working
			session.AgentIdle = status.Idle
			visible = append(visible, session)
		}
		return refreshMsg{sessions: snapshot.SessionsInRecencyOrder(api.Env["HOME"], visible)}
	}
}

func snapshotTick() tea.Cmd {
	return tea.Tick(time.Minute, func(time.Time) tea.Msg {
		return snapshotTickMsg{}
	})
}

func sessionNames(sessions []tmux.Session) []string {
	names := make([]string, 0, len(sessions))
	for _, session := range sessions {
		names = append(names, session.Name)
	}
	return names
}

func itoa(value int) string {
	if value == 0 {
		return "0"
	}
	digits := make([]byte, 0, 8)
	for value > 0 {
		digits = append([]byte{byte('0' + value%10)}, digits...)
		value /= 10
	}
	return string(digits)
}
