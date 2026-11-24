// Package space reads the Hyprland desktop as two axes. X is the normal
// workspaces in ascending id order. Z is the windows of one workspace,
// stacked front to back by focusHistoryID.
package space

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

const Usage = "valid shape: tesseract space list | tesseract space new | tesseract space focus <workspace-id> <address> | tesseract space move <workspace-id> <address> | tesseract space close <workspace-id>"

// Layer is one window on the Z axis of its workspace.
type Layer struct {
	Address string `json:"address"`
	Class   string `json:"class"`
	Title   string `json:"title"`
}

// Workspace is one place on the X axis and its layers, front first.
type Workspace struct {
	ID     int     `json:"id"`
	Name   string  `json:"name"`
	Layers []Layer `json:"layers"`
}

// Space is the X axis left to right. Active is the active workspace when it
// is on the axis, and 0 otherwise.
type Space struct {
	Workspaces []Workspace `json:"workspaces"`
	Active     int         `json:"active,omitempty"`
}

// Target names one window and a workspace: the layer that confirm focuses, or
// the workspace a move puts the window on.
type Target struct {
	Workspace int    `json:"workspace"`
	Address   string `json:"address"`
}

// Closed is what space close did: the apps it asked to close, and the
// workspace it focused afterwards, if it left the closed one.
type Closed struct {
	Workspace int      `json:"workspace"`
	Closed    []string `json:"closed"`
	Focus     int      `json:"focus,omitempty"`
}

// Runner runs hyprctl with args and returns what it printed on stdout.
type Runner func(args ...string) ([]byte, error)

type workspaceJSON struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

type clientJSON struct {
	Address        string        `json:"address"`
	Mapped         bool          `json:"mapped"`
	Hidden         bool          `json:"hidden"`
	Workspace      workspaceJSON `json:"workspace"`
	Class          string        `json:"class"`
	Title          string        `json:"title"`
	FocusHistoryID int           `json:"focusHistoryID"`
}

// addressRE keeps a target inside the Lua string of a dispatch.
var addressRE = regexp.MustCompile(`^0x[0-9a-f]{1,16}$`)

// waitLayerEnv asks Focus and Move to poll until the navigator layer is gone.
// The Quickshell host starts the command in its own session and then closes.
// Hyprland refuses window focus while that exclusive layer is mapped.
const waitLayerEnv = "TESSERACT_SPACE_WAIT_LAYER"

var (
	hostLayerWait   = 50 * time.Millisecond
	hostLayerGiveUp = 2 * time.Second
)

// A closed active workspace is left only once its windows are gone. Close
// reads the clients every closeSettleWait, for up to closeSettleGiveUp.
var (
	closeSettleWait   = 100 * time.Millisecond
	closeSettleGiveUp = 1500 * time.Millisecond
)

// waitForHostLayer returns immediately unless waitLayerEnv is set. It polls
// hyprctl layers until omarchy-launcher and tesseract-space-navigator are
// gone, or until hostLayerGiveUp. A layers error counts as gone.
func waitForHostLayer(run Runner) {
	if os.Getenv(waitLayerEnv) != "1" {
		return
	}
	deadline := time.Now().Add(hostLayerGiveUp)
	for {
		out, err := run("layers", "-j")
		if err != nil || !hostLayerMapped(out) || !time.Now().Before(deadline) {
			return
		}
		time.Sleep(hostLayerWait)
	}
}

func hostLayerMapped(out []byte) bool {
	text := string(out)
	return strings.Contains(text, "omarchy-launcher") || strings.Contains(text, "tesseract-space-navigator")
}

// Hyprctl runs the hyprctl on PATH.
func Hyprctl(args ...string) ([]byte, error) {
	var stderr bytes.Buffer
	cmd := exec.Command("hyprctl", args...)
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil && stderr.Len() > 0 {
		err = fmt.Errorf("%w: %s", err, strings.TrimSpace(stderr.String()))
	}
	return out, err
}

