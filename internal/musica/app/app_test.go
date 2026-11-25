package app

import (
	"fmt"
	"math"
	"regexp"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/ryangerardwilson/tesseract/internal/musica/config"
	"github.com/ryangerardwilson/tesseract/internal/musica/library"
	"github.com/ryangerardwilson/tesseract/internal/musica/player"
	"github.com/ryangerardwilson/tesseract/internal/musica/store"
	"github.com/ryangerardwilson/tesseract/internal/musica/visualizer"
)

var ansiPattern = regexp.MustCompile("\x1b\\[[0-9;]*m")

type fakeAudioPlayer struct {
	path    string
	elapsed time.Duration
	paused  bool
	playing bool
	playErr error
}

func (p *fakeAudioPlayer) Play(path string, offset time.Duration) error {
	if p.playErr != nil {
		return p.playErr
	}
	p.path = path
	p.elapsed = offset
	p.paused = false
	p.playing = true
	return nil
}

func (p *fakeAudioPlayer) TogglePause() bool {
	if p.path == "" && !p.playing {
		return false
	}
	p.paused = !p.paused
	return p.paused
}

func (p *fakeAudioPlayer) Stop() {
	p.path = ""
	p.elapsed = 0
	p.paused = false
	p.playing = false
}

func (p *fakeAudioPlayer) State() (string, time.Duration) {
	return p.path, p.elapsed
}

func (p *fakeAudioPlayer) IsPlaying() bool {
	return p.playing
}

func (p *fakeAudioPlayer) IsPaused() bool {
	return p.paused
}

func TestVisualizerHeightCanUseMoreVerticalSpace(t *testing.T) {
	m := model{
		width:      80,
		height:     24,
		visualizer: visualizer.New(8),
	}
	if !m.visualizer.Available() {
		t.Skip("ffmpeg not available")
	}

	if got := m.visualizerHeightFor(true); got != m.height-visualizerChromeLines() {
		t.Fatalf("visualizerHeightFor(true) = %d, want remaining viewport %d", got, m.height-visualizerChromeLines())
	}
}

func TestVisualizerHeightUsesRemainingViewport(t *testing.T) {
	m := model{
		width:      80,
		height:     18,
		visualizer: visualizer.New(8),
	}
	if !m.visualizer.Available() {
		t.Skip("ffmpeg not available")
	}

	if got := m.visualizerHeightFor(true); got != m.height-visualizerChromeLines() {
		t.Fatalf("visualizerHeightFor(true) = %d, want remaining viewport %d", got, m.height-visualizerChromeLines())
	}
}

func TestVisualizerHeightRunsWithoutActivePlayback(t *testing.T) {
	m := model{
		width:  80,
		height: 18,
	}

	if got := m.visualizerHeightFor(false); got != m.height {
		t.Fatalf("visualizerHeightFor(false) = %d, want full viewport %d", got, m.height)
	}
}

func TestVisualizerHeightSkipsWhenViewportCannotFitChrome(t *testing.T) {
	m := model{
		width:      80,
		height:     2,
		visualizer: visualizer.New(8),
	}
	if !m.visualizer.Available() {
		t.Skip("ffmpeg not available")
	}

	if got := m.visualizerHeightFor(true); got != 0 {
		t.Fatalf("visualizerHeightFor(true) = %d, want 0", got)
	}
}

func TestViewRendersMatrixWhenIdleAndMenuClosed(t *testing.T) {
	m := model{
		cfg:        config.AppConfig{MusicDir: "/music"},
		player:     &fakeAudioPlayer{},
		visualizer: visualizer.New(8),
		width:      80,
		height:     18,
		status:     "ready",
	}

	view := stripANSI(m.View())
	if strings.TrimSpace(view) == "" {
		t.Fatalf("idle closed view should render matrix visualizer, got blank view")
	}
}

func TestViewPadsToTerminalHeight(t *testing.T) {
	m := model{
		cfg:        config.AppConfig{MusicDir: "/music"},
		player:     player.New(),
		visualizer: visualizer.New(8),
		width:      80,
		height:     18,
		status:     "ready",
	}

	view := m.View()
	if lines := strings.Count(view, "\n") + 1; lines != m.height {
		t.Fatalf("view lines = %d, want %d", lines, m.height)
	}
}

func TestMenuOverlaysMatrixBackground(t *testing.T) {
	m := model{
		cfg:        config.AppConfig{MusicDir: "/music"},
		player:     &fakeAudioPlayer{},
		visualizer: visualizer.New(8),
		width:      80,
		height:     18,
		showMenu:   true,
		status:     "ready",
		tracks: []library.Track{{
			Path: "/music/track.mp3",
			Name: "Track",
		}},
		view: tracksView,
	}

	lines := strings.Split(stripANSI(m.View()), "\n")
	menuTop := -1
	menuBottom := -1
	for index, line := range lines {
		if strings.Contains(line, "+---") {
			if menuTop == -1 {
				menuTop = index
			}
			menuBottom = index
		}
	}
	if menuTop == -1 {
		t.Fatalf("menu overlay missing box: %#v", lines)
	}
	backgroundVisible := false
	for index, line := range lines {
		if index >= menuTop && index <= menuBottom {
			continue
		}
		if strings.TrimSpace(line) != "" {
			backgroundVisible = true
			break
		}
	}
	if !backgroundVisible {
		t.Fatalf("menu should overlay matrix background, got only blank space outside menu: %#v", lines)
	}
}

func TestNewModelShowsCassetteTrackMenuByDefault(t *testing.T) {
	m := newModel(config.AppConfig{MusicDir: "/music"})

	if !m.showMenu {
		t.Fatalf("showMenu = false, want menu open by default on launch")
	}
}

