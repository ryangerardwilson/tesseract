package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/ryangerardwilson/tesseract/internal/tmux"
)

const fakeHyprctlScript = `#!/bin/sh
for arg in "$@"; do printf '%s\t' "$arg"; done >> "$HYPRCTL_LOG"
printf '\n' >> "$HYPRCTL_LOG"
case "$1" in
workspaces) printf '%s\n' '[{"id":3,"name":"3"},{"id":-98,"name":"special:musica-hidden"},{"id":1,"name":"1"},{"id":2,"name":"2"}]' ;;
clients) printf '%s\n' '[{"address":"0x3b","mapped":true,"hidden":false,"workspace":{"id":3,"name":"3"},"class":"org.pwmt.zathura","title":"book.pdf","focusHistoryID":2},{"address":"0x1a","mapped":true,"hidden":false,"workspace":{"id":1,"name":"1"},"class":"omarchy-tmux","title":"TMUX","focusHistoryID":3},{"address":"0x3a","mapped":true,"hidden":false,"workspace":{"id":3,"name":"3"},"class":"omarchy-terminal","title":"Terminal","focusHistoryID":0},{"address":"0x2a","mapped":true,"hidden":false,"workspace":{"id":2,"name":"2"},"class":"chromium","title":"Docs","focusHistoryID":1}]' ;;
activeworkspace) printf '%s\n' '{"id":3,"name":"3"}' ;;
dispatch)
  case "$2" in
  *window*) printf '%s\n' "${HYPRCTL_WINDOW_REPLY:-ok}" ;;
  *) printf 'ok\n' ;;
  esac ;;
*) printf 'unknown request\n'; exit 1 ;;
esac
`

// fakeHyprctl puts a logging hyprctl alone on PATH, so these tests can never
// reach the live compositor, and returns a reader for its calls.
func fakeHyprctl(t *testing.T) func() [][]string {
	t.Helper()
	binDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(binDir, "hyprctl"), []byte(fakeHyprctlScript), 0o755); err != nil {
		t.Fatal(err)
	}
	logPath := filepath.Join(t.TempDir(), "hyprctl.log")
	t.Setenv("PATH", binDir)
	t.Setenv("HYPRCTL_LOG", logPath)
	return func() [][]string {
		data, err := os.ReadFile(logPath)
		if os.IsNotExist(err) {
			return nil
		}
		if err != nil {
			t.Fatal(err)
		}
		var calls [][]string
		for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
			calls = append(calls, strings.Split(strings.TrimSuffix(line, "\t"), "\t"))
		}
		return calls
	}
}

func runCLI(t *testing.T, argv ...string) (int, string, string) {
	t.Helper()
	var out bytes.Buffer
	var errOut bytes.Buffer
	code := Run(argv, tmux.NewAPI(map[string]string{"HOME": t.TempDir()}), &out, &errOut)
	return code, out.String(), errOut.String()
}

func TestRunSpaceListPrintsTheSpace(t *testing.T) {
	calls := fakeHyprctl(t)

	code, out, errOut := runCLI(t, "space", "list")

	if code != 0 {
		t.Fatalf("code = %d stderr=%s", code, errOut)
	}
	want := `{"workspaces":[` +
		`{"id":1,"name":"1","layers":[{"address":"0x1a","class":"omarchy-tmux","title":"TMUX"}]},` +
		`{"id":2,"name":"2","layers":[{"address":"0x2a","class":"chromium","title":"Docs"}]},` +
		`{"id":3,"name":"3","layers":[{"address":"0x3a","class":"omarchy-terminal","title":"Terminal"},{"address":"0x3b","class":"org.pwmt.zathura","title":"book.pdf"}]}` +
		`],"active":3}` + "\n"
	if out != want {
		t.Fatalf("stdout:\n%s\nwant:\n%s", out, want)
	}
	wantCalls := [][]string{{"workspaces", "-j"}, {"clients", "-j"}, {"activeworkspace", "-j"}}
	if got := calls(); !reflect.DeepEqual(got, wantCalls) {
		t.Fatalf("space list must only read: calls = %q", got)
	}
}

func TestRunSpaceFocusDispatchesWorkspaceThenWindow(t *testing.T) {
	calls := fakeHyprctl(t)

	code, out, errOut := runCLI(t, "space", "focus", "3", "0x3b")

	if code != 0 {
		t.Fatalf("code = %d stderr=%s", code, errOut)
	}
	if out != `{"workspace":3,"address":"0x3b"}`+"\n" {
		t.Fatalf("stdout = %q", out)
	}
	want := [][]string{
		{"clients", "-j"},
		{"dispatch", `hl.dsp.focus({ workspace = "3" })`},
		{"dispatch", `hl.dsp.focus({ window = "address:0x3b" })`},
	}
	if got := calls(); !reflect.DeepEqual(got, want) {
		t.Fatalf("calls = %q, want %q", got, want)
	}
}

func TestRunSpaceFocusRefusesAStaleLayer(t *testing.T) {
	calls := fakeHyprctl(t)

	code, _, errOut := runCLI(t, "space", "focus", "2", "0x3b")

	if code == 0 || !strings.Contains(errOut, "no layer 0x3b on workspace 2") {
		t.Fatalf("code = %d stderr=%s", code, errOut)
	}
	if got := calls(); !reflect.DeepEqual(got, [][]string{{"clients", "-j"}}) {
		t.Fatalf("a stale layer must not dispatch: calls = %q", got)
	}
}

