package tmux

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	IndexSessionName = "index"
	IndexWindowName  = "index"

	listSessionsFormat = "#{session_name}\t#{session_attached}\t#{session_windows}\t#{session_last_attached}\t#{session_created}"
	listWindowsFormat  = "#{window_index}\t#{window_name}\t#{window_active}\t#{window_layout}"
	listPanesFormat    = "#{session_name}\t#{window_index}\t#{pane_index}\t#{pane_id}\t#{pane_pid}\t#{pane_current_command}\t#{pane_current_path}\t#{pane_active}\t#{pane_title}"

	defaultIndexKey = "M-i"
)

var (
	rolloutPathRE        = regexp.MustCompile(`rollout-.*-([0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12})\.jsonl$`)
	grokSessionPathRE    = regexp.MustCompile(`\.grok/sessions/([^/\s]+)/([0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12})(?:/(events\.jsonl|updates\.jsonl|summary\.json))?`)
	uuidRE               = regexp.MustCompile(`(?i)^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)
	managedIndexBindRE   = regexp.MustCompile(`^bind -n "([^"]+)" run-shell "TMUX_CLIENT_TTY=`)
	errTmuxCommandFailed = errors.New("tmux command failed")
)

var managedPrefixKeys = []string{
	"C-Space", "M-a", "q", "h", "v", "x", "r", "c", "k", "R", "C", "K", "P", "N",
}

var managedRootKeys = []string{
	"C-i", "Tab", "C-Insert", "Insert", "F8", "F9", "F12", "M-h", "M-|", "M-\\",
	"M-c", "M--", "M-v", "M-Home", "M-End", "M-DC", "M-p", "M-o", "M-l", "M-k",
	"M-j", "M-H", "M-L", "M-K", "M-J", "C-M-Left", "C-M-Right", "C-M-Up",
	"C-M-Down", "C-M-S-Left", "C-M-S-Down", "C-M-S-Up", "C-M-S-Right", "M-1",
	"M-2", "M-3", "M-4", "M-5", "M-6", "M-7", "M-8", "M-9", "M-Left",
	"M-Right", "M-S-Left", "M-S-Right", "M-Up", "M-Down",
}

var managedCopyModeVIKeys = []string{"v", "y", "M-v", "M-j", "M-k", "C-j", "C-k"}

type AgentStatus struct {
	Total   int
	Working int
	Idle    int
}

type Pane struct {
	SessionName    string
	PaneID         string
	PanePID        int
	CurrentCommand string
	WindowIndex    int
	PaneIndex      int
	CurrentPath    string
	Active         bool
	Title          string
}

type ProcessInfo struct {
	PID  int
	PPID int
	Comm string
	Args string
}

type Session struct {
	Name         string
	Attached     int
	Windows      int
	LastAttached int
	Created      int
	AgentTotal   int
	AgentWorking int
	AgentIdle    int
}

type Window struct {
	Index  int
	Name   string
	Active bool
	Layout string
}

type CommandResult struct {
	Args       []string
	ReturnCode int
	Stdout     string
	Stderr     string
}

type API struct {
	Env map[string]string
}

func NewAPI(env map[string]string) *API {
	if env == nil {
		env = envMap(os.Environ())
	} else {
		env = copyEnv(env)
	}
	return &API{Env: env}
}

func ShellQuote(value string) string {
	if value == "" {
		return "''"
	}
	return "'" + strings.ReplaceAll(value, "'", `'"'"'`) + "'"
}

func SessionTarget(name string) string {
	return "=" + name
}

func (api *API) DefaultStartDirectory() string {
	if home := strings.TrimSpace(api.Env["HOME"]); home != "" {
		return home
	}
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return "."
	}
	return home
}

func (api *API) CurrentClientTTY() string {
	return strings.TrimSpace(api.Env["TMUX_CLIENT_TTY"])
}

func (api *API) HasSession(name string) bool {
	result, _ := api.run([]string{"has-session", "-t", SessionTarget(name)}, false)
	return result.ReturnCode == 0
}

func (api *API) WindowBaseIndex() int {
	return api.numericOption([]string{"show-options", "-gqv", "base-index"}, 0)
}

func (api *API) PaneBaseIndex() int {
	return api.numericOption([]string{"show-options", "-gwqv", "pane-base-index"}, 0)
}

func (api *API) ListSessions(allowMissingServer bool) ([]Session, error) {
	result, err := api.run([]string{"list-sessions", "-F", listSessionsFormat}, false)
	if err != nil {
		return nil, err
	}
	if result.ReturnCode != 0 {
		message := result.ErrorText()
		if allowMissingServer && tmuxServerMissing(message) {
			return nil, nil
		}
		return nil, fmt.Errorf("%s", message)
	}
	return parseSessions(result.Stdout), nil
}

func parseSessions(output string) []Session {
	sessions := make([]Session, 0)
	for _, line := range strings.Split(output, "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		fields := pad(strings.Split(line, "\t"), 5)
		sessions = append(sessions, Session{
			Name:         fields[0],
			Attached:     atoi(fields[1]),
			Windows:      atoi(fields[2]),
			LastAttached: atoi(fields[3]),
			Created:      atoi(fields[4]),
		})
	}
	sort.SliceStable(sessions, func(i, j int) bool {
		return sessions[i].LastAttached > sessions[j].LastAttached
	})
	return sessions
}

func (api *API) ListWindows(sessionName string) ([]Window, error) {
	result, err := api.run([]string{"list-windows", "-t", SessionTarget(sessionName), "-F", listWindowsFormat}, true)
	if err != nil {
		return nil, err
	}
	windows := make([]Window, 0)
	for _, line := range strings.Split(result.Stdout, "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		fields := pad(strings.Split(line, "\t"), 4)
		windows = append(windows, Window{
			Index:  atoi(fields[0]),
			Name:   fields[1],
			Active: atoi(fields[2]) != 0,
			Layout: fields[3],
		})
	}
	sort.SliceStable(windows, func(i, j int) bool {
		return windows[i].Index < windows[j].Index
	})
	return windows, nil
}

func (api *API) ListPanes(target string) ([]Pane, error) {
	args := []string{"list-panes"}
	if target == "" {
		args = append(args, "-a")
	} else {
		args = append(args, "-t", target)
	}
	args = append(args, "-F", listPanesFormat)
	result, err := api.run(args, true)
	if err != nil {
		return nil, err
	}
	panes := make([]Pane, 0)
	for _, line := range strings.Split(result.Stdout, "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		parts := strings.Split(line, "\t")
		if len(parts) > 8 {
			parts = append(parts[:8], strings.Join(parts[8:], "\t"))
		}
		fields := pad(parts, 9)
		panes = append(panes, Pane{
			SessionName:    fields[0],
			WindowIndex:    atoi(fields[1]),
			PaneIndex:      atoi(fields[2]),
			PaneID:         fields[3],
			PanePID:        atoi(fields[4]),
			CurrentCommand: fields[5],
			CurrentPath:    fields[6],
			Active:         atoi(fields[7]) != 0,
			Title:          fields[8],
		})
	}
	sort.SliceStable(panes, func(i, j int) bool {
		left := panes[i]
		right := panes[j]
		if left.SessionName != right.SessionName {
			return left.SessionName < right.SessionName
		}
		if left.WindowIndex != right.WindowIndex {
			return left.WindowIndex < right.WindowIndex
		}
		return left.PaneIndex < right.PaneIndex
	})
	return panes, nil
}