// List reads Hyprland and builds the space. It only runs read queries.
func List(run Runner) (Space, error) {
	workspaces, err := query(run, "workspaces")
	if err != nil {
		return Space{}, err
	}
	clients, err := query(run, "clients")
	if err != nil {
		return Space{}, err
	}
	active, err := query(run, "activeworkspace")
	if err != nil {
		return Space{}, err
	}
	return Build(workspaces, clients, active)
}

// Build turns the JSON of hyprctl workspaces, clients, and activeworkspace
// into the space.
func Build(workspacesJSON, clientsJSON, activeJSON []byte) (Space, error) {
	var workspaces []workspaceJSON
	if err := decode("workspaces", workspacesJSON, &workspaces); err != nil {
		return Space{}, err
	}
	var clients []clientJSON
	if err := decode("clients", clientsJSON, &clients); err != nil {
		return Space{}, err
	}
	// The active workspace only picks where a navigator starts, so a reply
	// without one leaves Active at 0 instead of failing the list.
	var active workspaceJSON
	_ = json.Unmarshal(activeJSON, &active)
	return build(workspaces, clients, active.ID), nil
}

func build(workspaces []workspaceJSON, clients []clientJSON, activeID int) Space {
	stacks := layersByWorkspace(clients)
	result := Space{Workspaces: []Workspace{}}
	for _, workspace := range normalWorkspaces(workspaces) {
		result.Workspaces = append(result.Workspaces, Workspace{
			ID:     workspace.ID,
			Name:   workspace.Name,
			Layers: append([]Layer{}, stacks[workspace.ID]...),
		})
		if workspace.ID == activeID {
			result.Active = activeID
		}
	}
	return result
}

// normalWorkspaces keeps the positive ids, once each, in ascending order.
func normalWorkspaces(workspaces []workspaceJSON) []workspaceJSON {
	seen := map[int]bool{}
	normal := make([]workspaceJSON, 0, len(workspaces))
	for _, workspace := range workspaces {
		if workspace.ID > 0 && !seen[workspace.ID] {
			seen[workspace.ID] = true
			normal = append(normal, workspace)
		}
	}
	sort.Slice(normal, func(i, j int) bool { return normal[i].ID < normal[j].ID })
	return normal
}

// layersByWorkspace groups the layers by workspace id and orders each
// workspace front to back.
func layersByWorkspace(clients []clientJSON) map[int][]Layer {
	stacks := map[int][]clientJSON{}
	for _, client := range clients {
		if isLayer(client) {
			stacks[client.Workspace.ID] = append(stacks[client.Workspace.ID], client)
		}
	}
	layers := make(map[int][]Layer, len(stacks))
	for id, stack := range stacks {
		sort.SliceStable(stack, func(i, j int) bool { return inFront(stack[i], stack[j]) })
		for _, client := range stack {
			layers[id] = append(layers[id], Layer{Address: client.Address, Class: client.Class, Title: client.Title})
		}
	}
	return layers
}

// isLayer reports whether a window counts on the Z axis: mapped, not hidden,
// and on a normal workspace.
func isLayer(client clientJSON) bool {
	return client.Mapped && !client.Hidden && client.Workspace.ID > 0
}

// inFront reports whether a sits in front of b. focusHistoryID is recency
// across the desktop, so among one workspace's windows the lower value was
// focused more recently. A window outside the history goes to the back.
func inFront(a, b clientJSON) bool {
	switch {
	case a.FocusHistoryID < 0:
		return false
	case b.FocusHistoryID < 0:
		return true
	default:
		return a.FocusHistoryID < b.FocusHistoryID
	}
}

// ParseTarget checks the command-line form of a target.
func ParseTarget(workspace, address string) (Target, error) {
	id, err := strconv.Atoi(workspace)
	if err != nil || id < 1 {
		return Target{}, fmt.Errorf("workspace id must be a positive integer: %q", workspace)
	}
	target := Target{Workspace: id, Address: strings.ToLower(address)}
	if err := target.check(); err != nil {
		return Target{}, err
	}
	return target, nil
}

