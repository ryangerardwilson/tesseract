package tmux

import "testing"

func TestC3poTUICommand(t *testing.T) {
	if _, _, ok := c3poTUICommand(ProcessInfo{Comm: "bash", Args: "bash -lc 'c3po tui o'"}); ok {
		t.Fatal("a shell that mentions c3po is not the screen")
	}
	if _, _, ok := c3poTUICommand(ProcessInfo{Comm: "c3po", Args: "c3po gmail tidy"}); ok {
		t.Fatal("c3po gmail is not a tui")
	}
	screen, resume, ok := c3poTUICommand(ProcessInfo{
		Comm: "c3po",
		Args: "c3po tui o --resume 01a17622-5fd9-7bc2-81ff-3e200a8c16b7",
	})
	if !ok || screen != "o" || resume != "01a17622-5fd9-7bc2-81ff-3e200a8c16b7" {
		t.Fatalf("screen %q resume %q ok %v", screen, resume, ok)
	}
	screen, _, ok = c3poTUICommand(ProcessInfo{Comm: "c3po", Args: "c3po tui looptab"})
	if !ok || screen != "looptab" {
		t.Fatalf("looptab %q %v", screen, ok)
	}
	screen, _, ok = c3poTUICommand(ProcessInfo{Comm: "c3po", Args: "c3po tui nlp"})
	if ok {
		t.Fatalf("nlp is not a c3po screen: %q", screen)
	}
	screen, resume, ok = c3poTUICommand(ProcessInfo{
		Comm: "r2d2",
		Args: "r2d2 o --resume 01a17622-5fd9-7bc2-81ff-3e200a8c16b7",
	})
	if !ok || screen != "o" || resume != "01a17622-5fd9-7bc2-81ff-3e200a8c16b7" {
		t.Fatalf("r2d2 screen %q resume %q ok %v", screen, resume, ok)
	}
	screen, _, ok = c3poTUICommand(ProcessInfo{Comm: "r2d2", Args: "r2d2 looptab"})
	if !ok || screen != "looptab" {
		t.Fatalf("r2d2 looptab %q %v", screen, ok)
	}
	if _, _, ok = c3poTUICommand(ProcessInfo{Comm: "r2d2", Args: "r2d2"}); ok {
		t.Fatal("the menu is not a screen")
	}
	if _, _, ok = c3poTUICommand(ProcessInfo{Comm: "r2d2", Args: "r2d2 web"}); ok {
		t.Fatal("web is not a restored screen")
	}
	if _, _, ok = c3poTUICommand(ProcessInfo{Comm: "r2d2", Args: "r2d2 nlp"}); ok {
		t.Fatal("nlp is not a screen")
	}
}

func TestMatchC3poActive(t *testing.T) {
	orig := readStartTicks
	t.Cleanup(func() { readStartTicks = orig })
	readStartTicks = func(int) (int64, bool) { return 99, true }
	rows := []c3poActive{{
		SessionID: "01a17622-5fd9-7bc2-81ff-3e200a8c16b7",
		PID:       7,
		PIDStart:  99,
		Screen:    "o",
		CWD:       "/work",
	}}
	got, ok := matchC3poActive(rows, 7)
	if !ok || got.Screen != "o" || got.CWD != "/work" {
		t.Fatalf("%+v %v", got, ok)
	}
	rows[0].PIDStart = 1
	if _, ok := matchC3poActive(rows, 7); ok {
		t.Fatal("stale pid start matched")
	}
}

func TestParseC3poActiveRows(t *testing.T) {
	out := []byte(`[{"session_id":"01a17622-5fd9-7bc2-81ff-3e200a8c16b7","pid":7,"pid_start":99,"screen":"o","cwd":"/work"},` +
		`{"session_id":"01a17622-5fd9-7bc2-81ff-3e200a8c16b8","pid":8,"pid_start":0,"screen":"","cwd":""}]`)
	rows := parseC3poActiveRows(out)
	if len(rows) != 2 {
		t.Fatalf("rows %+v", rows)
	}
	want := c3poActive{SessionID: "01a17622-5fd9-7bc2-81ff-3e200a8c16b7", PID: 7, PIDStart: 99, Screen: "o", CWD: "/work"}
	if rows[0] != want || rows[1].PID != 8 || rows[1].Screen != "" {
		t.Fatalf("rows %+v", rows)
	}
	if parseC3poActiveRows([]byte("")) != nil {
		t.Fatal("empty output")
	}
}