func TestRunSpaceFocusReportsHyprlandRefusal(t *testing.T) {
	fakeHyprctl(t)
	t.Setenv("HYPRCTL_WINDOW_REPLY", "hl.focus: window not found")

	code, _, errOut := runCLI(t, "space", "focus", "3", "0x3a")

	if code == 0 || !strings.Contains(errOut, "hl.focus: window not found") {
		t.Fatalf("code = %d stderr=%s", code, errOut)
	}
}

func TestRunSpaceFocusRefusesMalformedTargets(t *testing.T) {
	calls := fakeHyprctl(t)

	for _, args := range [][2]string{{"0", "0x3a"}, {"3", "special"}, {"3", `0x3a" })`}} {
		if code, _, _ := runCLI(t, "space", "focus", args[0], args[1]); code == 0 {
			t.Fatalf("space focus %q %q succeeded", args[0], args[1])
		}
	}
	if got := calls(); len(got) != 0 {
		t.Fatalf("a malformed target must not reach hyprctl: calls = %q", got)
	}
}

func TestRunSpaceMoveDispatchesTheWindowMove(t *testing.T) {
	calls := fakeHyprctl(t)

	code, out, errOut := runCLI(t, "space", "move", "1", "0x3b")

	if code != 0 {
		t.Fatalf("code = %d stderr=%s", code, errOut)
	}
	if out != `{"workspace":1,"address":"0x3b"}`+"\n" {
		t.Fatalf("stdout = %q", out)
	}
	want := [][]string{
		{"clients", "-j"},
		{"dispatch", `hl.dsp.window.move({ workspace = "1", window = "address:0x3b" })`},
	}
	if got := calls(); !reflect.DeepEqual(got, want) {
		t.Fatalf("calls = %q, want %q", got, want)
	}
}

func TestRunSpaceMoveRefusesAMissingWindow(t *testing.T) {
	calls := fakeHyprctl(t)

	code, _, errOut := runCLI(t, "space", "move", "1", "0x99")

	if code == 0 || !strings.Contains(errOut, "no layer 0x99") {
		t.Fatalf("code = %d stderr=%s", code, errOut)
	}
	if got := calls(); !reflect.DeepEqual(got, [][]string{{"clients", "-j"}}) {
		t.Fatalf("a missing window must not dispatch: calls = %q", got)
	}
}

func TestRunSpaceMoveRefusesMalformedTargets(t *testing.T) {
	calls := fakeHyprctl(t)

	for _, args := range [][2]string{{"0", "0x3a"}, {"3", "special"}, {"3", `0x3a" })`}} {
		if code, _, _ := runCLI(t, "space", "move", args[0], args[1]); code == 0 {
			t.Fatalf("space move %q %q succeeded", args[0], args[1])
		}
	}
	if got := calls(); len(got) != 0 {
		t.Fatalf("a malformed target must not reach hyprctl: calls = %q", got)
	}
}

func TestRunSpaceRejectsOtherShapes(t *testing.T) {
	fakeHyprctl(t)

	for _, argv := range [][]string{{"space"}, {"space", "focus", "3"}, {"space", "list", "all"}} {
		code, _, errOut := runCLI(t, argv...)
		if code == 0 || !strings.Contains(errOut, "tesseract space focus <workspace-id> <address>") {
			t.Fatalf("%q: code = %d stderr=%s", argv, code, errOut)
		}
	}
}

func TestRunUsageListsTheSpaceForms(t *testing.T) {
	code, _, errOut := runCLI(t, "open", "nowhere")

	if code == 0 {
		t.Fatal("expected an invalid command to fail")
	}
	for _, form := range []string{"tesseract open space", "tesseract space list", "tesseract space focus <workspace-id> <address>", "tesseract space move <workspace-id> <address>", "tesseract open picker"} {
		if !strings.Contains(errOut, form) {
			t.Fatalf("usage missing %q: %s", form, errOut)
		}
	}
}

func TestRunHelpListsTheSpaceCommands(t *testing.T) {
	code, out, _ := runCLI(t, "help")

	if code != 0 || !strings.Contains(out, "tesseract open space") || !strings.Contains(out, "tesseract space list") || !strings.Contains(out, "tesseract space move") {
		t.Fatalf("help missing the space commands:\n%s", out)
	}
}

func TestRunOpenSpaceWithoutTheOverlayOrATerminalFails(t *testing.T) {
	if stdinIsTerminal() && stdoutIsTerminal() {
		t.Skip("the terminal navigator would open on this terminal")
	}
	calls := fakeHyprctl(t)
	t.Setenv("TM_APP_DIR", t.TempDir())

	code, _, errOut := runCLI(t, "open", "space")

	if code == 0 || strings.TrimSpace(errOut) != "tesseract space navigator is not installed" {
		t.Fatalf("code = %d stderr=%s", code, errOut)
	}
	if got := calls(); len(got) != 0 {
		t.Fatalf("a navigator that did not open must not touch hyprctl: calls = %q", got)
	}
}