func (api *API) CapturePaneTail(paneID string, lines int) string {
	if lines <= 0 {
		lines = 40
	}
	result, err := api.run([]string{"capture-pane", "-p", "-t", paneID, "-S", fmt.Sprintf("-%d", lines)}, false)
	if err != nil || result.ReturnCode != 0 {
		return ""
	}
	return result.Stdout
}

func (api *API) ProcessTree() (map[int]ProcessInfo, map[int][]int) {
	processes := api.processSnapshot()
	children := map[int][]int{}
	for _, process := range processes {
		children[process.PPID] = append(children[process.PPID], process.PID)
	}
	return processes, children
}

func (api *API) PaneCodexProcessPID(pane Pane, processes map[int]ProcessInfo, children map[int][]int) int {
	for _, process := range api.paneProcesses(pane, processes, children) {
		if isCodexProcess(process) {
			return process.PID
		}
	}
	return 0
}

func (api *API) PaneCodexProfile(pane Pane, processes map[int]ProcessInfo, children map[int][]int) string {
	for _, process := range api.paneProcesses(pane, processes, children) {
		if isCodexProcess(process) {
			if profile := codexProfileFromArgs(process.Args); profile != "" {
				return profile
			}
		}
	}
	return ""
}

func (api *API) PaneAIProcessPID(pane Pane, processes map[int]ProcessInfo, children map[int][]int) int {
	for _, process := range api.paneProcesses(pane, processes, children) {
		if isCodexProcess(process) || isGrokProcess(process) || isClaudeProcess(process) {
			return process.PID
		}
	}
	return 0
}

func (api *API) paneHasGrokProcess(pane Pane, processes map[int]ProcessInfo, children map[int][]int) bool {
	for _, process := range api.paneProcesses(pane, processes, children) {
		if isGrokProcess(process) {
			return true
		}
	}
	return false
}

func (api *API) paneHasClaudeProcess(pane Pane, processes map[int]ProcessInfo, children map[int][]int) bool {
	for _, process := range api.paneProcesses(pane, processes, children) {
		if isClaudeProcess(process) {
			return true
		}
	}
	return false
}

func (api *API) PaneHasBrowserProcess(pane Pane, processes map[int]ProcessInfo, children map[int][]int) bool {
	for _, process := range api.paneProcesses(pane, processes, children) {
		if isTmBrowserProcess(process) {
			return true
		}
	}
	return false
}

func (api *API) PaneCodexThreadID(pane Pane, processes map[int]ProcessInfo, children map[int][]int) string {
	if processes == nil || children == nil {
		processes, children = api.ProcessTree()
	}
	codexProcesses := make([]ProcessInfo, 0)
	for _, process := range api.paneProcesses(pane, processes, children) {
		if isCodexProcess(process) {
			codexProcesses = append(codexProcesses, process)
		}
	}
	for _, process := range codexProcesses {
		if threadID := rolloutThreadIDFromProcessFDs(process.PID); threadID != "" {
			return threadID
		}
	}
	for _, process := range codexProcesses {
		cmd := exec.Command("lsof", "-p", strconv.Itoa(process.PID))
		cmd.Env = api.envList()
		var stdout bytes.Buffer
		cmd.Stdout = &stdout
		if err := cmd.Run(); err != nil {
			var exitErr *exec.ExitError
			if errors.As(err, &exitErr) {
				continue
			}
			return ""
		}
		if threadID := rolloutThreadIDFromOutput(stdout.String()); threadID != "" {
			return threadID
		}
	}
	return ""
}

func (api *API) PaneAIThreadID(pane Pane, processes map[int]ProcessInfo, children map[int][]int) string {
	return api.PaneAIResume(pane, processes, children).ID
}

// Resume is the agent or c3po screen a pane should restore.
type Resume struct {
	ID        string
	Workspace string
	Kind      string
	Screen    string
}

// PaneAIResume returns the Codex thread id or the Grok session id open in the pane.
// grokWorkspace is the directory Grok stored that session under. grok --resume
// only finds a session from that workspace, so restore must start there.
func (api *API) PaneAIResume(pane Pane, processes map[int]ProcessInfo, children map[int][]int) Resume {
	if processes == nil || children == nil {
		processes, children = api.ProcessTree()
	}
	paneProcs := api.paneProcesses(pane, processes, children)
	codexProcesses := make([]ProcessInfo, 0)
	grokProcesses := make([]ProcessInfo, 0)
	for _, process := range paneProcs {
		if isCodexProcess(process) {
			codexProcesses = append(codexProcesses, process)
		}
		if isGrokProcess(process) {
			grokProcesses = append(grokProcesses, process)
		}
	}
	if len(codexProcesses) > 0 {
		return Resume{ID: api.codexThreadIDFromProcesses(codexProcesses), Kind: "codex"}
	}
	for _, process := range grokProcesses {
		if id, workspace := api.grokResumeForProcess(process, pane.CurrentPath); id != "" {
			return Resume{ID: id, Workspace: workspace, Kind: "grok"}
		}
	}
	for _, process := range paneProcs {
		if id, screen, workspace, kind, ok := c3poResumeForProcess(process, pane.CurrentPath); ok {
			return Resume{ID: id, Workspace: workspace, Kind: kind, Screen: screen}
		}
	}
	return Resume{}
}

func (api *API) codexThreadIDFromProcesses(processes []ProcessInfo) string {
	for _, process := range processes {
		if threadID := rolloutThreadIDFromProcessFDs(process.PID); threadID != "" {
			return threadID
		}
	}
	for _, process := range processes {
		cmd := exec.Command("lsof", "-p", strconv.Itoa(process.PID))
		cmd.Env = api.envList()
		var stdout bytes.Buffer
		cmd.Stdout = &stdout
		if err := cmd.Run(); err != nil {
			var exitErr *exec.ExitError
			if errors.As(err, &exitErr) {
				continue
			}
			return ""
		}
		if threadID := rolloutThreadIDFromOutput(stdout.String()); threadID != "" {
			return threadID
		}
	}
	return ""
}

func (api *API) grokResumeForProcess(process ProcessInfo, paneCWD string) (string, string) {
	if id, workspace, ok := api.grokActiveSession(process.PID); ok {
		if workspace == "" {
			if _, fromFiles := chooseGrokSession(api.grokSessionHitsForProcess(process.PID), paneCWD); fromFiles != "" {
				workspace = fromFiles
			}
		}
		return id, workspace
	}
	if id, workspace := chooseGrokSession(api.grokSessionHitsForProcess(process.PID), paneCWD); id != "" {
		return id, workspace
	}
	if id := grokSessionIDFromArgs(process.Args); id != "" {
		return id, ""
	}
	return "", ""
}

func (api *API) grokSessionHitsForProcess(pid int) []grokSessionHit {
	hits := grokSessionHitsFromOutput(strings.Join(processFDTargets(pid), "\n"))
	if len(hits) > 0 {
		return hits
	}
	return grokSessionHitsFromOutput(api.processLsofOutput(pid))
}

func (api *API) processLsofOutput(pid int) string {
	cmd := exec.Command("lsof", "-p", strconv.Itoa(pid))
	cmd.Env = api.envList()
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	if err := cmd.Run(); err != nil {
		return ""
	}
	return stdout.String()
}

