package snapshot

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/ryangerardwilson/tesseract/internal/tmux"
)

const (
	IntervalSeconds = 3600
	Version         = 1
	FileName        = "session_snapshot.json"
)

type PaneSnapshot struct {
	CWD           string `json:"cwd"`
	Kind          string `json:"kind"`
	CodexThreadID string `json:"codex_thread_id,omitempty"`
	CodexProfile  string `json:"codex_profile,omitempty"`
	Screen        string `json:"screen,omitempty"`
}

type WindowSnapshot struct {
	Name               string         `json:"name"`
	Layout             string         `json:"layout"`
	ActivePanePosition int            `json:"active_pane_position"`
	Panes              []PaneSnapshot `json:"panes"`
}

type SessionSnapshot struct {
	Name                 string           `json:"name"`
	LastAccessed         int64            `json:"last_accessed,omitempty"`
	ActiveWindowPosition int              `json:"active_window_position"`
	Windows              []WindowSnapshot `json:"windows"`
}

type SavedSnapshot struct {
	Version    int               `json:"version"`
	CapturedAt int64             `json:"captured_at"`
	Sessions   []SessionSnapshot `json:"sessions"`
}

type RestoreResult struct {
	RestoredSessions []string
	SkippedSessions  []string
	FailedSessions   []string
}

func Path(home string) string {
	if home == "" {
		var err error
		home, err = os.UserHomeDir()
		if err != nil || home == "" {
			home = "."
		}
	}
	return filepath.Join(home, ".tesseract", "state", FileName)
}

func LoadSavedSnapshot(home string) (*SavedSnapshot, error) {
	dbPath, err := openDB(home)
	if err != nil {
		return nil, fmt.Errorf("Unable to read saved sessions: %w", err)
	}
	if err := importLegacySnapshot(home, dbPath); err != nil {
		return nil, err
	}
	return readSavedSnapshot(dbPath)
}

func WriteRuntimeSnapshot(api *tmux.API, sessionOrder []string) (*SavedSnapshot, error) {
	return writeRuntimeSnapshotAt(api, sessionOrder, time.Now())
}

func writeRuntimeSnapshotAt(api *tmux.API, sessionOrder []string, now time.Time) (*SavedSnapshot, error) {
	saved, err := LoadSavedSnapshot(api.Env["HOME"])
	if err != nil {
		return nil, err
	}
	if _, err := pruneInactiveLiveSessions(api, saved, now); err != nil {
		return nil, err
	}
	snapshot, err := captureRuntimeSnapshot(api, sessionOrder, savedSessionAccesses(saved), now.Unix())
	if err != nil {
		return nil, err
	}
	if err := writeSavedSnapshot(api.Env["HOME"], snapshot); err != nil {
		return nil, err
	}
	return snapshot, nil
}

func writeSavedSnapshot(home string, snapshot *SavedSnapshot) error {
	if snapshot == nil {
		return fmt.Errorf("Unable to save sessions: missing snapshot")
	}
	dbPath, err := openDB(home)
	if err != nil {
		return fmt.Errorf("Unable to save sessions: %w", err)
	}
	if err := replaceSnapshot(dbPath, snapshot); err != nil {
		return fmt.Errorf("Unable to save sessions: %w", err)
	}
	return nil
}

func WriteRuntimeSnapshotIfNonempty(api *tmux.API, sessionOrder []string) (*SavedSnapshot, error) {
	live, err := liveNonIndexSessions(api)
	if err != nil {
		return nil, err
	}
	if len(live) == 0 {
		return nil, nil
	}
	return WriteRuntimeSnapshot(api, sessionOrder)
}

func CaptureRuntimeSnapshot(api *tmux.API, sessionOrder []string) (*SavedSnapshot, error) {
	return captureRuntimeSnapshot(api, sessionOrder, nil, time.Now().Unix())
}