func (t Target) check() error {
	if t.Workspace < 1 {
		return fmt.Errorf("workspace id must be a positive integer: %d", t.Workspace)
	}
	if !addressRE.MatchString(t.Address) {
		return fmt.Errorf("window address must be 0x and hex digits: %q", t.Address)
	}
	return nil
}

// Focus confirms that the target is still a layer of its workspace, then
// focuses that workspace and then that window. It moves no window.
func Focus(run Runner, target Target) error {
	if err := target.check(); err != nil {
		return err
	}
	out, err := query(run, "clients")
	if err != nil {
		return err
	}
	var clients []clientJSON
	if err := decode("clients", out, &clients); err != nil {
		return err
	}
	if !hasLayer(clients, target) {
		return fmt.Errorf("no layer %s on workspace %d", target.Address, target.Workspace)
	}
	waitForHostLayer(run)
	if err := dispatch(run, fmt.Sprintf(`hl.dsp.focus({ workspace = "%d" })`, target.Workspace)); err != nil {
		return err
	}
	return dispatch(run, fmt.Sprintf(`hl.dsp.focus({ window = "address:%s" })`, target.Address))
}

func hasLayer(clients []clientJSON, target Target) bool {
	for _, client := range clients {
		if isLayer(client) && client.Workspace.ID == target.Workspace && strings.EqualFold(client.Address, target.Address) {
			return true
		}
	}
	return false
}

// Move confirms that the window is still a layer, then puts it on the target
// workspace with one dispatch: the stock window move with the window selector
// that focus uses. A window already on that workspace is left where it is.
func Move(run Runner, target Target) error {
	if err := target.check(); err != nil {
		return err
	}
	out, err := query(run, "clients")
	if err != nil {
		return err
	}
	var clients []clientJSON
	if err := decode("clients", out, &clients); err != nil {
		return err
	}
	from, found := layerWorkspace(clients, target.Address)
	if !found {
		return fmt.Errorf("no layer %s", target.Address)
	}
	if from == target.Workspace {
		return nil
	}
	waitForHostLayer(run)
	return dispatch(run, fmt.Sprintf(`hl.dsp.window.move({ workspace = "%d", window = "address:%s" })`, target.Workspace, target.Address))
}

// New adds a workspace: it focuses the id one past the highest positive
// workspace, which Hyprland creates on focus, and moves no window. Free ids
// below the highest are not reused.
func New(run Runner) (int, error) {
	out, err := query(run, "workspaces")
	if err != nil {
		return 0, err
	}
	var workspaces []workspaceJSON
	if err := decode("workspaces", out, &workspaces); err != nil {
		return 0, err
	}
	id := nextWorkspaceID(workspaces)
	waitForHostLayer(run)
	return id, dispatch(run, focusWorkspace(id))
}

func nextWorkspaceID(workspaces []workspaceJSON) int {
	highest := 0
	for _, workspace := range workspaces {
		highest = max(highest, workspace.ID)
	}
	return highest + 1
}

func focusWorkspace(id int) string {
	return fmt.Sprintf(`hl.dsp.focus({ workspace = "%d" })`, id)
}

// ParseWorkspace checks the command-line form of a workspace id.
func ParseWorkspace(workspace string) (int, error) {
	id, err := strconv.Atoi(workspace)
	if err != nil || id < 1 {
		return 0, fmt.Errorf("workspace id must be a positive integer: %q", workspace)
	}
	return id, nil
}

// Close asks every app of one workspace to close: one graceful close each,
// front first, and never a kill. It lists the space afresh, refuses a
// workspace off the X axis and the only one on it, and waits for the host
// layer first, so a save dialog an app raises can take the keyboard. When the
// workspace was the active one and no window is left on it, it focuses the
// nearest other workspace, the next lower id or else the next higher, so
// Hyprland drops the empty one. A window that stays keeps the workspace.
func Close(run Runner, workspace int) (Closed, error) {
	result := Closed{Workspace: workspace, Closed: []string{}}
	current, err := List(run)
	if err != nil {
		return result, err
	}
	apps, nearest, err := closable(current, workspace)
	if err != nil {
		return result, err
	}
	waitForHostLayer(run)
	var failed []error
	for _, app := range apps {
		if err := closeApp(run, app.Address); err != nil {
			failed = append(failed, err)
			continue
		}
		result.Closed = append(result.Closed, app.Address)
	}
	if current.Active == workspace && emptied(run, workspace) {
		if err := dispatch(run, focusWorkspace(nearest)); err != nil {
			failed = append(failed, err)
		} else {
			result.Focus = nearest
		}
	}
	return result, errors.Join(failed...)
}