func (api *API) grokActiveSession(pid int) (string, string, bool) {
	home := strings.TrimSpace(api.Env["HOME"])
	if home == "" {
		var err error
		home, err = os.UserHomeDir()
		if err != nil {
			return "", "", false
		}
	}
	if home == "" || pid <= 0 {
		return "", "", false
	}
	data, err := os.ReadFile(filepath.Join(home, ".grok", "active_sessions.json"))
	if err != nil {
		return "", "", false
	}
	var rows []activeGrokSession
	if err := json.Unmarshal(data, &rows); err != nil {
		return "", "", false
	}
	ticks, haveTicks := processStartTicks(pid)
	return matchActiveGrokSession(rows, pid, ticks, haveTicks)
}

func (api *API) ListSessionAgentStatuses() (map[string]AgentStatus, error) {
	panes, err := api.ListPanes("")
	if err != nil {
		return nil, err
	}
	processes, children := api.ProcessTree()
	statuses := map[string]AgentStatus{}
	now := time.Now()
	claudeHolds := readClaudeActivityHolds()
	claudeHoldsDirty := false
	for _, pane := range panes {
		if api.PaneAIProcessPID(pane, processes, children) == 0 {
			continue
		}
		current := statuses[pane.SessionName]
		tail := api.CapturePaneTail(pane.PaneID, 120)
		working := false
		switch {
		case api.paneHasGrokProcess(pane, processes, children):
			working = grokPaneTextShowsWorking(tail) || grokTitleShowsWorking(pane.Title)
		case api.paneHasClaudeProcess(pane, processes, children):
			var until time.Time
			working, until = stabilizeClaudeActivity(claudeTailActivity(tail), now, claudeHoldTime(claudeHolds[pane.PaneID]))
			if until.IsZero() {
				if _, ok := claudeHolds[pane.PaneID]; ok {
					delete(claudeHolds, pane.PaneID)
					claudeHoldsDirty = true
				}
			} else if claudeHolds[pane.PaneID] != until.Unix() {
				claudeHolds[pane.PaneID] = until.Unix()
				claudeHoldsDirty = true
			}
		default:
			working = paneTextShowsWorking(tail)
		}
		current.Total++
		if working {
			current.Working++
		} else {
			current.Idle++
		}
		statuses[pane.SessionName] = current
	}
	if claudeHoldsDirty {
		pruneClaudeActivityHolds(claudeHolds, now)
		writeClaudeActivityHolds(claudeHolds)
	}
	return statuses, nil
}

func (api *API) ListClientTTYs(sessionName string) []string {
	result, err := api.run([]string{"list-clients", "-t", SessionTarget(sessionName), "-F", "#{client_tty}"}, false)
	if err != nil || result.ReturnCode != 0 {
		return nil
	}
	return nonEmptyLines(result.Stdout)
}

func (api *API) ListAllClientTTYs() []string {
	result, err := api.run([]string{"list-clients", "-F", "#{client_tty}"}, false)
	if err != nil || result.ReturnCode != 0 {
		return nil
	}
	return nonEmptyLines(result.Stdout)
}

func (api *API) HasWindow(sessionName, windowName string) bool {
	result, _ := api.run([]string{"display-message", "-p", "-t", sessionName + ":" + windowName, "#{window_id}"}, false)
	return result.ReturnCode == 0
}

func (api *API) SwitchClient(targetSession, clientTTY string) error {
	args := []string{"switch-client"}
	if clientTTY != "" {
		args = append(args, "-c", clientTTY)
	}
	args = append(args, "-t", SessionTarget(targetSession))
	_, err := api.run(args, true)
	return err
}

func (api *API) AttachSession(sessionName string) int {
	result, err := api.run([]string{"attach-session", "-t", SessionTarget(sessionName)}, false)
	if err != nil {
		return 1
	}
	return result.ReturnCode
}

func (api *API) UnbindKey(key, table string, noPrefix bool) error {
	args := []string{"unbind-key", "-q"}
	if noPrefix {
		args = append(args, "-n")
	}
	if table != "" {
		args = append(args, "-T", table)
	}
	args = append(args, key)
	_, err := api.run(args, false)
	return err
}

func (api *API) SourceFile(path string) error {
	_, err := api.run([]string{"source-file", path}, true)
	return err
}

func (api *API) NewSession(sessionName string, detached bool, startDirectory, windowName, command string) int {
	args := []string{"new-session"}
	if detached {
		args = append(args, "-d")
	}
	args = append(args, "-c", coalesce(startDirectory, api.DefaultStartDirectory()), "-s", sessionName)
	if windowName != "" {
		args = append(args, "-n", windowName)
	}
	if command != "" {
		args = append(args, command)
	}
	result, err := api.run(args, false)
	if err != nil {
		return 1
	}
	return result.ReturnCode
}

func (api *API) NewWindow(sessionName, windowName, startDirectory, command string) int {
	args := []string{"new-window", "-d", "-t", SessionTarget(sessionName), "-n", windowName, "-c", coalesce(startDirectory, api.DefaultStartDirectory())}
	if command != "" {
		args = append(args, command)
	}
	result, err := api.run(args, false)
	if err != nil {
		return 1
	}
	return result.ReturnCode
}

func (api *API) SplitWindow(target, startDirectory, command string) int {
	args := []string{"split-window", "-d", "-t", target, "-c", coalesce(startDirectory, api.DefaultStartDirectory())}
	if command != "" {
		args = append(args, command)
	}
	result, err := api.run(args, false)
	if err != nil {
		return 1
	}
	return result.ReturnCode
}

func (api *API) RespawnPane(target, command string) int {
	result, err := api.run([]string{"respawn-pane", "-k", "-t", target, command}, false)
	if err != nil {
		return 1
	}
	return result.ReturnCode
}

func (api *API) SelectLayout(target, layout string) error {
	_, err := api.run([]string{"select-layout", "-t", target, layout}, true)
	return err
}

func (api *API) SelectWindow(target string) error {
	_, err := api.run([]string{"select-window", "-t", target}, true)
	return err
}

func (api *API) SelectPane(target string) error {
	_, err := api.run([]string{"select-pane", "-t", target}, true)
	return err
}

func (api *API) KillSession(sessionName string) error {
	_, err := api.run([]string{"kill-session", "-t", SessionTarget(sessionName)}, true)
	return err
}

func (api *API) KillSessionIfDetached(sessionName string) (bool, error) {
	target := SessionTarget(sessionName)
	command := "kill-session -t " + ShellQuote(target)
	if _, err := api.run([]string{"if-shell", "-F", "-t", target, "#{==:#{session_attached},}", command}, true); err != nil {
		return false, err
	}
	return !api.HasSession(sessionName), nil
}

func (api *API) RenameSession(currentName, newName string) error {
	_, err := api.run([]string{"rename-session", "-t", SessionTarget(currentName), newName}, true)
	return err
}

func (api *API) FallbackSession(targetSession string) (string, bool, error) {
	sessions, err := api.ListSessions(false)
	if err != nil {
		return "", false, err
	}
	for _, session := range sessions {
		if session.Name != targetSession {
			return session.Name, false, nil
		}
	}
	fallback := fmt.Sprintf("__tmp_%d", time.Now().Unix())
	result, err := api.run([]string{"new-session", "-d", "-c", api.DefaultStartDirectory(), "-s", fallback}, true)
	if err != nil || result.ReturnCode != 0 {
		if err == nil {
			err = errTmuxCommandFailed
		}
		return "", false, err
	}
	return fallback, true, nil
}