func captureRuntimeSnapshot(api *tmux.API, sessionOrder []string, previous map[string]int64, capturedAt int64) (*SavedSnapshot, error) {
	sessions, err := liveNonIndexSessions(api)
	if err != nil {
		return nil, err
	}
	sessions = applySessionOrder(sessions, sessionOrder)
	processes, children := api.ProcessTree()
	sessionSnapshots := make([]SessionSnapshot, 0, len(sessions))
	for _, session := range sessions {
		windows, err := api.ListWindows(session.Name)
		if err != nil {
			return nil, err
		}
		windowSnapshots := make([]WindowSnapshot, 0, len(windows))
		activeWindowPosition := 0
		for windowPosition, window := range windows {
			if window.Active {
				activeWindowPosition = windowPosition
			}
			panes, err := api.ListPanes(fmt.Sprintf("%s:%d", session.Name, window.Index))
			if err != nil {
				return nil, err
			}
			paneSnapshots := make([]PaneSnapshot, 0, len(panes))
			activePanePosition := 0
			for panePosition, pane := range panes {
				if pane.Active {
					activePanePosition = panePosition
				}
				resume := api.PaneAIResume(pane, processes, children)
				snap := paneSnapshotForThread(api, pane, resume.ID, processes, children)
				if resume.Kind == "c3po" && resume.ID != "" {
					snap.Kind = "c3po"
					snap.Screen = resume.Screen
					snap.CodexProfile = ""
					snap = applyNamedWorkspace(snap, resume.Workspace, "c3po")
				} else {
					snap = applyGrokWorkspace(snap, resume.Workspace)
				}
				paneSnapshots = append(paneSnapshots, snap)
			}
			if len(paneSnapshots) == 0 {
				paneSnapshots = append(paneSnapshots, PaneSnapshot{
					CWD:  api.DefaultStartDirectory(),
					Kind: "shell",
				})
			}
			windowSnapshots = append(windowSnapshots, WindowSnapshot{
				Name:               window.Name,
				Layout:             window.Layout,
				ActivePanePosition: activePanePosition,
				Panes:              paneSnapshots,
			})
		}
		sessionSnapshots = append(sessionSnapshots, SessionSnapshot{
			Name:                 session.Name,
			LastAccessed:         runtimeSessionLastAccessed(session, previous[session.Name], capturedAt),
			ActiveWindowPosition: activeWindowPosition,
			Windows:              windowSnapshots,
		})
	}
	return &SavedSnapshot{
		Version:    Version,
		CapturedAt: capturedAt,
		Sessions:   sessionSnapshots,
	}, nil
}

func paneSnapshotForThread(api *tmux.API, pane tmux.Pane, threadID string, processes map[int]tmux.ProcessInfo, children map[int][]int) PaneSnapshot {
	kind := "shell"
	codexProfile := ""
	if threadID != "" {
		if api.PaneCodexProcessPID(pane, processes, children) > 0 {
			kind = "codex"
			codexProfile = api.PaneCodexProfile(pane, processes, children)
		} else {
			kind = "grok"
		}
	}
	cwd := pane.CurrentPath
	if cwd == "" {
		cwd = api.DefaultStartDirectory()
	}
	return PaneSnapshot{
		CWD:           cwd,
		Kind:          kind,
		CodexThreadID: threadID,
		CodexProfile:  codexProfile,
	}
}

func applyGrokWorkspace(pane PaneSnapshot, workspace string) PaneSnapshot {
	return applyNamedWorkspace(pane, workspace, "grok")
}

func applyNamedWorkspace(pane PaneSnapshot, workspace, kind string) PaneSnapshot {
	if pane.Kind != kind || workspace == "" {
		return pane
	}
	info, err := os.Stat(workspace)
	if err != nil || !info.IsDir() {
		return pane
	}
	pane.CWD = workspace
	return pane
}

func RestoreSavedSessionsIfNeeded(api *tmux.API) (*RestoreResult, error) {
	saved, err := loadRetainedSavedSnapshot(api, time.Now())
	if err != nil || saved == nil || len(saved.Sessions) == 0 {
		return nil, err
	}
	live, err := liveNonIndexSessions(api)
	if err != nil {
		return nil, err
	}
	if len(live) > 0 {
		return nil, nil
	}
	return RestoreSnapshot(api, saved), nil
}

func RestoreSavedSessions(api *tmux.API) (*RestoreResult, error) {
	saved, err := loadRetainedSavedSnapshot(api, time.Now())
	if err != nil || saved == nil || len(saved.Sessions) == 0 {
		return nil, err
	}
	return RestoreSnapshot(api, saved), nil
}

func SavedSessionsRestoreNeeded(api *tmux.API) (bool, error) {
	saved, err := loadRetainedSavedSnapshot(api, time.Now())
	if err != nil || saved == nil || len(saved.Sessions) == 0 {
		return false, err
	}
	live, err := liveNonIndexSessions(api)
	if err != nil {
		return false, err
	}
	return len(live) == 0, nil
}

func SavedSessionOrder(home string) ([]string, error) {
	saved, err := LoadSavedSnapshot(home)
	if err != nil || saved == nil {
		return nil, err
	}
	saved, _ = retainedSavedSnapshot(saved, time.Now())
	order := make([]string, 0, len(saved.Sessions))
	for _, session := range saved.Sessions {
		order = append(order, session.Name)
	}
	return order, nil
}

func RestoreSnapshot(api *tmux.API, saved *SavedSnapshot) *RestoreResult {
	result := &RestoreResult{}
	for _, session := range saved.Sessions {
		if api.HasSession(session.Name) {
			result.SkippedSessions = append(result.SkippedSessions, session.Name)
			continue
		}
		if err := restoreSession(api, session); err != nil {
			result.FailedSessions = append(result.FailedSessions, session.Name)
			continue
		}
		result.RestoredSessions = append(result.RestoredSessions, session.Name)
	}
	return result
}

