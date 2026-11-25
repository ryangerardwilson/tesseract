package app

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/ryangerardwilson/tesseract/internal/musica/config"
	"github.com/ryangerardwilson/tesseract/internal/musica/library"
	"github.com/ryangerardwilson/tesseract/internal/musica/store"
	"github.com/ryangerardwilson/tesseract/internal/musica/visualizer"
)

type fakeWatchers struct {
	watching bool
	lines    []string
}

func (w *fakeWatchers) Publish(line []byte) { w.lines = append(w.lines, string(line)) }
func (w *fakeWatchers) Watching() bool      { return w.watching }

func TestKeyMsgReadsWatcherKeysAsTheTUIDoes(t *testing.T) {
	for _, name := range []string{"enter", "esc", "backspace", "ctrl+h", "ctrl+w", "ctrl+c", "ctrl+j", "ctrl+k", "up", "down", "left", "right", "j", "k", "m", "q", "?", "/", ",", "N", " "} {
		line, _ := json.Marshal(map[string]string{"key": name})
		msg, ok := keyMsg(string(line))
		if !ok || msg.String() != name {
			t.Fatalf("key %q read as %q, %v", name, msg.String(), ok)
		}
	}
	for _, line := range []string{"", "j", `{"key":""}`, `{"key":"ab"}`, `{"key":"\u0001"}`, `{"key":"ctrl+z"}`} {
		if _, ok := keyMsg(line); ok {
			t.Fatalf("line %q read as a key", line)
		}
	}
}

func TestServedPublishesFramesOnlyWhileWatched(t *testing.T) {
	out := &fakeWatchers{}
	s := served{model: model{playlists: []library.Playlist{{Name: "a", Path: "/m/a"}, {Name: "b", Path: "/m/b"}}}, out: out}
	next, _ := s.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	if len(out.lines) != 0 {
		t.Fatalf("published %d frames with no watcher", len(out.lines))
	}
	out.watching = true
	next, _ = next.(served).Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	if len(out.lines) != 1 {
		t.Fatalf("published %d frames, want 1", len(out.lines))
	}
	var got frame
	if err := json.Unmarshal([]byte(out.lines[0]), &got); err != nil {
		t.Fatal(err)
	}
	if got.Type != "frame" || got.Selected != 0 || got.Count != 2 {
		t.Fatalf("frame = %+v, want the selection wrapped to the first of two", got)
	}
	if next.(served).View() != "" {
		t.Fatal("a served model drew a view")
	}
}

func leaveCounter() (*int, func()) {
	count := 0
	return &count, func() { count++ }
}

func run(cmd tea.Cmd) {
	if cmd != nil {
		cmd()
	}
}

func TestServedMTogglesTheMenuAndStays(t *testing.T) {
	player := &fakeAudioPlayer{path: "/music/track.mp3", playing: true}
	left, leave := leaveCounter()
	m := model{player: player, showMenu: false, leave: leave, currentPath: "/music/track.mp3"}
	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'m'}})
	run(cmd)
	if *left != 0 {
		t.Fatalf("m left %d times, want 0", *left)
	}
	if !next.(model).showMenu {
		t.Fatal("m did not open the menu")
	}
	if !player.IsPlaying() {
		t.Fatal("m stopped playback")
	}
	again, cmd := next.(model).Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'m'}})
	run(cmd)
	if again.(model).showMenu || *left != 0 {
		t.Fatal("the second m did not close the menu")
	}
}

func TestTerminalMStillTogglesTheMenu(t *testing.T) {
	next, cmd := model{showMenu: true}.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'m'}})
	if cmd != nil || next.(model).showMenu {
		t.Fatal("m in the TUI did not close the menu")
	}
}

func TestServedMTypesIntoAPrompt(t *testing.T) {
	left, leave := leaveCounter()
	m := model{inputKind: inputSearch, leave: leave}
	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'m'}})
	run(cmd)
	if *left != 0 || next.(model).input != "m" {
		t.Fatalf("m in a prompt: left %d, input %q", *left, next.(model).input)
	}
}