func TestViewHidesCassetteTrackMenuWhenClosedAndTogglesOpen(t *testing.T) {
	m := model{
		cfg:        config.AppConfig{MusicDir: "/music"},
		player:     &fakeAudioPlayer{},
		visualizer: visualizer.New(8),
		width:      80,
		height:     18,
		status:     "ready",
		tracks: []library.Track{{
			Path: "/music/track.mp3",
			Name: "Track",
		}},
		view: tracksView,
	}

	view := m.View()
	if strings.Contains(view, "selected") || strings.Contains(view, "TRACK") || strings.Contains(view, "Track") {
		t.Fatalf("view should hide cassette/track menu until m toggles it: %q", view)
	}

	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'m'}})
	menu := next.(model)
	if !menu.showMenu {
		t.Fatalf("showMenu = false, want true after m")
	}
	menuView := menu.View()
	for _, want := range []string{"[A01]", "[01/01]", "TRACK", "Track"} {
		if !strings.Contains(menuView, want) {
			t.Fatalf("menu view %q missing %q", menuView, want)
		}
	}
	if strings.Contains(menuView, "selected") {
		t.Fatalf("menu view %q should not include selected filler text", menuView)
	}

	next, _ = menu.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'m'}})
	closed := next.(model)
	if closed.showMenu {
		t.Fatalf("showMenu = true, want false after second m")
	}
}

func TestMenuModalShowsCassetteTrackAndPlaybackOnOneLine(t *testing.T) {
	m := model{
		cfg:                 config.AppConfig{MusicDir: "/music"},
		player:              &fakeAudioPlayer{path: "/music/road-trip/blue-monday.mp3", elapsed: 65 * time.Second, playing: true},
		visualizer:          visualizer.New(8),
		width:               96,
		height:              18,
		status:              "playing",
		view:                tracksView,
		currentPlaylistName: "Road Trip",
		currentPath:         "/music/road-trip/blue-monday.mp3",
		tracks: []library.Track{{
			Path: "/music/road-trip/blue-monday.mp3",
			Name: "Blue Monday",
		}},
	}

	view := m.renderMenuModal()
	lines := strings.Split(view, "\n")
	menuLine := ""
	for _, line := range lines {
		if strings.Contains(line, "Road Trip") {
			menuLine = line
			break
		}
	}
	if menuLine == "" {
		t.Fatalf("menu view %q missing cassette/track line", view)
	}
	for _, want := range []string{"CASSETTE", "Road Trip", "LIVE", "Blue Monday", "playing", "01:05", "blue-monday"} {
		if !strings.Contains(menuLine, want) {
			t.Fatalf("menu line %q missing %q", menuLine, want)
		}
	}
	for index, line := range lines {
		if line == menuLine {
			continue
		}
		if strings.Contains(line, "Road Trip") || strings.Contains(line, "Blue Monday") {
			t.Fatalf("cassette/track information should stay on one menu line; line %d = %q", index, line)
		}
	}
}

func TestMenuBoxOmitsNonInputStatusRow(t *testing.T) {
	m := model{
		view:                tracksView,
		selected:            0,
		status:              "cassette contemplative_rock",
		currentPlaylistName: "contemplative_rock",
		tracks: []library.Track{{
			Path: "/music/contemplative_rock/hurt.mp3",
			Name: "hurt",
		}},
	}

	box := m.renderMenuBox(96)
	if strings.Count(box, "\n") != 2 {
		t.Fatalf("menu box should have one content row plus borders, got %q", box)
	}
	if strings.Contains(box, "| cassette contemplative_rock") {
		t.Fatalf("menu box should not render non-input status row: %q", box)
	}
}

func TestMenuBoxKeepsActiveInputPromptRow(t *testing.T) {
	m := model{
		view:      tracksView,
		selected:  0,
		inputKind: inputRename,
		input:     "hurt",
		tracks: []library.Track{{
			Path: "/music/contemplative_rock/hurt.mp3",
			Name: "hurt",
		}},
	}

	box := m.renderMenuBox(80)
	if strings.Count(box, "\n") != 3 {
		t.Fatalf("menu box should keep input prompt row plus borders, got %q", box)
	}
	if !strings.Contains(box, "rename hurt") {
		t.Fatalf("menu box missing input prompt row: %q", box)
	}
}

func TestViewOmitsSeparatePlaybackStatusLine(t *testing.T) {
	for _, status := range []string{"playing", "paused"} {
		t.Run(status, func(t *testing.T) {
			m := model{
				cfg:         config.AppConfig{MusicDir: "/music"},
				player:      &fakeAudioPlayer{path: "/music/road-trip/blue-monday.mp3", elapsed: 65 * time.Second, playing: true, paused: status == "paused"},
				visualizer:  visualizer.New(8),
				width:       96,
				height:      18,
				status:      status,
				view:        tracksView,
				currentPath: "/music/road-trip/blue-monday.mp3",
				tracks: []library.Track{{
					Path: "/music/road-trip/blue-monday.mp3",
					Name: "Blue Monday",
				}},
			}

			lines := strings.Split(m.View(), "\n")
			for _, line := range lines {
				if strings.TrimSpace(line) == status {
					t.Fatalf("view should not render separate %q status line: %#v", status, lines)
				}
			}
		})
	}
}

func TestViewOmitsPersistentShortcutMenu(t *testing.T) {
	m := model{
		cfg:        config.AppConfig{MusicDir: "/music"},
		player:     player.New(),
		visualizer: visualizer.New(8),
		width:      80,
		height:     18,
		status:     "ready",
	}

	view := m.View()
	for _, notWant := range []string{"j/k dial", "l/enter open/play", "dd delete"} {
		if strings.Contains(view, notWant) {
			t.Fatalf("view should not include persistent shortcut menu %q in %q", notWant, view)
		}
	}
}

func TestTickAdvancesVisualizerFrame(t *testing.T) {
	m := model{cfg: config.AppConfig{FPS: 20}}

	next, _ := m.Update(tickMsg(time.Now()))
	updated := next.(model)
	if updated.visualizerFrame != 1 {
		t.Fatalf("visualizerFrame = %d, want 1", updated.visualizerFrame)
	}
}

func TestTickFreezesVisualizerFrameWhenPaused(t *testing.T) {
	m := model{
		cfg:             config.AppConfig{FPS: 20},
		player:          &fakeAudioPlayer{path: "/music/live.mp3", paused: true, playing: true},
		currentPath:     "/music/live.mp3",
		visualizerFrame: 7,
	}

	next, _ := m.Update(tickMsg(time.Now()))
	updated := next.(model)
	if updated.visualizerFrame != 7 {
		t.Fatalf("visualizerFrame = %d, want frozen frame 7", updated.visualizerFrame)
	}
}