func restoreSession(api *tmux.API, session SessionSnapshot) error {
	if len(session.Windows) == 0 {
		return nil
	}
	windowBase := api.WindowBaseIndex()
	firstWindow := session.Windows[0]
	firstPane := firstWindow.Panes[0]
	if rc := newSessionWithShellFallback(api, session.Name, true, coalesce(firstPane.CWD, api.DefaultStartDirectory()), firstWindow.Name, restoreCommand(firstPane)); rc != 0 {
		return fmt.Errorf("Unable to restore session: %s", session.Name)
	}
	if err := restoreWindowLayout(api, session.Name, windowBase, firstWindow); err != nil {
		return err
	}
	for windowPosition, window := range session.Windows[1:] {
		firstWindowPane := window.Panes[0]
		if rc := newWindowWithShellFallback(api, session.Name, window.Name, coalesce(firstWindowPane.CWD, api.DefaultStartDirectory()), restoreCommand(firstWindowPane)); rc != 0 {
			return fmt.Errorf("Unable to restore session: %s", session.Name)
		}
		if err := restoreWindowLayout(api, session.Name, windowBase+windowPosition+1, window); err != nil {
			return err
		}
	}
	return api.SelectWindow(fmt.Sprintf("%s:%d", session.Name, windowBase+session.ActiveWindowPosition))
}

func restoreWindowLayout(api *tmux.API, sessionName string, windowIndex int, window WindowSnapshot) error {
	paneBase := api.PaneBaseIndex()
	target := fmt.Sprintf("%s:%d", sessionName, windowIndex)
	for _, pane := range window.Panes[1:] {
		if rc := splitWindowWithShellFallback(api, target, coalesce(pane.CWD, api.DefaultStartDirectory()), restoreCommand(pane)); rc != 0 {
			return fmt.Errorf("Unable to restore session: %s", sessionName)
		}
	}
	if window.Layout != "" {
		if err := api.SelectLayout(target, window.Layout); err != nil {
			return err
		}
	}
	return api.SelectPane(fmt.Sprintf("%s.%d", target, paneBase+window.ActivePanePosition))
}

func newSessionWithShellFallback(api *tmux.API, sessionName string, detached bool, startDirectory, windowName, command string) int {
	rc := api.NewSession(sessionName, detached, startDirectory, windowName, command)
	if command == "" {
		return rc
	}
	if api.HasSession(sessionName) {
		return 0
	}
	return api.NewSession(sessionName, detached, startDirectory, windowName, "")
}

func newWindowWithShellFallback(api *tmux.API, sessionName, windowName, startDirectory, command string) int {
	rc := api.NewWindow(sessionName, windowName, startDirectory, command)
	if rc == 0 || command == "" {
		return rc
	}
	return api.NewWindow(sessionName, windowName, startDirectory, "")
}

func splitWindowWithShellFallback(api *tmux.API, target, startDirectory, command string) int {
	rc := api.SplitWindow(target, startDirectory, command)
	if rc == 0 || command == "" {
		return rc
	}
	return api.SplitWindow(target, startDirectory, "")
}

func restoreCommand(pane PaneSnapshot) string {
	if pane.CodexThreadID == "" {
		return ""
	}
	if pane.Kind == "codex" {
		command := "codex"
		if pane.CodexProfile != "" {
			command += " --profile " + tmux.ShellQuote(pane.CodexProfile)
		}
		command += " resume " + tmux.ShellQuote(pane.CodexThreadID)
		return commandWithShellFallback(command)
	}
	if pane.Kind == "grok" {
		// Grok supports --resume <session-id> (see ~/.grok/docs/user-guide/17-sessions.md)
		return commandWithShellFallback("grok --resume " + tmux.ShellQuote(pane.CodexThreadID))
	}
	if pane.Kind == "c3po" {
		if pane.Screen == "" || pane.CodexThreadID == "" {
			return ""
		}
		return commandWithShellFallback("c3po tui " + tmux.ShellQuote(pane.Screen) + " --resume " + tmux.ShellQuote(pane.CodexThreadID))
	}
	return ""
}

func commandWithShellFallback(command string) string {
	return command + `; exec "${SHELL:-/bin/sh}"`
}

func liveNonIndexSessions(api *tmux.API) ([]tmux.Session, error) {
	sessions, err := api.ListSessions(true)
	if err != nil {
		return nil, err
	}
	live := make([]tmux.Session, 0, len(sessions))
	for _, session := range sessions {
		if session.Name != tmux.IndexSessionName {
			live = append(live, session)
		}
	}
	return live, nil
}

func applySessionOrder(sessions []tmux.Session, sessionOrder []string) []tmux.Session {
	if len(sessionOrder) == 0 {
		return sessions
	}
	order := map[string]int{}
	for index, name := range sessionOrder {
		order[name] = index
	}
	fallback := len(order)
	out := append([]tmux.Session{}, sessions...)
	sortSessions(out, func(session tmux.Session) int {
		if index, ok := order[session.Name]; ok {
			return index
		}
		return fallback
	})
	return out
}

func sortSessions(sessions []tmux.Session, key func(tmux.Session) int) {
	for i := 1; i < len(sessions); i++ {
		for j := i; j > 0 && key(sessions[j]) < key(sessions[j-1]); j-- {
			sessions[j], sessions[j-1] = sessions[j-1], sessions[j]
		}
	}
}

func coalesce(value, fallback string) string {
	if value != "" {
		return value
	}
	return fallback
}