// openCassette is a served model with cassette b of three open in the menu
// and its second track selected.
func openCassette(leave func()) model {
	return model{
		view:                 tracksView,
		showMenu:             true,
		leave:                leave,
		playlists:            []library.Playlist{{Name: "a", Path: "/m/a"}, {Name: "b", Path: "/m/b"}, {Name: "c", Path: "/m/c"}},
		cassetteSelected:     1,
		cassetteSelectedPath: "/m/b",
		currentPlaylist:      "/m/b",
		currentPlaylistName:  "b",
		tracks:               []library.Track{{Name: "one", Path: "/m/b/one.mp3"}, {Name: "two", Path: "/m/b/two.mp3"}},
		selected:             1,
	}
}

func TestServedEscapeLeavesFromAnywhereAndBacksOutOfNothing(t *testing.T) {
	left, leave := leaveCounter()
	helped := openCassette(leave)
	helped.showHelp = true
	for name, m := range map[string]model{
		"a cassette's tracks": openCassette(leave),
		"the cassette list":   {showMenu: true, leave: leave, playlists: openCassette(leave).playlists},
		"help":                helped,
		"the download view":   {showDownloads: true, leave: leave},
		"a prompt":            {inputKind: inputSearch, input: "a", leave: leave},
	} {
		before := *left
		next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
		run(cmd)
		got := next.(model)
		if *left != before+1 {
			t.Fatalf("esc on %s left %d times, want 1", name, *left-before)
		}
		if got.view != m.view || got.selected != m.selected || got.showMenu != m.showMenu || got.showHelp != m.showHelp || got.showDownloads != m.showDownloads || got.inputKind != m.inputKind || got.input != m.input {
			t.Fatalf("esc on %s backed out: view %v selected %d menu %v help %v downloads %v prompt %q %q", name, got.view, got.selected, got.showMenu, got.showHelp, got.showDownloads, got.inputKind, got.input)
		}
	}
}

func TestServedHGoesBackToTheCassetteList(t *testing.T) {
	left, leave := leaveCounter()
	helped := openCassette(leave)
	helped.showHelp = true
	listing := openCassette(leave)
	listing.showDownloads = true
	armed := openCassette(leave)
	armed.deleteArmed = true
	for name, m := range map[string]model{
		"a cassette's tracks":          openCassette(leave),
		"help over a cassette":         helped,
		"the download view":            listing,
		"a cassette with delete armed": armed,
		"help over the picture":        {showHelp: true, leave: leave, playlists: openCassette(leave).playlists},
		"downloads over the picture":   {showDownloads: true, leave: leave, playlists: openCassette(leave).playlists},
	} {
		next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'h'}})
		run(cmd)
		got := next.(model)
		if *left != 0 {
			t.Fatalf("h from %s left the center", name)
		}
		if got.view != rootView || !got.showMenu || got.showHelp || got.showDownloads || got.deleteArmed {
			t.Fatalf("h from %s: view %v menu %v help %v downloads %v armed %v", name, got.view, got.showMenu, got.showHelp, got.showDownloads, got.deleteArmed)
		}
		if f := got.frame(); f.View != "cassettes" || !f.Menu || f.Help || f.Downloads.Shown {
			t.Fatalf("h from %s: frame %+v", name, f)
		}
		if m.view == tracksView && got.playlists[got.selected].Name != "b" {
			t.Fatalf("h from %s selected %d, want the cassette that was open", name, got.selected)
		}
	}
}