func TestVisualizerRowStyleAvoidsHorizontalColorBands(t *testing.T) {
	top := fmt.Sprint(visualizerRowStyle(0, 6).GetForeground())
	middle := fmt.Sprint(visualizerRowStyle(3, 6).GetForeground())
	bottom := fmt.Sprint(visualizerRowStyle(5, 6).GetForeground())

	if top != middle || middle != bottom {
		t.Fatalf("visualizer row styles should not create horizontal bands: top=%q middle=%q bottom=%q", top, middle, bottom)
	}
}

func TestPaletteUsesWhiteText(t *testing.T) {
	styles := []struct {
		name  string
		style lipgloss.Style
	}{
		{"title", titleStyle},
		{"status", statusStyle},
		{"error", errorStyle},
		{"cassettePlaying", cassettePlayingStyle},
		{"cassetteSelected", cassetteSelectedStyle},
		{"visualizerRainTrace", visualizerCellStyle(visualizer.ToneRainTrace, 0, 6)},
		{"visualizerRainBright", visualizerCellStyle(visualizer.ToneRainBright, 0, 6)},
	}
	for _, item := range styles {
		foreground := fmt.Sprint(item.style.GetForeground())
		if foreground != "15" {
			t.Fatalf("%s foreground = %q, want white", item.name, foreground)
		}
	}
}

func TestPauseOpensMenuAndResumeLeavesIt(t *testing.T) {
	player := &fakeAudioPlayer{path: "/music/live.mp3", playing: true}
	m := model{
		player:        player,
		visualizer:    visualizer.New(4),
		showDownloads: true,
		downloads:     []store.Job{{ID: "a", Title: "Blue Monday", Status: "running"}},
	}

	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{' '}})
	paused := next.(model)
	if !player.IsPaused() {
		t.Fatal("space did not pause")
	}
	if !paused.showMenu {
		t.Fatal("pause left the menu closed")
	}
	if paused.showDownloads {
		t.Fatal("pause left the download view covering the menu")
	}
	if paused.downloads[0].Status != "running" {
		t.Fatal("pause cancelled the download")
	}

	next, _ = paused.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{' '}})
	resumed := next.(model)
	if player.IsPaused() {
		t.Fatal("second space did not resume")
	}
	if !resumed.showMenu {
		t.Fatal("resume closed the menu")
	}
}

func TestServeResumeClosesTheMenu(t *testing.T) {
	player := &fakeAudioPlayer{path: "/music/live.mp3", playing: true}
	m := model{
		player:     player,
		visualizer: visualizer.New(4),
		leave:      func() {},
	}

	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{' '}})
	paused := next.(model)
	if !player.IsPaused() || !paused.showMenu {
		t.Fatalf("pause: paused %v menu %v", player.IsPaused(), paused.showMenu)
	}

	next, _ = paused.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{' '}})
	resumed := next.(model)
	if player.IsPaused() {
		t.Fatal("second space did not resume")
	}
	if resumed.showMenu {
		t.Fatal("resume left the menu over the picture")
	}
	if state, _, _ := resumed.nowPlaying(); state != "playing" {
		t.Fatalf("state = %q, want playing", state)
	}
}

func TestPauseWithoutTrackDoesNotOpenMenu(t *testing.T) {
	m := model{player: &fakeAudioPlayer{}, visualizer: visualizer.New(4)}
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{' '}})
	updated := next.(model)
	if updated.showMenu {
		t.Fatal("space opened the menu with nothing playing")
	}
	if updated.status != "nothing is playing" {
		t.Fatalf("status = %q", updated.status)
	}
}

func TestCassetteSelectedStyleUsesDefaultBackground(t *testing.T) {
	if background := fmt.Sprint(cassetteSelectedStyle.GetBackground()); background != "{}" {
		t.Fatalf("cassette selected background = %q, want terminal default", background)
	}
}

func TestCassetteSlotLabels(t *testing.T) {
	cases := map[int]string{
		0:  "A01",
		9:  "A10",
		10: "B01",
		25: "C06",
	}

	for index, want := range cases {
		if got := cassetteSlot(index); got != want {
			t.Fatalf("cassetteSlot(%d) = %q, want %q", index, got, want)
		}
	}
}

func TestRenderCassetteDialRowSelectedTrack(t *testing.T) {
	m := model{
		view:     tracksView,
		selected: 0,
		width:    64,
		tracks: []library.Track{{
			Path: "/music/blue-monday.mp3",
			Name: "Blue Monday",
		}},
	}

	row := m.renderCassetteDialRow(0)
	for _, want := range []string{"[A01]", "[01/01]", "TRACK", "Blue Monday"} {
		if !strings.Contains(row, want) {
			t.Fatalf("row %q missing %q", row, want)
		}
	}
	if strings.Contains(row, "O====O") {
		t.Fatalf("row %q should not include decorative cassette rail", row)
	}
}

func TestRenderCassetteDialRowSelectedCassette(t *testing.T) {
	m := model{
		view:     rootView,
		selected: 0,
		width:    72,
		playlists: []library.Playlist{{
			Path: "/music/road-trip",
			Name: "Road Trip",
		}},
	}

	row := m.renderCassetteDialRow(0)
	for _, want := range []string{"[A01]", "[01/01]", "CASSETTE", "Road Trip"} {
		if !strings.Contains(row, want) {
			t.Fatalf("row %q missing %q", row, want)
		}
	}
	if strings.Contains(row, "O====O") {
		t.Fatalf("row %q should not include decorative cassette rail", row)
	}
}