func EnsureSessionExists(api *API, sessionName string, detached bool) (bool, error) {
	if api.HasSession(sessionName) {
		return false, nil
	}
	if rc := api.NewSession(sessionName, detached, "", "", ""); rc != 0 {
		return false, fmt.Errorf("Unable to create session: %s", sessionName)
	}
	return true, nil
}

func ManagedTmuxConfigPath(api *API) string {
	home := api.DefaultStartDirectory()
	return filepath.Join(home, ".config", "tmux", "tmux.conf")
}

func ManagedIndexKey(configPath string) string {
	paths := []string{configPath}
	fragments, err := filepath.Glob(filepath.Join(filepath.Dir(configPath), "tmux.conf.d", "*.conf"))
	if err == nil {
		sort.Strings(fragments)
		paths = append(paths, fragments...)
	}
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		for _, line := range strings.Split(string(data), "\n") {
			if match := managedIndexBindRE.FindStringSubmatch(strings.TrimSpace(line)); len(match) == 2 {
				return match[1]
			}
		}
	}
	return defaultIndexKey
}

func ReloadManagedConfig(api *API) error {
	configPath := ManagedTmuxConfigPath(api)
	rootKeys := orderedSet(append([]string{ManagedIndexKey(configPath), defaultIndexKey}, managedRootKeys...))
	for _, key := range rootKeys {
		if err := api.UnbindKey(key, "", true); err != nil {
			return err
		}
	}
	for _, key := range managedPrefixKeys {
		if err := api.UnbindKey(key, "", false); err != nil {
			return err
		}
	}
	for _, key := range managedCopyModeVIKeys {
		if err := api.UnbindKey(key, "copy-mode-vi", false); err != nil {
			return err
		}
	}
	return api.SourceFile(configPath)
}

func IndexBrowserCommand() string {
	executable, err := os.Executable()
	if err != nil || executable == "" {
		executable = "tesseract"
	}
	return ShellQuote(executable) + " open browser"
}

func EnsureIndexSession(api *API) (bool, error) {
	browserCommand := IndexBrowserCommand()
	if !api.HasSession(IndexSessionName) {
		if rc := api.NewSession(IndexSessionName, true, "", IndexWindowName, browserCommand); rc != 0 {
			return false, fmt.Errorf("Unable to create session: %s", IndexSessionName)
		}
		return true, nil
	}
	if !api.HasWindow(IndexSessionName, IndexWindowName) {
		if rc := api.NewWindow(IndexSessionName, IndexWindowName, "", browserCommand); rc != 0 {
			return false, fmt.Errorf("Unable to create session: %s", IndexSessionName)
		}
		return true, nil
	}
	panes, err := api.ListPanes(IndexSessionName + ":" + IndexWindowName)
	if err != nil {
		return false, err
	}
	processes, children := api.ProcessTree()
	for _, pane := range panes {
		if api.PaneHasBrowserProcess(pane, processes, children) {
			return false, nil
		}
	}
	targetPane := IndexSessionName + ":" + IndexWindowName + ".0"
	if len(panes) > 0 {
		targetPane = panes[0].PaneID
	}
	if rc := api.RespawnPane(targetPane, browserCommand); rc != 0 {
		return false, fmt.Errorf("Unable to create session: %s", IndexSessionName)
	}
	return true, nil
}

func AttachOrCreateSession(api *API, sessionName string) int {
	if sessionName == IndexSessionName {
		if _, err := EnsureIndexSession(api); err != nil {
			return 1
		}
	}
	insideTmux := strings.TrimSpace(api.Env["TMUX"]) != ""
	clientTTY := api.CurrentClientTTY()
	if api.HasSession(sessionName) {
		if insideTmux {
			if err := api.SwitchClient(sessionName, clientTTY); err != nil {
				return 1
			}
			return 0
		}
		return api.AttachSession(sessionName)
	}
	if insideTmux {
		if rc := api.NewSession(sessionName, true, "", "", ""); rc != 0 {
			return rc
		}
		if err := api.SwitchClient(sessionName, clientTTY); err != nil {
			return 1
		}
		return 0
	}
	return api.NewSession(sessionName, false, "", "", "")
}

func KillSessionSafely(api *API, sessionName string) (string, error) {
	return KillSessionsSafely(api, []string{sessionName})
}

func KillSessionsSafely(api *API, sessionNames []string) (string, error) {
	targets := orderedSet(sessionNames)
	if len(targets) == 0 {
		return "", fmt.Errorf("No sessions selected")
	}
	for _, name := range targets {
		if name == IndexSessionName {
			return "", fmt.Errorf("Cannot kill managed session: %s", IndexSessionName)
		}
	}
	for _, name := range targets {
		if !api.HasSession(name) {
			return "", fmt.Errorf("No such session: %s", name)
		}
	}
	targetSet := map[string]bool{}
	for _, target := range targets {
		targetSet[target] = true
	}
	sessions, err := api.ListSessions(false)
	if err != nil {
		return "", err
	}
	fallback := ""
	for _, session := range sessions {
		if !targetSet[session.Name] {
			fallback = session.Name
			break
		}
	}
	if fallback == "" {
		var err error
		fallback, _, err = api.FallbackSession(targets[0])
		if err != nil {
			return "", err
		}
	}
	for _, name := range targets {
		for _, tty := range api.ListClientTTYs(name) {
			if err := api.SwitchClient(fallback, tty); err != nil {
				return "", err
			}
		}
	}
	for _, name := range targets {
		if err := api.KillSession(name); err != nil {
			return "", err
		}
	}
	return fallback, nil
}

func (result CommandResult) ErrorText() string {
	message := strings.TrimSpace(result.Stderr)
	if message == "" {
		message = strings.TrimSpace(result.Stdout)
	}
	if message == "" {
		message = errTmuxCommandFailed.Error()
	}
	return message
}

func (api *API) run(args []string, check bool) (CommandResult, error) {
	cmd := exec.Command("tmux", args...)
	cmd.Env = api.envList()
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	result := CommandResult{
		Args:   append([]string{}, args...),
		Stdout: stdout.String(),
		Stderr: stderr.String(),
	}
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			result.ReturnCode = exitErr.ExitCode()
		} else {
			result.ReturnCode = 127
			if result.Stderr == "" {
				result.Stderr = err.Error()
			}
		}
	} else {
		result.ReturnCode = 0
	}
	if check && result.ReturnCode != 0 {
		return result, fmt.Errorf("%s", result.ErrorText())
	}
	return result, nil
}

func (api *API) numericOption(args []string, fallback int) int {
	result, err := api.run(args, false)
	if err != nil || result.ReturnCode != 0 {
		return fallback
	}
	value, err := strconv.Atoi(strings.TrimSpace(result.Stdout))
	if err != nil {
		return fallback
	}
	return value
}

func (api *API) processSnapshot() map[int]ProcessInfo {
	cmd := exec.Command("ps", "-eo", "pid=,ppid=,comm=,args=")
	cmd.Env = api.envList()
	out, err := cmd.Output()
	if err != nil {
		return map[int]ProcessInfo{}
	}
	processes := map[int]ProcessInfo{}
	for _, line := range strings.Split(string(out), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 3 {
			continue
		}
		pid, err1 := strconv.Atoi(fields[0])
		ppid, err2 := strconv.Atoi(fields[1])
		if err1 != nil || err2 != nil {
			continue
		}
		args := fields[2]
		if len(fields) > 3 {
			args = strings.Join(fields[3:], " ")
		}
		processes[pid] = ProcessInfo{PID: pid, PPID: ppid, Comm: fields[2], Args: args}
	}
	return processes
}