func TestServedHAndLStillMoveOnTheCassetteList(t *testing.T) {
	left, leave := leaveCounter()
	m := model{showMenu: true, leave: leave, playlists: openCassette(leave).playlists, selected: 1}
	for _, step := range []struct {
		key  rune
		want int
	}{{'h', 0}, {'h', 2}, {'l', 0}, {'l', 1}} {
		next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{step.key}})
		run(cmd)
		m = next.(model)
		if m.selected != step.want || m.view != rootView || *left != 0 {
			t.Fatalf("%c on the cassette list: selected %d view %v left %d, want %d", step.key, m.selected, m.view, *left, step.want)
		}
	}

	// Over the picture, h opens nothing.
	playing := openCassette(leave)
	playing.showMenu = false
	next, _ := playing.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'h'}})
	if got := next.(model); got.showMenu || got.view != tracksView {
		t.Fatalf("h over the picture: menu %v view %v", got.showMenu, got.view)
	}
}

func TestTerminalEscapeStillBacksOutAndHStillMoves(t *testing.T) {
	m := openCassette(nil)
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'h'}})
	if got := next.(model); got.view != tracksView || got.selected != 0 {
		t.Fatalf("h in the TUI: view %v selected %d, want the track above", got.view, got.selected)
	}
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if got := next.(model); got.view != rootView || got.playlists[got.selected].Name != "b" {
		t.Fatalf("esc in the TUI: view %v selected %d", got.view, got.selected)
	}
}

func TestServedQDoesNotEndTheSession(t *testing.T) {
	player := &fakeAudioPlayer{path: "/music/track.mp3", playing: true}
	left, leave := leaveCounter()
	next, cmd := model{player: player, leave: leave, currentPath: "/music/track.mp3"}.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	if cmd != nil || !player.IsPlaying() || *left != 0 || next.(model).quitting {
		t.Fatalf("q in serve: cmd %v playing %v left %d quitting %v", cmd != nil, player.IsPlaying(), *left, next.(model).quitting)
	}
}

func TestFrameCarriesTheListAroundTheSelection(t *testing.T) {
	playlists := make([]library.Playlist, 12)
	for i := range playlists {
		playlists[i] = library.Playlist{Name: fmt.Sprintf("c%02d", i), Path: fmt.Sprintf("/m/c%02d", i)}
	}
	m := model{playlists: playlists, selected: 11, showMenu: true}
	f := m.frame()
	if f.View != "cassettes" || f.Count != 12 || f.Selected != 11 || !f.Menu {
		t.Fatalf("frame = %+v", f)
	}
	if len(f.Rows) != frameRows || f.Rows[0].Index != 3 || f.Rows[len(f.Rows)-1].Index != 11 {
		t.Fatalf("rows = %+v, want indexes 3 to 11", f.Rows)
	}
	if f.Rows[len(f.Rows)-1].Slot != "B02" || f.Rows[0].Kind != "CASSETTE" {
		t.Fatalf("row = %+v", f.Rows[len(f.Rows)-1])
	}
	if len(f.Levels) != frameBands || len(f.Peaks) != frameBands || f.Bass != 0 || f.Energy != 0 {
		t.Fatalf("levels %d peaks %d bass %v energy %v", len(f.Levels), len(f.Peaks), f.Bass, f.Energy)
	}
	if f.Prompt != nil || f.State != "idle" {
		t.Fatalf("prompt %+v state %q", f.Prompt, f.State)
	}
}