func TestMenuDialShowsOnlySelectedSlot(t *testing.T) {
	m := model{
		view:     tracksView,
		selected: 1,
		width:    64,
		tracks: []library.Track{
			{Path: "/music/first.mp3", Name: "First Track"},
			{Path: "/music/second.mp3", Name: "Second Track"},
			{Path: "/music/third.mp3", Name: "Third Track"},
		},
	}

	rendered := m.menuDialLine(60)
	for _, want := range []string{"A02", "02/03", "TRACK", "Second Track"} {
		if !strings.Contains(rendered, want) {
			t.Fatalf("rendered %q missing %q", rendered, want)
		}
	}
	for _, notWant := range []string{"First Track", "Third Track"} {
		if strings.Contains(rendered, notWant) {
			t.Fatalf("rendered %q should not include %q", rendered, notWant)
		}
	}
}

func TestMenuDialUsesFixedSpacingBetweenSelectedAndPlaying(t *testing.T) {
	m := model{
		view:     tracksView,
		selected: 0,
		width:    80,
		tracks: []library.Track{
			{Path: "/music/short.mp3", Name: "Short"},
		},
	}
	shortLine := m.menuDialLine(76)

	m.tracks[0].Name = "A very long selected track title that should be clipped"
	longLine := m.menuDialLine(76)

	if strings.Contains(shortLine, "|") || strings.Contains(longLine, "|") {
		t.Fatalf("menu dial lines should not use hard pipes: short=%q long=%q", shortLine, longLine)
	}
	shortPlaying := strings.Index(shortLine, " idle")
	longPlaying := strings.Index(longLine, " idle")
	if shortPlaying < 0 || longPlaying < 0 {
		t.Fatalf("menu dial lines missing fixed playing field: short=%q long=%q", shortLine, longLine)
	}
	if shortPlaying != longPlaying {
		t.Fatalf("playing field moved from %d to %d", shortPlaying, longPlaying)
	}
}

func TestMenuDialShowsThePlayingSongName(t *testing.T) {
	m := model{
		view:     rootView,
		selected: 0,
		player: &fakeAudioPlayer{
			path:    "/music/cyborg/Working as Intended.mp3",
			elapsed: 4*time.Minute + 11*time.Second,
			playing: true,
		},
		playlists: []library.Playlist{{Path: "/music/cyborg", Name: "cyborg"}},
	}
	line := m.menuDialLine(116)
	if !strings.Contains(line, "playing 04:11 Working as Intended") {
		t.Fatalf("line = %q", line)
	}
}

func TestMenuDialTruncatesTheSongTitleAfterTheClock(t *testing.T) {
	title := strings.Repeat("a", 80)
	m := model{
		view:     rootView,
		selected: 0,
		player: &fakeAudioPlayer{
			path:    "/music/cyborg/" + title + ".mp3",
			elapsed: 4*time.Minute + 11*time.Second,
			playing: true,
		},
		playlists: []library.Playlist{{Path: "/music/cyborg", Name: "cyborg"}},
	}
	line := m.menuDialLine(80)
	if !strings.Contains(line, "playing 04:11 ") || strings.Contains(line, "04:11...") {
		t.Fatalf("line = %q", line)
	}
	if !strings.HasSuffix(strings.TrimRight(line, " "), "...") {
		t.Fatalf("title was not end-truncated: %q", line)
	}
}

func TestCassetteDialNavigationWrapsWithJKModel(t *testing.T) {
	m := model{
		view:     tracksView,
		selected: 0,
		tracks: []library.Track{
			{Path: "/music/first.mp3", Name: "First Track"},
			{Path: "/music/second.mp3", Name: "Second Track"},
		},
	}

	m.moveSelection(-1)
	if m.selected != 1 {
		t.Fatalf("selected after previous = %d, want 1", m.selected)
	}
	m.moveSelection(1)
	if m.selected != 0 {
		t.Fatalf("selected after next = %d, want 0", m.selected)
	}
}

func TestEnterOpensCassetteAndEscapeRestoresCassetteSlot(t *testing.T) {
	m := model{
		view:     rootView,
		selected: 1,
		player:   &fakeAudioPlayer{},
		playlists: []library.Playlist{
			{Path: "/music/first", Name: "First"},
			{Path: "/music/second", Name: "Second"},
		},
	}

	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	opened := next.(model)
	if opened.view != tracksView {
		t.Fatalf("view after enter = %v, want tracksView", opened.view)
	}
	if opened.currentPlaylistName != "Second" {
		t.Fatalf("current cassette = %q, want Second", opened.currentPlaylistName)
	}
	if opened.selected != 0 {
		t.Fatalf("track selection after open = %d, want 0", opened.selected)
	}

	opened.selected = 3
	next, _ = opened.Update(tea.KeyMsg{Type: tea.KeyEsc})
	back := next.(model)
	if back.view != rootView {
		t.Fatalf("view after esc = %v, want rootView", back.view)
	}
	if back.selected != 1 {
		t.Fatalf("root selection after esc = %d, want original cassette slot 1", back.selected)
	}
}

func TestEscapeLeavesCassetteSelectionMenuVisibilityAlone(t *testing.T) {
	m := model{
		view:                 tracksView,
		selected:             1,
		showMenu:             true,
		cassetteSelected:     0,
		cassetteSelectedPath: "/music/first",
		currentPlaylist:      "/music/first",
		currentPlaylistName:  "First",
		player:               &fakeAudioPlayer{},
		playlists: []library.Playlist{
			{Path: "/music/first", Name: "First"},
			{Path: "/music/second", Name: "Second"},
		},
		tracks: []library.Track{
			{Path: "/music/first/first.mp3", Name: "First Track"},
			{Path: "/music/first/second.mp3", Name: "Second Track"},
		},
	}

	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	updated := next.(model)
	if !updated.showMenu {
		t.Fatalf("showMenu = false, want esc to leave menu visibility unchanged")
	}
	if updated.view != rootView {
		t.Fatalf("view after esc = %v, want rootView cassette selection", updated.view)
	}
	if updated.selected != 0 {
		t.Fatalf("selected after esc = %d, want cassette slot 0", updated.selected)
	}
}