func (api *API) paneProcesses(pane Pane, processes map[int]ProcessInfo, children map[int][]int) []ProcessInfo {
	if pane.PanePID <= 0 {
		return nil
	}
	stack := []int{pane.PanePID}
	seen := map[int]bool{}
	found := make([]ProcessInfo, 0)
	for len(stack) > 0 {
		pid := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if seen[pid] {
			continue
		}
		seen[pid] = true
		if process, ok := processes[pid]; ok {
			found = append(found, process)
		}
		stack = append(stack, children[pid]...)
	}
	return found
}

func (api *API) envList() []string {
	env := make([]string, 0, len(api.Env))
	for key, value := range api.Env {
		env = append(env, key+"="+value)
	}
	sort.Strings(env)
	return env
}

func isCodexProcess(process ProcessInfo) bool {
	return process.Comm == "codex" || strings.Contains(process.Args, "/bin/codex") || strings.Contains(process.Args, "@openai/codex")
}

func codexProfileFromArgs(args string) string {
	fields := strings.Fields(args)
	for i, field := range fields {
		if field == "--profile" && i+1 < len(fields) {
			return fields[i+1]
		}
		if strings.HasPrefix(field, "--profile=") {
			return strings.TrimPrefix(field, "--profile=")
		}
	}
	return ""
}

func isClaudeProcess(process ProcessInfo) bool {
	if process.Comm == "claude" {
		return true
	}
	args := process.Args
	return strings.Contains(args, "/bin/claude") ||
		strings.Contains(args, "/claude ") ||
		strings.HasSuffix(args, "/claude")
}

func isGrokProcess(process ProcessInfo) bool {
	if process.Comm == "grok" {
		return true
	}
	args := process.Args
	return strings.Contains(args, " grok ") ||
		strings.Contains(args, "/grok ") ||
		strings.Contains(args, "grok-build") ||
		strings.Contains(args, "grok --resume") ||
		strings.Contains(args, ".grok/sessions")
}

var c3poScreens = map[string]bool{
	"cyclotree": true,
	"l":         true,
	"o":         true,
	"tunnel":    true,
	"gdrive":    true,
	"pass":      true,
	"deport":    true,
	"xl":        true,
	"looptab":   true,
}

// c3poActive is one running c3po screen and the place its session holds, as
// c3po __session active prints it. c3po owns screen sessions.
type c3poActive struct {
	SessionID string `json:"session_id"`
	PID       int    `json:"pid"`
	PIDStart  int64  `json:"pid_start"`
	Screen    string `json:"screen"`
	CWD       string `json:"cwd"`
}

var readStartTicks = processStartTicks

func isC3poBinary(process ProcessInfo) bool {
	if process.Comm == "c3po" {
		return true
	}
	fields := strings.Fields(process.Args)
	if len(fields) == 0 {
		return false
	}
	return filepath.Base(fields[0]) == "c3po"
}

func isR2D2Binary(process ProcessInfo) bool {
	if process.Comm == "r2d2" {
		return true
	}
	fields := strings.Fields(process.Args)
	if len(fields) == 0 {
		return false
	}
	return filepath.Base(fields[0]) == "r2d2"
}

func r2d2ScreenCommand(process ProcessInfo) (string, string, bool) {
	fields := strings.Fields(process.Args)
	screen := ""
	resume := ""
	for i := 1; i < len(fields); i++ {
		field := fields[i]
		if screen == "" && c3poScreens[field] {
			screen = field
			continue
		}
		if strings.HasPrefix(field, "--resume=") {
			resume = strings.TrimPrefix(field, "--resume=")
			continue
		}
		if field == "--resume" && i+1 < len(fields) {
			resume = fields[i+1]
			i++
		}
	}
	if screen == "" {
		return "", "", false
	}
	return screen, resume, true
}

func c3poTUICommand(process ProcessInfo) (string, string, bool) {
	if isR2D2Binary(process) {
		return r2d2ScreenCommand(process)
	}
	if !isC3poBinary(process) {
		return "", "", false
	}
	fields := strings.Fields(process.Args)
	screen := ""
	resume := ""
	sawTUI := false
	for i, field := range fields {
		if field == "tui" {
			sawTUI = true
			continue
		}
		if sawTUI && screen == "" && c3poScreens[field] {
			screen = field
		}
		if strings.HasPrefix(field, "--resume=") {
			resume = strings.TrimPrefix(field, "--resume=")
		}
		if field == "--resume" && i+1 < len(fields) {
			resume = fields[i+1]
		}
	}
	if !sawTUI || screen == "" {
		return "", "", false
	}
	return screen, resume, true
}

func c3poResumeForProcess(process ProcessInfo, paneCWD string) (string, string, string, string, bool) {
	screen, resume, ok := c3poTUICommand(process)
	if !ok {
		return "", "", "", "", false
	}
	kind := "c3po"
	if isR2D2Binary(process) {
		kind = "r2d2"
	}
	if row, found := matchC3poActive(readC3poActiveRows(), process.PID); found {
		workspace := row.CWD
		if workspace == "" {
			workspace = paneCWD
		}
		if row.Screen != "" {
			screen = row.Screen
		}
		return row.SessionID, screen, workspace, kind, row.SessionID != ""
	}
	if resume != "" {
		return resume, screen, paneCWD, kind, true
	}
	return "", "", "", "", false
}

func matchC3poActive(rows []c3poActive, pid int) (c3poActive, bool) {
	ticks, known := readStartTicks(pid)
	for _, row := range rows {
		if row.PID != pid || row.SessionID == "" {
			continue
		}
		if known && row.PIDStart != 0 && row.PIDStart != ticks {
			continue
		}
		return row, true
	}
	return c3poActive{}, false
}

func readC3poActiveRows() []c3poActive {
	cmd := exec.Command("c3po", "__session", "active")
	out, err := cmd.Output()
	if err != nil {
		return nil
	}
	return parseC3poActiveRows(out)
}

func parseC3poActiveRows(out []byte) []c3poActive {
	text := strings.TrimSpace(string(out))
	if text == "" {
		return nil
	}
	var rows []c3poActive
	if err := json.Unmarshal([]byte(text), &rows); err != nil {
		return nil
	}
	return rows
}

func isTmBrowserProcess(process ProcessInfo) bool {
	args := process.Args
	for _, name := range []string{"tesseract", "tm"} {
		if strings.Contains(args, " "+name+" open browser") ||
			strings.Contains(args, "/"+name+" open browser") ||
			strings.HasSuffix(args, name+" open browser") {
			return true
		}
	}
	return false
}

func paneTextShowsWorking(text string) bool {
	lines := make([]string, 0)
	for _, line := range strings.Split(text, "\n") {
		trimmed := strings.TrimSpace(strings.ToLower(line))
		if trimmed != "" {
			lines = append(lines, trimmed)
		}
	}
	if len(lines) == 0 {
		return false
	}
	if len(lines) > 30 {
		lines = lines[len(lines)-30:]
	}
	for i := len(lines) - 1; i >= 0; i-- {
		line := lines[i]
		if claudePaneLineShowsWorking(line) {
			return true
		}
		if lineShowsWorkingIndicator(line) {
			if strings.Contains(line, "thinking") {
				return true
			}
			if trailingLinesCanFollowWorking(lines[i+1:]) {
				return true
			}
		}
		if codexActivityHeader(line, stripCodexStatusPrefix(line)) && trailingLinesAreComposer(lines[i+1:]) {
			return true
		}
		if (line == "terminal" || strings.HasPrefix(line, "terminal ")) && i > 0 && strings.Contains(lines[i-1], "waiting for background") {
			return trailingLinesCanFollowWorking(lines[i+1:])
		}
	}
	return false
}

