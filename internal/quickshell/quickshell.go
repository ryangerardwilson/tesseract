package quickshell

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/ryangerardwilson/tesseract/internal/assets"
	"github.com/ryangerardwilson/tesseract/internal/snapshot"
	"github.com/ryangerardwilson/tesseract/internal/tmux"
)

const Usage = "valid shape: tesseract quickshell list | tesseract quickshell select <name>"

type PickerResult struct {
	Action string
	Name   string
}

type SessionPayloadItem struct {
	Name         string `json:"name"`
	Attached     int    `json:"attached"`
	Windows      int    `json:"windows"`
	LastAttached int    `json:"last_attached"`
	AgentTotal   int    `json:"agent_total"`
	AgentWorking int    `json:"agent_working"`
	AgentIdle    int    `json:"agent_idle"`
	AgentUnseen  int    `json:"agent_unseen"`
}

type SessionPayload struct {
	Sessions  []SessionPayloadItem `json:"sessions"`
	Preferred string               `json:"preferred,omitempty"`
}

func APIPayload(api *tmux.API, argv []string) (map[string]any, error) {
	if len(argv) == 1 && argv[0] == "list" {
		if _, err := snapshot.RestoreSavedSessionsIfNeeded(api); err != nil {
			return nil, err
		}
		payload, err := Payload(api, "")
		if err != nil {
			return nil, err
		}
		data, err := json.Marshal(payload)
		if err != nil {
			return nil, err
		}
		var out map[string]any
		if err := json.Unmarshal(data, &out); err != nil {
			return nil, err
		}
		return out, nil
	}
	if len(argv) == 2 && argv[0] == "select" {
		selected, err := DesktopSelectSession(api, argv[1])
		if err != nil {
			return nil, err
		}
		return map[string]any{"selected": selected}, nil
	}
	return nil, fmt.Errorf(Usage)
}

func Payload(api *tmux.API, preferred string) (*SessionPayload, error) {
	sessions, err := visibleSessions(api)
	if err != nil {
		return nil, err
	}
	statuses, statusErr := api.ListSessionAgentStatuses()
	var unseen map[string]bool
	// Without pane statuses every session would look agentless and lose
	// its settled time, so a failed scan leaves the saved state alone.
	if statusErr == nil {
		unseen, _ = snapshot.RecordAgentActivity(api.Env["HOME"], agentActivity(sessions, statuses), time.Now())
	}
	payload := &SessionPayload{
		Sessions: make([]SessionPayloadItem, 0, len(sessions)),
	}
	if preferred != "" {
		payload.Preferred = preferred
	}
	for _, session := range sessions {
		payload.Sessions = append(payload.Sessions, payloadItem(session, statuses[session.Name], unseen[session.Name]))
	}
	return payload, nil
}

func payloadItem(session tmux.Session, status tmux.AgentStatus, unseen bool) SessionPayloadItem {
	item := SessionPayloadItem{
		Name:         session.Name,
		Attached:     session.Attached,
		Windows:      session.Windows,
		LastAttached: session.LastAttached,
		AgentTotal:   status.Total,
		AgentWorking: status.Working,
		AgentIdle:    status.Idle,
	}
	if unseen && status.Total > 0 && status.Working == 0 {
		item.AgentUnseen = 1
	}
	return item
}

func agentActivity(sessions []tmux.Session, statuses map[string]tmux.AgentStatus) []snapshot.AgentActivity {
	activity := make([]snapshot.AgentActivity, 0, len(sessions))
	for _, session := range sessions {
		status := statuses[session.Name]
		activity = append(activity, snapshot.AgentActivity{
			Name:     session.Name,
			Attached: session.Attached > 0,
			Total:    status.Total,
			Working:  status.Working,
		})
	}
	return activity
}

func WritePayload(api *tmux.API, dataPath, preferred string) error {
	payload, err := Payload(api, preferred)
	if err != nil {
		return err
	}
	return WritePayloadData(dataPath, payload)
}

func WritePayloadData(dataPath string, payload any) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	return os.WriteFile(dataPath, data, 0o644)
}