func TestFrameNamesTheCassetteTrackAndPlayingState(t *testing.T) {
	player := &fakeAudioPlayer{path: "/m/a/Blue Monday.mp3", playing: true, paused: true}
	m := model{
		view:                tracksView,
		player:              player,
		currentPath:         "/m/a/Blue Monday.mp3",
		currentPlaylistName: "a",
		tracks:              []library.Track{{Name: "Blue Monday", Path: "/m/a/Blue Monday.mp3"}, {Name: "Age", Path: "/m/a/Age.mp3"}},
		inputKind:           inputYouTubeName,
		input:               "Ceremony",
		youtubeURL:          "https://youtu.be/x",
		status:              "song name",
		showDownloads:       true,
		downloads:           []store.Job{{ID: "1", Title: "done", Status: "done"}, {ID: "2", Title: "Age", Status: "running", Percent: 41.26}},
	}
	f := m.frame()
	if f.View != "tracks" || f.Cassette != "a" || f.State != "paused" || f.Title != "Blue Monday" || f.Elapsed == "" {
		t.Fatalf("frame = %+v", f)
	}
	if !f.Rows[0].Live || f.Rows[1].Live || f.Rows[0].Kind != "TRACK" {
		t.Fatalf("rows = %+v", f.Rows)
	}
	if f.Prompt == nil || f.Prompt.Label != "SONG NAME" || f.Prompt.Text != "Ceremony" || f.Prompt.URL != "https://youtu.be/x" {
		t.Fatalf("prompt = %+v", f.Prompt)
	}
	if !f.Downloads.Shown || len(f.Downloads.Rows) != 1 || f.Downloads.Rows[0].Percent != 41.3 {
		t.Fatalf("downloads = %+v", f.Downloads)
	}

	m.audioPending = "starting…"
	if f := m.frame(); f.State != "starting…" {
		t.Fatalf("state while starting = %q", f.State)
	}
	if line := string(m.frameLine()); !strings.HasPrefix(line, `{"type":"frame"`) || strings.Contains(line, "\n") {
		t.Fatalf("frame line = %q", line)
	}
}

func TestServedFrameCarriesAKickOnItsFrameOnly(t *testing.T) {
	analyzer := visualizer.New(32)
	out := &fakeWatchers{watching: true}
	var s tea.Model = served{model: model{cfg: config.AppConfig{FPS: 20}, visualizer: analyzer}, out: out}
	frameAfter := func(samples []int16) float64 {
		t.Helper()
		analyzer.Feed(samples)
		s, _ = s.Update(tickMsg(time.Now()))
		var got frame
		if err := json.Unmarshal([]byte(out.lines[len(out.lines)-1]), &got); err != nil {
			t.Fatal(err)
		}
		return got.Thump
	}

	if got := frameAfter(make([]int16, 4*analysisWindow)); got > 0.05 {
		t.Fatalf("thump over silence = %v, want near 0", got)
	}
	if got := frameAfter(kickWindow()); got < 0.9 {
		t.Fatalf("thump on the kick's frame = %v, want near 1", got)
	}
	for i := 1; i <= 3; i++ {
		if got := frameAfter(make([]int16, analysisWindow)); got > 0.05 {
			t.Fatalf("thump %d quiet windows after the kick = %v, want near 0", i, got)
		}
	}

	// A kick heard while no one watches is not sent late.
	out.watching = false
	analyzer.Feed(kickWindow())
	s, _ = s.Update(tickMsg(time.Now()))
	out.watching = true
	if got := frameAfter(make([]int16, analysisWindow)); got > 0.05 {
		t.Fatalf("thump after a kick no one watched = %v, want near 0", got)
	}
}

// analysisWindow is how many samples the visualizer hears as one window.
const analysisWindow = 1024

// kickWindow is one analysis window, at the visualizer's 11025 Hz, holding a
// kick drum: a low tone that falls from about 140 Hz to 50 Hz as it dies
// away.
func kickWindow() []int16 {
	out := make([]int16, analysisWindow)
	for i := range out {
		t := float64(i) / 11025
		out[i] = int16(26000 * math.Sin(2*math.Pi*(50*t+3*(1-math.Exp(-30*t)))) * math.Exp(-18*t))
	}
	return out
}

func TestFrameLevelsStayBetweenZeroAndOne(t *testing.T) {
	values := bandValues([]float64{-0.5, 0.12345, 1.7})
	if len(values) != frameBands || values[0] != 0 || values[1] != 0.123 || values[2] != 1 || values[31] != 0 {
		t.Fatalf("bandValues = %v", values)
	}
}