// closable is the apps of workspace, front first, and the nearest other
// workspace on the X axis: the next lower id, or else the next higher.
func closable(current Space, workspace int) ([]Layer, int, error) {
	for index, candidate := range current.Workspaces {
		if candidate.ID != workspace {
			continue
		}
		if len(current.Workspaces) < 2 {
			return nil, 0, fmt.Errorf("workspace %d is the only workspace", workspace)
		}
		if index > 0 {
			return candidate.Layers, current.Workspaces[index-1].ID, nil
		}
		return candidate.Layers, current.Workspaces[index+1].ID, nil
	}
	return nil, 0, fmt.Errorf("no workspace %d", workspace)
}

func closeApp(run Runner, address string) error {
	if !addressRE.MatchString(strings.ToLower(address)) {
		return fmt.Errorf("window address must be 0x and hex digits: %q", address)
	}
	return dispatch(run, fmt.Sprintf(`hl.dsp.window.close({ window = "address:%s" })`, address))
}

// emptied waits for the closes to land. It reads the clients until no mapped
// window is left on workspace, or until closeSettleGiveUp, and reports
// whether the workspace came empty.
func emptied(run Runner, workspace int) bool {
	deadline := time.Now().Add(closeSettleGiveUp)
	for {
		out, err := run("clients", "-j")
		if err == nil && !holdsWindow(out, workspace) {
			return true
		}
		if !time.Now().Before(deadline) {
			return false
		}
		time.Sleep(closeSettleWait)
	}
}

// holdsWindow reports whether a mapped window is on workspace. A reply that
// is not JSON counts as holding one, so focus never leaves on a guess.
func holdsWindow(out []byte, workspace int) bool {
	var clients []clientJSON
	if json.Unmarshal(out, &clients) != nil {
		return true
	}
	for _, client := range clients {
		if client.Mapped && client.Workspace.ID == workspace {
			return true
		}
	}
	return false
}

// layerWorkspace is the workspace that holds the window at address as a
// layer.
func layerWorkspace(clients []clientJSON, address string) (int, bool) {
	for _, client := range clients {
		if isLayer(client) && strings.EqualFold(client.Address, address) {
			return client.Workspace.ID, true
		}
	}
	return 0, false
}

// dispatch runs one Lua dispatch. It succeeds only when hyprctl exits 0 and
// replies ok, or prints nothing.
func dispatch(run Runner, expression string) error {
	out, err := run("dispatch", expression)
	reply := strings.TrimSpace(string(out))
	if err != nil {
		if reply != "" {
			return fmt.Errorf("hyprctl dispatch %s: %s", expression, reply)
		}
		return fmt.Errorf("hyprctl dispatch %s: %w", expression, err)
	}
	if reply != "" && !strings.EqualFold(reply, "ok") {
		return fmt.Errorf("hyprctl dispatch %s: %s", expression, reply)
	}
	return nil
}

func query(run Runner, what string) ([]byte, error) {
	out, err := run(what, "-j")
	if err != nil {
		return nil, fmt.Errorf("hyprctl %s -j: %w", what, err)
	}
	return out, nil
}

func decode(what string, data []byte, into any) error {
	if err := json.Unmarshal(data, into); err != nil {
		return fmt.Errorf("hyprctl %s -j: %w (reply %q)", what, err, excerpt(data))
	}
	return nil
}

func excerpt(data []byte) string {
	text := strings.TrimSpace(string(data))
	if line, _, found := strings.Cut(text, "\n"); found {
		text = line
	}
	if len(text) > 120 {
		text = text[:120]
	}
	return text
}