var (
	// A still-running status counts monitors in the single form
	// ("1 monitor still running") and the combined form
	// ("1 command · 2 monitors · 1 loop still running").
	grokMonitorStillRunningRE = regexp.MustCompile(`^[0-9]+ (?:commands?|monitors?|loops?|subagents?)(?: · [0-9]+ (?:commands?|monitors?|loops?|subagents?))* still running(?: · send a message to interrupt)?$`)
	// "▾ Watchers N" stays visible while a monitor is running.
	grokWatcherHeaderRE = regexp.MustCompile(`^[▾▸]\s*watchers\s+[1-9][0-9]*\b`)
	// The watcher row spinner cycles ⋅ : ⸬ ⁙ before the task kind.
	grokMonitorRowRE = regexp.MustCompile(`^[⋅:⸬⁙]\s+monitor\b`)
	// "▾ Tasks N" and a "⋅ Run …" row stay up while a command is running.
	grokTasksHeaderRE = regexp.MustCompile(`^[▾▸]\s*tasks\s+[1-9][0-9]*\b`)
	grokRunRowRE      = regexp.MustCompile(`^[⋅:⸬⁙]\s+run\b`)
	// A foreground status row carries an elapsed time such as "0.1s" or "3m55s".
	grokElapsedRE = regexp.MustCompile(`(?:^|\s)(?:\d+(?:\.\d+)?s|\d+m\d+s)\b`)
)

func grokPaneTextShowsWorking(text string) bool {
	lines := make([]string, 0)
	for _, line := range strings.Split(text, "\n") {
		if trimmed := strings.TrimSpace(line); trimmed != "" {
			lines = append(lines, trimmed)
		}
	}
	if len(lines) == 0 {
		return false
	}
	if len(lines) > 40 {
		lines = lines[len(lines)-40:]
	}
	// "Worked for" and "Recap:" close the turn. Tool rows above them are
	// history, so only the status below the latest marker can be working.
	start := 0
	for i, line := range lines {
		if grokTurnSettledLine(strings.ToLower(line)) {
			start = i + 1
		}
	}
	for _, line := range lines[start:] {
		lower := strings.ToLower(line)
		if grokStatusLineShowsWorking(line, lower) {
			return true
		}
	}
	return false
}

func grokTurnSettledLine(lower string) bool {
	if strings.Contains(lower, "turn completed") {
		return true
	}
	status := grokStatusText(lower)
	if status == "worked for" || strings.HasPrefix(status, "worked for ") {
		return true
	}
	if strings.HasPrefix(status, "compaction completed") {
		return true
	}
	// Compaction prints "Recap:" and an idle return prints "Recap —".
	if status == "recap" || strings.HasPrefix(status, "recap:") || strings.HasPrefix(status, "recap ") {
		return true
	}
	return false
}

func grokStatusLineShowsWorking(line, lower string) bool {
	if strings.Contains(lower, "[stop]") {
		return true
	}
	// Foreground Dispatch, Run, and Monitor rows keep a braille spinner and an
	// elapsed time. The live row often ends in [↓] and omits [stop].
	if grokLineStartsWithBraille(line) && (strings.Contains(line, "[↓]") || grokElapsedRE.MatchString(lower)) {
		return true
	}
	status := grokStatusText(lower)
	if strings.Contains(status, "monitor") && grokMonitorStillRunningRE.MatchString(status) {
		return true
	}
	if grokWatcherHeaderRE.MatchString(status) || grokMonitorRowRE.MatchString(status) ||
		grokTasksHeaderRE.MatchString(status) || grokRunRowRE.MatchString(status) {
		return true
	}
	if grokHasTaskSpinner(line) && (grokTaskKindStatus(status) || strings.HasPrefix(status, "running:")) {
		return true
	}
	// Status rows carry a braille spinner. Transcript prose that merely
	// mentions the phrase does not.
	if grokWaitingStatus(status) && lineContainsBraille(line) {
		return true
	}
	if lineShowsThinkingActivity(lower) || strings.Contains(lower, "esc to interrupt") {
		return true
	}
	if lineContainsBraille(line) {
		if lineShowsThinkingActivity(lower) || strings.Contains(lower, " run ") || strings.HasPrefix(lower, "◆ run") {
			return true
		}
	}
	return false
}

func grokTitleShowsWorking(title string) bool {
	// Active Grok titles start with the braille spinner:
	// "{spinner} - Running: Dispatch|dispatch - {session} - grok".
	// An idle title is "{session} - grok".
	r, ok := grokFirstRune(title)
	return ok && r >= '\u2800' && r <= '\u28FF'
}

func grokFirstRune(text string) (rune, bool) {
	for _, r := range strings.TrimSpace(text) {
		return r, true
	}
	return 0, false
}

func grokLineStartsWithBraille(line string) bool {
	r, ok := grokFirstRune(line)
	return ok && r >= '\u2800' && r <= '\u28FF'
}

func grokHasTaskSpinner(line string) bool {
	if grokLineStartsWithBraille(line) {
		return true
	}
	r, ok := grokFirstRune(line)
	return ok && strings.ContainsRune("⋅:⸬⁙", r)
}

func grokTaskKindStatus(status string) bool {
	status = strings.TrimLeft(status, "⋅:⸬⁙ ")
	for _, kind := range []string{"dispatch", "run", "monitor"} {
		if status == kind || strings.HasPrefix(status, kind+" ") || strings.HasPrefix(status, kind+"|") {
			return true
		}
	}
	return false
}

func isGrokKeybindingHintLine(lower string) bool {
	// Grok footer rows list bindings separated by box-drawing pipes.
	return strings.Contains(lower, "│") && strings.Contains(lower, ":")
}

func grokStatusText(lower string) string {
	status := strings.TrimSpace(lower)
	status = strings.TrimLeft(status, "┃│╭╰╯─ ")
	status = strings.TrimSpace(status)
	for _, prefix := range []string{"◆", "◇", "◎", "◉", "○", "\u2022"} {
		if strings.HasPrefix(status, prefix) {
			status = strings.TrimSpace(strings.TrimPrefix(status, prefix))
		}
	}
	status = strings.TrimLeftFunc(status, func(r rune) bool {
		return r == ' ' || (r >= '\u2800' && r <= '\u28FF')
	})
	return strings.TrimSpace(status)
}

func grokWaitingStatus(status string) bool {
	for _, prefix := range []string{
		"waiting for response",
		"waiting on subagent",
		"waiting on task",
	} {
		if strings.HasPrefix(status, prefix) {
			return true
		}
	}
	return false
}

func lineContainsBraille(line string) bool {
	for _, r := range line {
		if r >= '\u2800' && r <= '\u28FF' {
			return true
		}
	}
	return false
}

func lineIsSpinnerChrome(line string) bool {
	saw := false
	for _, r := range line {
		if r == ' ' || r == '\t' {
			continue
		}
		if r < '\u2800' || r > '\u28FF' {
			return false
		}
		saw = true
	}
	return saw
}