func TestEscapeRestoresCassetteByPathIfMenuOrderChanges(t *testing.T) {
	m := model{
		view:     rootView,
		selected: 1,
		player:   &fakeAudioPlayer{},
		playlists: []library.Playlist{
			{Path: "/music/first", Name: "First"},
			{Path: "/music/second", Name: "Second"},
		},
	}

	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	opened := next.(model)
	opened.playlists = []library.Playlist{
		{Path: "/music/second", Name: "Second"},
		{Path: "/music/first", Name: "First"},
	}

	next, _ = opened.Update(tea.KeyMsg{Type: tea.KeyEsc})
	back := next.(model)
	if back.selected != 0 {
		t.Fatalf("root selection after reordered esc = %d, want cassette path at slot 0", back.selected)
	}
	if back.playlists[back.selected].Name != "Second" {
		t.Fatalf("selected cassette after reordered esc = %q, want Second", back.playlists[back.selected].Name)
	}
}

func TestHLNavigateWithinCurrentCassetteOrTrackDialOnly(t *testing.T) {
	root := model{
		view:     rootView,
		selected: 0,
		player:   &fakeAudioPlayer{},
		playlists: []library.Playlist{
			{Path: "/music/first", Name: "First"},
			{Path: "/music/second", Name: "Second"},
		},
	}
	next, _ := root.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'l'}})
	afterL := next.(model)
	if afterL.view != rootView {
		t.Fatalf("l changed view to %v, want rootView", afterL.view)
	}
	if afterL.selected != 1 {
		t.Fatalf("l selected = %d, want next cassette slot 1", afterL.selected)
	}

	tracks := model{
		view:                tracksView,
		selected:            1,
		cassetteSelected:    1,
		currentPlaylist:     "/music/first",
		currentPlaylistName: "First",
		player:              &fakeAudioPlayer{},
		tracks: []library.Track{
			{Path: "/music/first/first.mp3", Name: "First Track"},
			{Path: "/music/first/second.mp3", Name: "Second Track"},
		},
	}
	next, _ = tracks.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'h'}})
	afterH := next.(model)
	if afterH.view != tracksView {
		t.Fatalf("h changed view to %v, want tracksView", afterH.view)
	}
	if afterH.selected != 0 {
		t.Fatalf("h selected = %d, want previous track slot 0", afterH.selected)
	}
}

func TestRenderCassetteDialRowMarksCurrentPlaying(t *testing.T) {
	m := model{
		view:        tracksView,
		selected:    1,
		width:       48,
		currentPath: "/music/live.mp3",
		tracks: []library.Track{
			{Path: "/music/live.mp3", Name: "Live Track"},
			{Path: "/music/next.mp3", Name: "Next Track"},
		},
	}

	row := m.renderCassetteDialRow(0)
	for _, want := range []string{"*", "[A01]", "LIVE", "Live Track"} {
		if !strings.Contains(row, want) {
			t.Fatalf("row %q missing %q", row, want)
		}
	}
	if strings.Contains(row, "O====O") {
		t.Fatalf("row %q should not include decorative cassette rail", row)
	}
}

func TestPlayPathStatusOmitsTrackName(t *testing.T) {
	m := model{
		player:     &fakeAudioPlayer{},
		visualizer: &visualizer.Analyzer{},
	}

	cmd := m.playPath("/music/blue-monday.mp3", 0)
	m = land(t, m, cmd)

	if m.status != "playing" {
		t.Fatalf("status = %q, want playing without track name", m.status)
	}
	if strings.Contains(m.status, "blue") || strings.Contains(m.status, "monday") {
		t.Fatalf("status = %q, should not include track name", m.status)
	}
}

func TestPlayPathClosesMenuAfterSuccessfulPlayback(t *testing.T) {
	m := model{
		player:     &fakeAudioPlayer{},
		visualizer: &visualizer.Analyzer{},
		showMenu:   true,
	}

	cmd := m.playPath("/music/blue-monday.mp3", 0)
	m = land(t, m, cmd)

	if m.showMenu {
		t.Fatalf("showMenu = true, want menu closed after playback starts")
	}
}

func TestPlayPathKeepsMenuOpenAfterPlaybackError(t *testing.T) {
	m := model{
		player:     &fakeAudioPlayer{playErr: fmt.Errorf("ffplay failed")},
		visualizer: &visualizer.Analyzer{},
		showMenu:   true,
	}

	cmd := m.playPath("/music/blue-monday.mp3", 0)
	m = land(t, m, cmd)

	if !m.showMenu {
		t.Fatalf("showMenu = false, want menu to remain open after playback error")
	}
}

func TestPlayPathStartsVisualizerEase(t *testing.T) {
	m := model{
		player:          &fakeAudioPlayer{},
		visualizer:      &visualizer.Analyzer{},
		visualizerFrame: 42,
	}

	cmd := m.playPath("/music/blue-monday.mp3", 0)
	m = land(t, m, cmd)

	if m.visualizerStartFrame != 42 {
		t.Fatalf("visualizerStartFrame = %d, want current frame 42", m.visualizerStartFrame)
	}
	if scale := m.visualizerStartupScale(); math.Abs(scale-visualizerStartupIntensityFloor) > 0.000001 {
		t.Fatalf("startup scale = %.6f, want initial ease floor %.2f", scale, visualizerStartupIntensityFloor)
	}
	m.visualizerFrame += visualizerStartupEaseFrames(m.cfg.FPS)
	if scale := m.visualizerStartupScale(); scale != 1 {
		t.Fatalf("startup scale after ease = %.6f, want 1", scale)
	}
}

func TestPlayPathStartsTrackChangeTransition(t *testing.T) {
	m := model{
		cfg:             config.AppConfig{FPS: 20},
		player:          &fakeAudioPlayer{},
		visualizer:      &visualizer.Analyzer{},
		currentPath:     "/music/old-song.mp3",
		visualizerFrame: 90,
	}

	cmd := m.playPath("/music/new-song.mp3", 0)
	m = land(t, m, cmd)

	if !m.visualizerTransitionActive {
		t.Fatalf("visualizerTransitionActive = false, want true when changing songs")
	}
	if m.visualizerTransitionStartFrame != 90 {
		t.Fatalf("visualizerTransitionStartFrame = %d, want 90", m.visualizerTransitionStartFrame)
	}
}