func CreateSessionPayload(api *tmux.API, name string) (*SessionPayload, error) {
	sessionName, err := ValidateSessionName(name)
	if err != nil {
		return nil, err
	}
	created, err := tmux.EnsureSessionExists(api, sessionName, true)
	if err != nil {
		return nil, err
	}
	if created {
		_, _ = snapshot.WriteRuntimeSnapshot(api, nil)
	}
	return Payload(api, sessionName)
}

func CreateSessionForPicker(api *tmux.API, name, dataPath string) (string, error) {
	payload, err := CreateSessionPayload(api, name)
	if err != nil {
		return "", err
	}
	if err := WritePayloadData(dataPath, payload); err != nil {
		return "", err
	}
	return payload.Preferred, nil
}

func DeleteSessionPayload(api *tmux.API, name string) (*SessionPayload, error) {
	sessionName, err := ValidateSessionName(name)
	if err != nil {
		return nil, err
	}
	if !api.HasSession(sessionName) {
		return nil, fmt.Errorf("No such session: %s", sessionName)
	}
	previousSessions, err := visibleSessions(api)
	if err != nil {
		return nil, err
	}
	previousOrder := make([]string, 0, len(previousSessions))
	for _, session := range previousSessions {
		previousOrder = append(previousOrder, session.Name)
	}
	if _, err := tmux.KillSessionSafely(api, sessionName); err != nil {
		return nil, err
	}
	remainingSessions, err := visibleSessions(api)
	if err != nil {
		return nil, err
	}
	preferred := preferredAfterRemoval(previousOrder, sessionName, remainingSessions)
	_, _ = snapshot.WriteRuntimeSnapshot(api, nil)
	return Payload(api, preferred)
}

func DeleteSessionForPicker(api *tmux.API, name, dataPath string) (string, error) {
	payload, err := DeleteSessionPayload(api, name)
	if err != nil {
		return "", err
	}
	if err := WritePayloadData(dataPath, payload); err != nil {
		return "", err
	}
	return payload.Preferred, nil
}

func RenameSessionPayload(api *tmux.API, currentName, newName string) (*SessionPayload, error) {
	current, err := ValidateSessionName(currentName)
	if err != nil {
		return nil, err
	}
	renamed, err := ValidateSessionName(newName)
	if err != nil {
		return nil, err
	}
	if !api.HasSession(current) {
		return nil, fmt.Errorf("No such session: %s", current)
	}
	if renamed != current && api.HasSession(renamed) {
		return nil, fmt.Errorf("Session exists: %s", renamed)
	}
	if renamed != current {
		if err := api.RenameSession(current, renamed); err != nil {
			return nil, err
		}
		_, _ = snapshot.WriteRuntimeSnapshot(api, nil)
	}
	return Payload(api, renamed)
}

func RenameSessionForPicker(api *tmux.API, currentName, newName, dataPath string) (string, error) {
	payload, err := RenameSessionPayload(api, currentName, newName)
	if err != nil {
		return "", err
	}
	if err := WritePayloadData(dataPath, payload); err != nil {
		return "", err
	}
	return payload.Preferred, nil
}

func DesktopSelectSession(api *tmux.API, name string) (string, error) {
	sessionName, err := ValidateSessionName(name)
	if err != nil {
		return "", err
	}
	if _, err := tmux.EnsureSessionExists(api, sessionName, true); err != nil {
		return "", err
	}
	for _, clientTTY := range api.ListAllClientTTYs() {
		if err := api.SwitchClient(sessionName, clientTTY); err != nil {
			return "", err
		}
	}
	return sessionName, nil
}