func lineShowsThinkingActivity(lower string) bool {
	if isGrokKeybindingHintLine(lower) {
		return false
	}
	status := grokStatusText(lower)
	return status == "thinking" ||
		status == "thinking..." ||
		status == "thinking…" ||
		strings.HasPrefix(status, "thinking... ") ||
		strings.HasPrefix(status, "thinking… ")
}

func claudePaneLineShowsWorking(line string) bool {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" {
		return false
	}
	lower := strings.ToLower(trimmed)
	// A finished turn uses the same star family as a live wait.
	if strings.Contains(lower, "worked for") {
		return false
	}
	if strings.Contains(lower, "waiting for") && strings.Contains(lower, "background agent") {
		return true
	}
	// A running subagent row: "◯ general-purpose  Searching… 2m 4s · ↓ 85.1k tokens".
	if (strings.HasPrefix(trimmed, "◯") || strings.HasPrefix(trimmed, "○")) &&
		strings.Contains(trimmed, "↓") &&
		(strings.Contains(trimmed, "…") || strings.Contains(trimmed, "...")) {
		return true
	}
	glyph, rest := claudeSpinnerPrefix(trimmed)
	if glyph == "" {
		return false
	}
	if !strings.Contains(rest, "…") && !strings.Contains(rest, "...") {
		return false
	}
	verb, _, _ := strings.Cut(rest, " ")
	verb = strings.TrimRight(verb, "…. ")
	verb = strings.Trim(verb, "'")
	verb = strings.ToLower(verb)
	if len(verb) < 4 {
		return false
	}
	return strings.HasSuffix(verb, "ing") || strings.HasSuffix(verb, "in")
}

const claudeWorkingHold = 9 * time.Second

func claudeTailActivity(text string) string {
	lines := strings.Split(text, "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			continue
		}
		if claudePaneLineShowsWorking(line) {
			return "working"
		}
		if strings.Contains(strings.ToLower(line), "worked for") {
			return "done"
		}
	}
	return "unknown"
}

func stabilizeClaudeActivity(activity string, now, heldUntil time.Time) (bool, time.Time) {
	switch activity {
	case "working":
		return true, now.Add(claudeWorkingHold)
	case "done":
		return false, time.Time{}
	default:
		if !heldUntil.IsZero() && now.Before(heldUntil) {
			return true, heldUntil
		}
		return false, time.Time{}
	}
}

func claudeHoldTime(until int64) time.Time {
	if until <= 0 {
		return time.Time{}
	}
	return time.Unix(until, 0)
}

func claudeActivityPath() string {
	dir := os.Getenv("XDG_RUNTIME_DIR")
	if dir == "" {
		dir = os.TempDir()
	}
	return filepath.Join(dir, "tesseract-claude-activity")
}

func readClaudeActivityHolds() map[string]int64 {
	holds := map[string]int64{}
	data, err := os.ReadFile(claudeActivityPath())
	if err != nil {
		return holds
	}
	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) != 2 {
			continue
		}
		until, err := strconv.ParseInt(fields[1], 10, 64)
		if err != nil {
			continue
		}
		holds[fields[0]] = until
	}
	return holds
}

func pruneClaudeActivityHolds(holds map[string]int64, now time.Time) {
	for id, until := range holds {
		if !now.Before(time.Unix(until, 0)) {
			delete(holds, id)
		}
	}
}

func writeClaudeActivityHolds(holds map[string]int64) {
	var builder strings.Builder
	ids := make([]string, 0, len(holds))
	for id := range holds {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		fmt.Fprintf(&builder, "%s %d\n", id, holds[id])
	}
	path := claudeActivityPath()
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, []byte(builder.String()), 0o600); err != nil {
		return
	}
	_ = os.Rename(tmp, path)
}

func claudeSpinnerPrefix(line string) (string, string) {
	for _, glyph := range []string{"✶", "✻", "✳", "✽", "✱", "✸"} {
		if strings.HasPrefix(line, glyph) {
			return glyph, strings.TrimSpace(strings.TrimPrefix(line, glyph))
		}
	}
	return "", ""
}

func lineShowsWorkingIndicator(line string) bool {
	lower := strings.ToLower(strings.TrimSpace(line))
	if strings.Contains(lower, "esc to int") || strings.Contains(lower, "waiting for background") || lineShowsThinkingActivity(lower) {
		return true
	}
	status := strings.ToLower(stripCodexStatusPrefix(line))
	// Prose ("I am working on...") should not trigger; only the status row.
	// Codex replaces "Working" with a live header such as "Planning ...", "Reviewing ...", or "Drafting ...".
	// Grok uses "Thinking..." and model lines instead of "• Working".
	if strings.Contains(lower, "i am working") || strings.Contains(lower, "i am planning") || strings.Contains(lower, "i am reviewing") || strings.Contains(lower, "i am drafting") {
		return false
	}
	return status == "working" || strings.HasPrefix(status, "working ")
}

func codexActivityHeader(line, status string) bool {
	if !strings.HasPrefix(strings.TrimSpace(line), "•") {
		return false
	}
	header := status
	if i := strings.Index(header, " ("); i > 0 {
		header = strings.TrimSpace(header[:i])
	}
	header = strings.TrimRight(header, ".…")
	word, _, _ := strings.Cut(header, " ")
	word = strings.Trim(word, ".,:;…")
	if len(word) < 5 || !strings.HasSuffix(word, "ing") {
		return false
	}
	for _, r := range word {
		if r < 'a' || r > 'z' {
			return false
		}
	}
	switch word {
	case "something", "anything", "nothing", "everything":
		return false
	default:
		return true
	}
}

func stripCodexStatusPrefix(line string) string {
	status := strings.TrimSpace(line)
	for strings.HasPrefix(status, "\u2022") {
		status = strings.TrimSpace(strings.TrimPrefix(status, "\u2022"))
	}
	return status
}

func lineShowsQueuedFollowUpInputs(line string) bool {
	status := strings.ToLower(stripCodexStatusPrefix(line))
	return strings.Contains(status, "queued follow-up inputs")
}

func trailingLinesAreComposer(lines []string) bool {
	sawComposer := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || lineIsSpinnerChrome(trimmed) {
			continue
		}
		if !lineCanFollowWorkingIndicator(trimmed) {
			return false
		}
		lower := strings.ToLower(trimmed)
		if strings.HasPrefix(trimmed, "›") || strings.HasPrefix(trimmed, ">") ||
			strings.HasPrefix(lower, "gpt-") || strings.HasPrefix(lower, "glm-") ||
			strings.HasPrefix(lower, "grok-") || strings.HasPrefix(lower, "o1") ||
			strings.HasPrefix(lower, "o3") || strings.HasPrefix(lower, "o4") ||
			strings.HasPrefix(lower, "tab to queue message") {
			sawComposer = true
		}
	}
	return sawComposer
}

func trailingLinesCanFollowWorking(lines []string) bool {
	inQueuedFollowUps := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if lineShowsQueuedFollowUpInputs(trimmed) {
			inQueuedFollowUps = true
			continue
		}
		if inQueuedFollowUps {
			continue
		}
		if !lineCanFollowWorkingIndicator(trimmed) {
			return false
		}
	}
	return true
}