func TestPlayPathDoesNotUseTrackChangeTransitionForSameSong(t *testing.T) {
	m := model{
		player:                     &fakeAudioPlayer{},
		visualizer:                 &visualizer.Analyzer{},
		currentPath:                "/music/current-song.mp3",
		visualizerTransitionActive: true,
	}

	cmd := m.playPath("/music/current-song.mp3", 0)
	m = land(t, m, cmd)

	if m.visualizerTransitionActive {
		t.Fatalf("visualizerTransitionActive = true, want false when restarting same song")
	}
}

func TestVisualizerStartupScaleRampsGradually(t *testing.T) {
	m := model{
		cfg:                  config.AppConfig{FPS: 20},
		currentPath:          "/music/blue-monday.mp3",
		visualizerStartFrame: 10,
		visualizerFrame:      10,
	}

	m.visualizerFrame++
	if scale := m.visualizerStartupScale(); scale > 0.03 {
		t.Fatalf("startup scale after first frame = %.6f, want very gentle ramp", scale)
	}

	m.visualizerFrame = m.visualizerStartFrame + m.cfg.FPS
	if scale := m.visualizerStartupScale(); scale > 0.15 {
		t.Fatalf("startup scale after one second = %.6f, want slow ease-in", scale)
	}

	m.visualizerFrame = m.visualizerStartFrame + visualizerStartupEaseFrames(m.cfg.FPS)/2
	if scale := m.visualizerStartupScale(); scale < 0.45 || scale > 0.55 {
		t.Fatalf("startup scale halfway = %.6f, want around midpoint", scale)
	}
}

func TestVisualizerTrackChangeSignalSlowsThenRampsNewSong(t *testing.T) {
	m := model{
		cfg:                            config.AppConfig{FPS: 20},
		visualizerFrame:                100,
		visualizerTransitionActive:     true,
		visualizerTransitionStartFrame: 100,
		visualizerTransitionLevels:     []float64{1.0, 0.5},
		visualizerTransitionPeaks:      []float64{0.8, 0.4},
	}
	newLevels := []float64{0.9, 0.7}
	newPeaks := []float64{0.6, 0.3}
	downFrames := visualizerTrackChangeDownFrames(m.cfg.FPS)
	upFrames := visualizerTrackChangeUpFrames(m.cfg.FPS)

	levels, peaks := m.visualizerTransitionSignal(newLevels, newPeaks)
	if !nearFloat(levels[0], 1.0) || !nearFloat(peaks[0], 0.8) {
		t.Fatalf("transition start should preserve old signal: levels=%#v peaks=%#v", levels, peaks)
	}

	m.visualizerFrame = m.visualizerTransitionStartFrame + downFrames/2
	levels, _ = m.visualizerTransitionSignal(newLevels, newPeaks)
	if levels[0] >= 1.0 || levels[0] <= 0.3 {
		t.Fatalf("transition down phase should gradually reduce old signal, got levels=%#v", levels)
	}

	m.visualizerFrame = m.visualizerTransitionStartFrame + downFrames
	levels, _ = m.visualizerTransitionSignal(newLevels, newPeaks)
	if levels[0] > newLevels[0]*0.03 {
		t.Fatalf("transition should return near neutral before new song ramps, got levels=%#v", levels)
	}

	m.visualizerFrame = m.visualizerTransitionStartFrame + downFrames + upFrames/2
	levels, _ = m.visualizerTransitionSignal(newLevels, newPeaks)
	if levels[0] <= 0.35 || levels[0] >= 0.6 {
		t.Fatalf("transition up phase should gradually release new signal, got levels=%#v", levels)
	}

	m.visualizerFrame = m.visualizerTransitionStartFrame + downFrames + upFrames
	levels, peaks = m.visualizerTransitionSignal(newLevels, newPeaks)
	if !nearFloat(levels[0], newLevels[0]) || !nearFloat(peaks[0], newPeaks[0]) {
		t.Fatalf("transition end should use full new signal: levels=%#v peaks=%#v", levels, peaks)
	}
}

func TestVisualizerTrackChangeTransitionExpires(t *testing.T) {
	m := model{
		cfg:                            config.AppConfig{FPS: 20},
		visualizerTransitionActive:     true,
		visualizerTransitionStartFrame: 10,
		visualizerTransitionLevels:     []float64{1},
		visualizerTransitionPeaks:      []float64{1},
	}
	m.visualizerFrame = m.visualizerTransitionStartFrame +
		visualizerTrackChangeDownFrames(m.cfg.FPS) +
		visualizerTrackChangeUpFrames(m.cfg.FPS)

	m.clearExpiredVisualizerTransition()

	if m.visualizerTransitionActive || len(m.visualizerTransitionLevels) != 0 || len(m.visualizerTransitionPeaks) != 0 {
		t.Fatalf("expired transition was not cleared: %#v", m)
	}
}

func TestScaleVisualizerSignalAppliesStartupEase(t *testing.T) {
	levels := []float64{1, 0.5}
	peaks := []float64{0.25, 0.75}

	scaledLevels, scaledPeaks := scaleVisualizerSignal(levels, peaks, 0.4)

	if !nearFloat(scaledLevels[0], 0.4) || !nearFloat(scaledLevels[1], 0.2) {
		t.Fatalf("scaled levels = %#v, want [0.4 0.2]", scaledLevels)
	}
	if !nearFloat(scaledPeaks[0], 0.1) || !nearFloat(scaledPeaks[1], 0.3) {
		t.Fatalf("scaled peaks = %#v, want [0.1 0.3]", scaledPeaks)
	}
	if levels[0] != 1 || peaks[1] != 0.75 {
		t.Fatalf("scaleVisualizerSignal mutated source slices: levels=%#v peaks=%#v", levels, peaks)
	}
}

func TestVisualizerFrameCellsKeepIdleRainDuringMusicEase(t *testing.T) {
	m := model{
		currentPath:          "/music/blue-monday.mp3",
		visualizerStartFrame: 10,
		visualizerFrame:      10,
	}

	rain := countAppRainTones(m.visualizerFrameCells(48, 18))
	if rain < 20 {
		t.Fatalf("startup music ease should keep baseline matrix rain visible, rain=%d", rain)
	}
}