// heldModel is a served model over a library with a focus cassette, holding
// focus/Flow.mp3 as musica serve does after musica end ended a session that
// had it loaded.
func heldModel(t *testing.T, player audioPlayer) (model, string) {
	t.Helper()
	root := t.TempDir()
	for _, name := range []string{"christ/Psalm.mp3", "focus/4 Seasons.mp3", "focus/Experience.mp3", "focus/Flow.mp3", "focus/Interstellar.mp3"} {
		path := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	_, leave := leaveCounter()
	m := model{cfg: config.AppConfig{MusicDir: root, FPS: 20}, player: player, visualizer: &visualizer.Analyzer{}, leave: leave, status: "ready"}
	m.reloadRoot()
	flow := filepath.Join(root, "focus", "Flow.mp3")
	m.hold(flow)
	return m, flow
}

func TestServeHoldsTheEndedTrackPausedWithoutStartingIt(t *testing.T) {
	player := &recordingPlayer{}
	m, _ := heldModel(t, player)

	f := m.frame()
	if f.State != "paused" || f.Title != "Flow" || f.Elapsed != "" || f.View != "tracks" || f.Cassette != "focus" || !f.Menu {
		t.Fatalf("frame = %+v", f)
	}
	if f.Selected != 2 || f.Rows[2].Name != "Flow" || f.Rows[2].Live {
		t.Fatalf("selected %d rows %+v, want Flow selected and not live", f.Selected, f.Rows)
	}
	for range 5 {
		next, _ := m.Update(tickMsg(time.Now()))
		m = next.(model)
	}
	if len(player.calls) != 0 || m.audioBusy {
		t.Fatalf("holding reached ffplay: calls %v busy %v", player.calls, m.audioBusy)
	}

	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{' '}})
	spaced := next.(model)
	if cmd != nil || len(player.calls) != 0 || spaced.audioBusy {
		t.Fatalf("space started the held track: calls %v", player.calls)
	}
	if spaced.status != "nothing to resume; enter plays from the start" || spaced.frame().State != "paused" {
		t.Fatalf("space: status %q state %q", spaced.status, spaced.frame().State)
	}
}

func TestEnterPlaysAHeldTrackFromTheStart(t *testing.T) {
	player := &recordingPlayer{}
	m, flow := heldModel(t, player)

	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	started := land(t, next.(model), cmd)
	if len(player.calls) != 1 || player.calls[0] != "play "+flow || player.elapsed != 0 {
		t.Fatalf("enter: calls %v at %v", player.calls, player.elapsed)
	}
	if started.held != "" || started.frame().State != "playing" {
		t.Fatalf("enter left the track held: held %q state %q", started.held, started.frame().State)
	}
}

func TestStopPutsAHeldTrackDown(t *testing.T) {
	m, _ := heldModel(t, &recordingPlayer{})

	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}})
	stopped := land(t, next.(model), cmd)
	if f := stopped.frame(); f.State != "idle" || f.Title != "" || stopped.status != "stopped" {
		t.Fatalf("x: frame %+v status %q", f, stopped.status)
	}
}

func TestHFromAHeldCassetteSelectsItOnTheList(t *testing.T) {
	m, _ := heldModel(t, &recordingPlayer{})

	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'h'}})
	back := next.(model)
	if back.view != rootView || back.playlists[back.selected].Name != "focus" {
		t.Fatalf("h: view %v selected %d", back.view, back.selected)
	}
	if f := back.frame(); f.State != "paused" || f.Title != "Flow" {
		t.Fatalf("h put the held track down: %+v", f)
	}
}

func TestHoldSkipsAMissingTrack(t *testing.T) {
	m, _ := heldModel(t, &recordingPlayer{})
	fresh := model{cfg: m.cfg, player: &recordingPlayer{}}
	fresh.reloadRoot()
	fresh.hold(filepath.Join(m.cfg.MusicDir, "focus", "Gone.mp3"))
	fresh.hold("")
	if fresh.view != rootView || fresh.held != "" || fresh.frame().State != "idle" {
		t.Fatalf("hold of a missing track: view %v held %q", fresh.view, fresh.held)
	}
}

func TestEventLines(t *testing.T) {
	if string(eventLine("leave")) != `{"type":"leave"}` || string(eventLine("ended")) != `{"type":"ended"}` {
		t.Fatalf("%s %s", eventLine("leave"), eventLine("ended"))
	}
}