func lineCanFollowWorkingIndicator(line string) bool {
	lower := strings.ToLower(strings.TrimSpace(line))
	if lineIsSpinnerChrome(lower) {
		return true
	}
	// The live spinner can sit in the prompt gap: "›⠁Ask" as well as "› Ask".
	return strings.HasPrefix(line, "›") ||
		strings.HasPrefix(line, "\u2514 ") ||
		strings.HasPrefix(line, "> ") ||
		strings.HasPrefix(line, "gpt-") ||
		strings.HasPrefix(line, "o1") ||
		strings.HasPrefix(line, "o3") ||
		strings.HasPrefix(line, "o4") ||
		strings.HasPrefix(lower, "grok-") ||
		strings.HasPrefix(lower, "glm-") ||
		strings.HasPrefix(lower, "tab to queue message") ||
		strings.Contains(lower, "thinking") ||
		strings.HasPrefix(lower, "\u2022") ||
		lineShowsQueuedFollowUpInputs(line) ||
		strings.Contains(lower, "edit last queued message")
}

func tmuxServerMissing(text string) bool {
	lowered := strings.ToLower(text)
	return strings.Contains(lowered, "no server running") ||
		strings.Contains(lowered, "failed to connect to server") ||
		strings.Contains(lowered, "error connecting to")
}

type grokSessionHit struct {
	ID        string
	Workspace string
	Rank      int
	Count     int
}

type activeGrokSession struct {
	SessionID string `json:"session_id"`
	PID       int    `json:"pid"`
	PIDStart  int64  `json:"pid_start"`
	CWD       string `json:"cwd"`
}

func grokSessionHitsFromOutput(output string) []grokSessionHit {
	matches := grokSessionPathRE.FindAllStringSubmatch(output, -1)
	if len(matches) == 0 {
		return nil
	}
	hits := make([]grokSessionHit, 0, len(matches))
	index := map[string]int{}
	for _, match := range matches {
		if len(match) < 3 {
			continue
		}
		id := strings.ToLower(match[2])
		if !uuidRE.MatchString(id) {
			continue
		}
		rank := 1
		if len(match) > 3 && match[3] != "" {
			rank = 3
		}
		workspace := grokWorkspaceFromEncoded(match[1])
		if existing, ok := index[id]; ok {
			hits[existing].Count++
			if rank > hits[existing].Rank {
				hits[existing].Rank = rank
			}
			if hits[existing].Workspace == "" {
				hits[existing].Workspace = workspace
			}
			continue
		}
		index[id] = len(hits)
		hits = append(hits, grokSessionHit{ID: id, Workspace: workspace, Rank: rank, Count: 1})
	}
	return hits
}

func chooseGrokSession(hits []grokSessionHit, paneCWD string) (string, string) {
	paneCWD = cleanWorkspace(paneCWD)
	bestScore := -1
	bestCount := -1
	bestID := ""
	bestWorkspace := ""
	for _, hit := range hits {
		score := hit.Rank
		if paneCWD != "" && hit.Workspace == paneCWD {
			score += 10
		}
		if score > bestScore || (score == bestScore && hit.Count > bestCount) {
			bestScore = score
			bestCount = hit.Count
			bestID = hit.ID
			bestWorkspace = hit.Workspace
		}
	}
	return bestID, bestWorkspace
}

func grokWorkspaceFromEncoded(encoded string) string {
	decoded, err := url.PathUnescape(encoded)
	if err != nil {
		return ""
	}
	return cleanWorkspace(decoded)
}

func cleanWorkspace(path string) string {
	path = strings.TrimSpace(path)
	if path == "" || !strings.HasPrefix(path, "/") {
		return ""
	}
	return filepath.Clean(path)
}

func grokSessionIDFromArgs(args string) string {
	fields := strings.Fields(args)
	for i, field := range fields {
		if field == "--resume" || field == "-r" {
			if i+1 < len(fields) && uuidRE.MatchString(fields[i+1]) {
				return strings.ToLower(fields[i+1])
			}
			continue
		}
		if id, ok := strings.CutPrefix(field, "--resume="); ok && uuidRE.MatchString(id) {
			return strings.ToLower(id)
		}
	}
	return ""
}

func matchActiveGrokSession(rows []activeGrokSession, pid int, ticks int64, haveTicks bool) (string, string, bool) {
	for _, row := range rows {
		if row.PID != pid || !uuidRE.MatchString(row.SessionID) {
			continue
		}
		if row.PIDStart > 0 && haveTicks && row.PIDStart != ticks {
			continue
		}
		return strings.ToLower(row.SessionID), cleanWorkspace(row.CWD), true
	}
	return "", "", false
}

func processStartTicks(pid int) (int64, bool) {
	data, err := os.ReadFile(filepath.Join("/proc", strconv.Itoa(pid), "stat"))
	if err != nil {
		return 0, false
	}
	text := string(data)
	end := strings.LastIndex(text, ")")
	if end < 0 || end+2 >= len(text) {
		return 0, false
	}
	fields := strings.Fields(text[end+2:])
	// starttime is field 22. Field 1 is pid and field 2 is comm, so the index here is 19.
	if len(fields) <= 19 {
		return 0, false
	}
	ticks, err := strconv.ParseInt(fields[19], 10, 64)
	if err != nil {
		return 0, false
	}
	return ticks, true
}

func processFDTargets(pid int) []string {
	if pid <= 0 {
		return nil
	}
	fdDir := filepath.Join("/proc", strconv.Itoa(pid), "fd")
	entries, err := os.ReadDir(fdDir)
	if err != nil {
		return nil
	}
	targets := make([]string, 0, len(entries))
	for _, entry := range entries {
		target, err := os.Readlink(filepath.Join(fdDir, entry.Name()))
		if err != nil {
			continue
		}
		targets = append(targets, target)
	}
	return targets
}

func rolloutThreadIDFromOutput(output string) string {
	for _, line := range strings.Split(output, "\n") {
		if match := rolloutPathRE.FindStringSubmatch(strings.TrimSpace(line)); len(match) == 2 {
			return match[1]
		}
	}
	return ""
}

func rolloutThreadIDFromProcessFDs(pid int) string {
	fdDir := filepath.Join("/proc", strconv.Itoa(pid), "fd")
	entries, err := os.ReadDir(fdDir)
	if err != nil {
		return ""
	}
	for _, entry := range entries {
		target, err := os.Readlink(filepath.Join(fdDir, entry.Name()))
		if err != nil {
			continue
		}
		if threadID := rolloutThreadIDFromOutput(target); threadID != "" {
			return threadID
		}
	}
	return ""
}

func envMap(values []string) map[string]string {
	env := map[string]string{}
	for _, item := range values {
		key, value, ok := strings.Cut(item, "=")
		if ok {
			env[key] = value
		}
	}
	return env
}

func copyEnv(values map[string]string) map[string]string {
	out := map[string]string{}
	for key, value := range values {
		out[key] = value
	}
	return out
}

func pad(fields []string, length int) []string {
	out := append([]string{}, fields...)
	for len(out) < length {
		out = append(out, "")
	}
	return out[:length]
}

func atoi(value string) int {
	parsed, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil {
		return 0
	}
	return parsed
}

func nonEmptyLines(text string) []string {
	lines := make([]string, 0)
	for _, line := range strings.Split(text, "\n") {
		if trimmed := strings.TrimSpace(line); trimmed != "" {
			lines = append(lines, trimmed)
		}
	}
	return lines
}

func coalesce(value, fallback string) string {
	if value != "" {
		return value
	}
	return fallback
}

func orderedSet(values []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(values))
	for _, value := range values {
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	return out
}
