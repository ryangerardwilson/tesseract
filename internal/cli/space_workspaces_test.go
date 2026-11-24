package cli

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// closingHyprctlScript is fakeHyprctlScript with one difference: after the
// first clients read it answers from HYPRCTL_CLIENTS_AFTER, so a test can
// say what the closes left behind.
const closingHyprctlScript = `#!/bin/sh
for arg in "$@"; do printf '%s\t' "$arg"; done >> "$HYPRCTL_LOG"
printf '\n' >> "$HYPRCTL_LOG"
case "$1" in
workspaces) printf '%s\n' '[{"id":3,"name":"3"},{"id":-98,"name":"special:musica-hidden"},{"id":1,"name":"1"},{"id":2,"name":"2"}]' ;;
clients)
  if [ -e "$HYPRCTL_LOG.read" ]; then printf '%s\n' "$HYPRCTL_CLIENTS_AFTER"; exit 0; fi
  : > "$HYPRCTL_LOG.read"
  printf '%s\n' '[{"address":"0x3b","mapped":true,"hidden":false,"workspace":{"id":3,"name":"3"},"class":"org.pwmt.zathura","title":"book.pdf","focusHistoryID":2},{"address":"0x1a","mapped":true,"hidden":false,"workspace":{"id":1,"name":"1"},"class":"omarchy-tmux","title":"TMUX","focusHistoryID":3},{"address":"0x3a","mapped":true,"hidden":false,"workspace":{"id":3,"name":"3"},"class":"omarchy-terminal","title":"Terminal","focusHistoryID":0},{"address":"0x2a","mapped":true,"hidden":false,"workspace":{"id":2,"name":"2"},"class":"chromium","title":"Docs","focusHistoryID":1}]' ;;
activeworkspace) printf '%s\n' '{"id":3,"name":"3"}' ;;
dispatch) printf 'ok\n' ;;
*) printf 'unknown request\n'; exit 1 ;;
esac
`

// closingHyprctl puts the closing fake alone on PATH, so these tests never
// reach the live compositor.
func closingHyprctl(t *testing.T, after string) func() [][]string {
	t.Helper()
	calls := fakeHyprctl(t)
	binDir := os.Getenv("PATH")
	if err := os.WriteFile(filepath.Join(binDir, "hyprctl"), []byte(closingHyprctlScript), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HYPRCTL_CLIENTS_AFTER", after)
	return calls
}

func TestRunSpaceNewFocusesPastTheHighest(t *testing.T) {
	calls := fakeHyprctl(t)

	code, out, errOut := runCLI(t, "space", "new")

	if code != 0 {
		t.Fatalf("code = %d stderr=%s", code, errOut)
	}
	if out != `{"workspace":4}`+"\n" {
		t.Fatalf("stdout = %q", out)
	}
	want := [][]string{
		{"workspaces", "-j"},
		{"dispatch", `hl.dsp.focus({ workspace = "4" })`},
	}
	if got := calls(); !reflect.DeepEqual(got, want) {
		t.Fatalf("calls = %q, want %q", got, want)
	}
}

func TestRunSpaceCloseClosesABackgroundWorkspace(t *testing.T) {
	calls := fakeHyprctl(t)

	code, out, errOut := runCLI(t, "space", "close", "2")

	if code != 0 {
		t.Fatalf("code = %d stderr=%s", code, errOut)
	}
	if out != `{"workspace":2,"closed":["0x2a"]}`+"\n" {
		t.Fatalf("stdout = %q", out)
	}
	want := [][]string{
		{"workspaces", "-j"},
		{"clients", "-j"},
		{"activeworkspace", "-j"},
		{"dispatch", `hl.dsp.window.close({ window = "address:0x2a" })`},
	}
	if got := calls(); !reflect.DeepEqual(got, want) {
		t.Fatalf("calls = %q, want %q", got, want)
	}
}

func TestRunSpaceCloseLandsBelowAnEmptiedActiveWorkspace(t *testing.T) {
	calls := closingHyprctl(t, `[{"address":"0x1a","mapped":true,"hidden":false,"workspace":{"id":1,"name":"1"}},{"address":"0x2a","mapped":true,"hidden":false,"workspace":{"id":2,"name":"2"}}]`)

	code, out, errOut := runCLI(t, "space", "close", "3")

	if code != 0 {
		t.Fatalf("code = %d stderr=%s", code, errOut)
	}
	if out != `{"workspace":3,"closed":["0x3a","0x3b"],"focus":2}`+"\n" {
		t.Fatalf("stdout = %q", out)
	}
	var dispatched []string
	for _, call := range calls() {
		if call[0] == "dispatch" {
			dispatched = append(dispatched, call[1])
		}
	}
	want := []string{
		`hl.dsp.window.close({ window = "address:0x3a" })`,
		`hl.dsp.window.close({ window = "address:0x3b" })`,
		`hl.dsp.focus({ workspace = "2" })`,
	}
	if !reflect.DeepEqual(dispatched, want) {
		t.Fatalf("dispatches = %q, want %q", dispatched, want)
	}
}

func TestRunSpaceCloseRefusesWithoutDispatch(t *testing.T) {
	calls := fakeHyprctl(t)

	for _, id := range []string{"0", "special", "9", "-98"} {
		if code, _, _ := runCLI(t, "space", "close", id); code == 0 {
			t.Fatalf("space close %q succeeded", id)
		}
	}
	for _, call := range calls() {
		if call[0] == "dispatch" {
			t.Fatalf("a refused close dispatched %q", call)
		}
	}
}

func TestRunSpaceNewAndCloseRejectOtherShapes(t *testing.T) {
	calls := fakeHyprctl(t)

	for _, argv := range [][]string{{"space", "new", "5"}, {"space", "close"}, {"space", "close", "2", "3"}} {
		code, _, errOut := runCLI(t, argv...)
		if code == 0 || !strings.Contains(errOut, "tesseract space close <workspace-id>") {
			t.Fatalf("%q: code = %d stderr=%s", argv, code, errOut)
		}
	}
	if got := calls(); len(got) != 0 {
		t.Fatalf("a malformed command must not reach hyprctl: calls = %q", got)
	}
}