func nearFloat(got, want float64) bool {
	return math.Abs(got-want) <= 0.000001
}

func stripANSI(value string) string {
	return ansiPattern.ReplaceAllString(value, "")
}

func countAppRainTones(cells [][]visualizer.Cell) int {
	count := 0
	for _, row := range cells {
		for _, cell := range row {
			switch cell.Tone {
			case visualizer.ToneRainTrace, visualizer.ToneRainDim, visualizer.ToneRainSoft, visualizer.ToneRainGlow, visualizer.ToneRainBright:
				count++
			}
		}
	}
	return count
}

func TestYOpensYouTubeURLModalInOpenCassette(t *testing.T) {
	m := model{
		view:            tracksView,
		currentPlaylist: "/music/cassette",
		width:           80,
		height:          18,
		status:          "ready",
	}

	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	updated := next.(model)
	if updated.inputKind != inputYouTubeURL {
		t.Fatalf("inputKind = %q, want YouTube URL modal", updated.inputKind)
	}
	view := updated.View()
	for _, want := range []string{"YouTube download", "YouTube URL"} {
		if !strings.Contains(view, want) {
			t.Fatalf("modal view %q missing %q", view, want)
		}
	}
}

func TestYRequiresOpenCassette(t *testing.T) {
	m := model{view: rootView, status: "ready"}

	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	updated := next.(model)
	if updated.inputKind != "" {
		t.Fatalf("inputKind = %q, want no modal outside cassette", updated.inputKind)
	}
	if updated.status != "open a cassette first" {
		t.Fatalf("status = %q, want cassette guard", updated.status)
	}
}

func TestYouTubeURLModalAdvancesToSongName(t *testing.T) {
	m := model{
		inputKind: inputYouTubeURL,
		input:     "https://youtu.be/example",
	}

	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	updated := next.(model)
	if updated.inputKind != inputYouTubeName {
		t.Fatalf("inputKind = %q, want song-name modal", updated.inputKind)
	}
	if updated.youtubeURL != "https://youtu.be/example" {
		t.Fatalf("youtubeURL = %q, want stored URL", updated.youtubeURL)
	}
	if updated.input != "" {
		t.Fatalf("input = %q, want cleared song-name input", updated.input)
	}
}

func TestYouTubeURLModalRejectsInvalidURL(t *testing.T) {
	m := model{
		inputKind: inputYouTubeURL,
		input:     "https://example.com/watch?v=x",
	}

	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	updated := next.(model)
	if updated.inputKind != inputYouTubeURL {
		t.Fatalf("inputKind = %q, want to remain on URL modal", updated.inputKind)
	}
	if updated.status != "not a YouTube URL" {
		t.Fatalf("status = %q, want invalid URL status", updated.status)
	}
}

func TestYouTubeURLExitClosesThePrompt(t *testing.T) {
	m := model{
		inputKind:  inputYouTubeURL,
		input:      "  /exit  ",
		youtubeURL: "https://youtu.be/left",
	}

	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	updated := next.(model)
	if updated.inputKind != "" || updated.input != "" || updated.youtubeURL != "" {
		t.Fatalf("exit state = inputKind %q input %q url %q, want cleared", updated.inputKind, updated.input, updated.youtubeURL)
	}
	if updated.status != "cancelled" {
		t.Fatalf("status = %q, want cancelled", updated.status)
	}
}

func TestYouTubeNameExitClosesThePrompt(t *testing.T) {
	originalDownload := enqueueDownload
	defer func() { enqueueDownload = originalDownload }()
	called := false
	enqueueDownload = func(playlistDir, filename, rawURL string) error {
		called = true
		return nil
	}

	m := model{
		inputKind:  inputYouTubeName,
		input:      "/exit",
		youtubeURL: "https://youtu.be/example",
	}

	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	updated := next.(model)
	if cmd != nil || called {
		t.Fatal("exit queued a download")
	}
	if updated.inputKind != "" || updated.input != "" || updated.youtubeURL != "" {
		t.Fatalf("exit state = inputKind %q input %q url %q, want cleared", updated.inputKind, updated.input, updated.youtubeURL)
	}
	if updated.status != "cancelled" {
		t.Fatalf("status = %q, want cancelled", updated.status)
	}
}

func TestYouTubeNameModalDownloadsTrack(t *testing.T) {
	originalDownload := enqueueDownload
	defer func() { enqueueDownload = originalDownload }()

	var gotPlaylistDir, gotFilename, gotURL string
	enqueueDownload = func(playlistDir, filename, rawURL string) error {
		gotPlaylistDir = playlistDir
		gotFilename = filename
		gotURL = rawURL
		return nil
	}

	playlistDir := t.TempDir()
	m := model{
		view:            tracksView,
		currentPlaylist: playlistDir,
		inputKind:       inputYouTubeName,
		input:           "Blue Monday",
		youtubeURL:      "https://youtu.be/example",
	}

	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	painted := next.(model)
	if gotURL != "" {
		t.Fatal("the download was queued inside Update")
	}
	updated := land(t, painted, cmd)
	if updated.inputKind != "" || updated.youtubeURL != "" {
		t.Fatalf("modal state after download = inputKind %q url %q, want cleared", updated.inputKind, updated.youtubeURL)
	}
	if updated.status != "downloading Blue Monday" || !updated.showDownloads {
		t.Fatalf("status = %q showDownloads=%v, want background download", updated.status, updated.showDownloads)
	}
	if gotPlaylistDir != playlistDir || gotFilename != "Blue Monday" || gotURL != "https://youtu.be/example" {
		t.Fatalf("download args = (%q, %q, %q)", gotPlaylistDir, gotFilename, gotURL)
	}
}

func TestYouTubeModalCancelClearsPendingURL(t *testing.T) {
	m := model{
		inputKind:  inputYouTubeName,
		input:      "Blue Monday",
		youtubeURL: "https://youtu.be/example",
	}

	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	updated := next.(model)
	if updated.inputKind != "" || updated.input != "" || updated.youtubeURL != "" {
		t.Fatalf("cancelled modal state = inputKind %q input %q url %q, want cleared", updated.inputKind, updated.input, updated.youtubeURL)
	}
	if updated.status != "cancelled" {
		t.Fatalf("status = %q, want cancelled", updated.status)
	}
}