func PickSession(api *tmux.API, appDir string) (*PickerResult, error) {
	if appDir == "" {
		appDir = assets.DefaultAppDir(api.Env["HOME"])
	}
	pickerDir := filepath.Join(appDir, "quickshell", "session-picker")
	if info, err := os.Stat(pickerDir); err != nil || !info.IsDir() {
		return nil, fmt.Errorf("tesseract quickshell picker is not installed")
	}
	command, err := pickerCommand(pickerDir)
	if err != nil {
		return nil, err
	}
	if _, err := snapshot.RestoreSavedSessionsIfNeeded(api); err != nil {
		return nil, err
	}
	tempDir, err := os.MkdirTemp("", "tesseract-picker-*")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(tempDir)

	dataPath := filepath.Join(tempDir, "sessions.json")
	resultPath := filepath.Join(tempDir, "result.tsv")
	if err := WritePayload(api, dataPath, ""); err != nil {
		return nil, err
	}
	executable, err := os.Executable()
	if err != nil || executable == "" {
		executable = "tesseract"
	}
	for {
		_ = os.Remove(resultPath)
		cmd := exec.Command(command[0], command[1:]...)
		cmd.Stdout = nil
		cmd.Stderr = nil
		cmd.Env = append(apiEnvList(api), []string{
			"TM_QUICKSHELL_PICKER_DATA=" + dataPath,
			"TM_QUICKSHELL_PICKER_OUT=" + resultPath,
			"TM_QUICKSHELL_PICKER_COMMAND=" + executable,
		}...)
		err := cmd.Run()
		if err != nil {
			if _, statErr := os.Stat(resultPath); os.IsNotExist(statErr) {
				return nil, fmt.Errorf("tesseract quickshell picker exited before selection")
			}
		}
		result := readPickerResult(resultPath)
		if result != nil && result.Action == "create" {
			if _, err := CreateSessionForPicker(api, result.Name, dataPath); err != nil {
				return nil, err
			}
			continue
		}
		return result, nil
	}
}

func ValidateSessionName(name string) (string, error) {
	sessionName := strings.TrimSpace(name)
	if sessionName == "" {
		return "", fmt.Errorf("Session name required")
	}
	if sessionName == tmux.IndexSessionName {
		return "", fmt.Errorf("Reserved session: %s", tmux.IndexSessionName)
	}
	if strings.ContainsAny(sessionName, ":\n\r\t") {
		return "", fmt.Errorf("Session name cannot contain : or control characters")
	}
	return sessionName, nil
}

func visibleSessions(api *tmux.API) ([]tmux.Session, error) {
	sessions, err := api.ListSessions(true)
	if err != nil {
		return nil, err
	}
	visible := make([]tmux.Session, 0, len(sessions))
	for _, session := range sessions {
		if session.Name != tmux.IndexSessionName {
			visible = append(visible, session)
		}
	}
	return snapshot.SessionsInRecencyOrder(api.Env["HOME"], visible), nil
}

func preferredAfterRemoval(previousOrder []string, removedName string, remainingSessions []tmux.Session) string {
	if len(remainingSessions) == 0 {
		return ""
	}
	remaining := map[string]bool{}
	for _, session := range remainingSessions {
		remaining[session.Name] = true
	}
	removedIndex := -1
	for index, name := range previousOrder {
		if name == removedName {
			removedIndex = index
			break
		}
	}
	if removedIndex < 0 {
		return remainingSessions[0].Name
	}
	for _, candidate := range append(previousOrder[removedIndex+1:], previousOrder[:removedIndex]...) {
		if remaining[candidate] {
			return candidate
		}
	}
	return remainingSessions[0].Name
}

func pickerCommand(pickerDir string) ([]string, error) {
	if _, err := exec.LookPath("quickshell"); err != nil {
		return nil, fmt.Errorf("'quickshell' is required for the graphical picker")
	}
	if _, err := exec.LookPath("uwsm-app"); err == nil {
		return []string{"uwsm-app", "--", "quickshell", "--path", pickerDir}, nil
	}
	return []string{"quickshell", "--path", pickerDir}, nil
}

func readPickerResult(resultPath string) *PickerResult {
	data, err := os.ReadFile(resultPath)
	if err != nil {
		return nil
	}
	text := strings.TrimSpace(string(data))
	if text == "" {
		return nil
	}
	action, name, ok := strings.Cut(text, "\t")
	if !ok || (action != "select" && action != "create") {
		return nil
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return nil
	}
	return &PickerResult{Action: action, Name: name}
}

func apiEnvList(api *tmux.API) []string {
	env := make([]string, 0, len(api.Env))
	for key, value := range api.Env {
		env = append(env, key+"="+value)
	}
	return env
}