func TestCommandBarRemovedFromHelpAndKeymap(t *testing.T) {
	m := model{}

	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{':'}})
	updated := next.(model)
	if updated.inputKind != "" {
		t.Fatalf("colon opened inputKind %q, want command bar removed", updated.inputKind)
	}

	help := m.renderHelp()
	if strings.Contains(help, "command bar") || strings.Contains(help, "name.mp3 https://youtube") {
		t.Fatalf("help should not mention command bar: %q", help)
	}
	if !strings.Contains(help, "y                   download YouTube audio") {
		t.Fatalf("help missing y download shortcut: %q", help)
	}
}

func TestQEndsTheSession(t *testing.T) {
	player := &fakeAudioPlayer{path: "/music/track.mp3", playing: true}
	m := model{
		view:          rootView,
		player:        player,
		visualizer:    visualizer.New(8),
		currentPath:   "/music/track.mp3",
		showDownloads: true,
		downloads:     []store.Job{{ID: "a", Title: "Blue Monday", Status: "running"}},
	}

	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	if !isQuitCommand(cmd) {
		t.Fatal("q did not quit")
	}
	if player.IsPlaying() {
		t.Fatal("q left playback running")
	}
	if next.(model).downloads[0].Status != "running" {
		t.Fatal("q cancelled the download")
	}

	typing := model{inputKind: inputSearch, input: "so", visualizer: visualizer.New(4)}
	next, cmd = typing.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	if isQuitCommand(cmd) {
		t.Fatal("q quit while a prompt was open")
	}
	if next.(model).input != "soq" {
		t.Fatalf("prompt = %q, want soq", next.(model).input)
	}
}

func TestServeQDoesNotStopOrEnd(t *testing.T) {
	player := &fakeAudioPlayer{path: "/music/track.mp3", playing: true}
	m := model{
		player:      player,
		visualizer:  visualizer.New(4),
		currentPath: "/music/track.mp3",
		leave:       func() {},
	}
	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	if cmd != nil {
		t.Fatal("q in musica serve returned a command")
	}
	if !player.IsPlaying() {
		t.Fatal("q stopped serve playback")
	}
	if next.(model).quitting {
		t.Fatal("q ended musica serve")
	}

	typing := model{leave: func() {}, inputKind: inputSearch, input: "so", visualizer: visualizer.New(4)}
	next, cmd = typing.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	if cmd != nil && isQuitCommand(cmd) {
		t.Fatal("q ended serve from a prompt")
	}
	if next.(model).input != "soq" {
		t.Fatalf("prompt = %q, want soq", next.(model).input)
	}
}

func TestCtrlCQuitsAndStopsPlaybackFromInput(t *testing.T) {
	player := &fakeAudioPlayer{path: "/music/track.mp3", playing: true}
	m := model{
		view:        tracksView,
		player:      player,
		visualizer:  visualizer.New(8),
		currentPath: "/music/track.mp3",
		inputKind:   inputSearch,
		input:       "query",
	}

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	if !isQuitCommand(cmd) {
		t.Fatal("ctrl+c did not return tea.Quit")
	}
	if player.IsPlaying() {
		t.Fatal("ctrl+c did not stop playback")
	}
}

func TestEscapeFromTracksReturnsToCassettesWithoutStoppingPlayback(t *testing.T) {
	player := &fakeAudioPlayer{path: "/music/track.mp3", playing: true}
	m := model{
		view:                tracksView,
		player:              player,
		visualizer:          visualizer.New(8),
		currentPath:         "/music/track.mp3",
		currentPlaylist:     "/music/cassette",
		currentPlaylistName: "Cassette",
	}

	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if isQuitCommand(cmd) {
		t.Fatal("first esc from tracks quit instead of returning to cassettes")
	}
	if next.(model).view != rootView {
		t.Fatalf("view after esc = %v, want rootView", next.(model).view)
	}
	if !player.IsPlaying() {
		t.Fatal("first esc from tracks stopped playback")
	}
}

func TestEscapeFromCassettesHidesDisplayAndKeepsPlayback(t *testing.T) {
	player := &fakeAudioPlayer{path: "/music/track.mp3", playing: true}
	m := model{
		view:             rootView,
		player:           player,
		visualizer:       visualizer.New(8),
		currentPath:      "/music/track.mp3",
		cassetteSelected: 3,
	}

	called := false
	previous := hideDisplay
	hideDisplay = func() error {
		called = true
		return nil
	}
	defer func() { hideDisplay = previous }()

	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if cmd == nil || isQuitCommand(cmd) {
		t.Fatal("esc from cassette menu should hide without quitting")
	}
	if _, ok := cmd().(hideDisplayMsg); !ok {
		t.Fatal("esc from cassette menu did not run the hide command")
	}
	if !called {
		t.Fatal("hide command was not called")
	}
	updated := next.(model)
	if !player.IsPlaying() {
		t.Fatal("esc from cassette menu stopped playback")
	}
	if updated.currentPath != "/music/track.mp3" || updated.cassetteSelected != 3 {
		t.Fatalf("esc changed retained state: path=%q cassette=%d", updated.currentPath, updated.cassetteSelected)
	}
}

func TestEndMessageQuitsAndStopsPlayback(t *testing.T) {
	player := &fakeAudioPlayer{path: "/music/track.mp3", playing: true}
	m := model{
		view:        rootView,
		player:      player,
		visualizer:  visualizer.New(8),
		currentPath: "/music/track.mp3",
	}

	_, cmd := m.Update(endMsg{})
	if !isQuitCommand(cmd) {
		t.Fatal("end message did not return tea.Quit")
	}
	if player.IsPlaying() {
		t.Fatal("end message did not stop playback")
	}
}

func isQuitCommand(cmd tea.Cmd) bool {
	if cmd == nil {
		return false
	}
	_, ok := cmd().(tea.QuitMsg)
	return ok
}
