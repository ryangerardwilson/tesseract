package app

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"
	"unicode"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/ryangerardwilson/tesseract/internal/musica/config"
	"github.com/ryangerardwilson/tesseract/internal/musica/download"
	"github.com/ryangerardwilson/tesseract/internal/musica/session"
)

// watchers is where musica serve sends its frames and events.
type watchers interface {
	Publish(line []byte)
	Watching() bool
}

// served is the model as musica serve runs it for the Tesseract Musica
// center: nothing is drawn, and each update goes to the watchers as a frame.
type served struct {
	model
	out watchers
}

func (s served) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	next, cmd := s.model.Update(msg)
	s.model = next.(model)
	// Every update takes the kick heard since the one before, watched or
	// not, so a frame's thump is never older than the update before it.
	s.model.thump = s.model.takeThump()
	if s.out.Watching() {
		s.out.Publish(s.model.frameLine())
	}
	return s, cmd
}

// takeThump is the hardest kick the analyzer heard since it was last asked,
// or 0 with no analyzer.
func (m model) takeThump() float64 {
	if m.visualizer == nil {
		return 0
	}
	return m.visualizer.Thump()
}

func (s served) View() string {
	return ""
}

// hold opens the cassette of track, the track an ended session had loaded,
// with that track selected and the cassette menu open, and shows it paused.
// It does not start the track: where that session was in it is not known, so
// nothing plays until the operator plays a track. A missing track is skipped.
func (m *model) hold(track string) {
	if track == "" {
		return
	}
	if info, err := os.Stat(track); err != nil || info.IsDir() {
		return
	}
	m.openTrackContext(track)
	m.cassetteSelectedPath = m.currentPlaylist
	m.held = track
	m.showMenu = true
	m.status = "paused"
}

// Serve runs the player with no terminal for the Tesseract Musica center. It
// holds the musica session as the TUI does and keeps playing after every
// watcher leaves. ctrl+c, musica end, SIGTERM, SIGINT, or SIGHUP stop
// playback and end it. q does not. reload replaces this process with the
// binary on disk and continues from the track that was loaded. When musica
// end ended a session that had a track loaded, and no reload note was left,
// Serve opens that track's cassette with the track held paused.
func Serve(cfg config.AppConfig) int {
	control, err := session.ListenWatched()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	defer control.Close()

	m := newModel(cfg)
	m.trackDownloads = true
	m.leave = func() { control.Publish(eventLine("leave")) }
	if note, ok := control.TakeResume(); ok {
		m.armResume(note)
	} else {
		m.hold(control.TakeHeld())
	}
	// Queued downloads resume beside the player, so a client that watches
	// gets its first frame without waiting on the store.
	go download.Resume()
	defer m.close()
	program := tea.NewProgram(served{model: m, out: control},
		tea.WithInput(nil), tea.WithOutput(io.Discard), tea.WithoutRenderer(), tea.WithoutSignalHandler())
	go sendKeys(program, control.Keys())
	go func() {
		<-control.EndRequests()
		program.Send(endMsg{})
	}()
	go func() {
		requests := control.ReloadRequests()
		if requests == nil {
			return
		}
		<-requests
		program.Send(reloadMsg{})
	}()
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGTERM, syscall.SIGINT, syscall.SIGHUP)
	defer signal.Stop(signals)
	go func() {
		<-signals
		program.Send(endMsg{})
	}()
	final, err := program.Run()
	if next, ok := final.(served); ok && next.reloading {
		if saveErr := control.SaveResume(next.reloadNote); saveErr != nil {
			fmt.Fprintln(os.Stderr, saveErr)
		}
		control.Close()
		session.ExecServe()
	}
	control.Publish(eventLine("ended"))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return 0
}

// keyNames are the keys a watcher names that are not one printable rune.
var keyNames = map[string]tea.KeyType{
	"enter":     tea.KeyEnter,
	"esc":       tea.KeyEsc,
	"backspace": tea.KeyBackspace,
	"ctrl+h":    tea.KeyCtrlH,
	"ctrl+w":    tea.KeyCtrlW,
	"ctrl+c":    tea.KeyCtrlC,
	"ctrl+j":    tea.KeyCtrlJ,
	"ctrl+k":    tea.KeyCtrlK,
	"up":        tea.KeyUp,
	"down":      tea.KeyDown,
	"left":      tea.KeyLeft,
	"right":     tea.KeyRight,
}

// keyMsg reads a watcher's key line, {"key":"<name>"}, as the key the TUI
// would read: one of keyNames, or one printable rune such as "j", "?", or " ".
func keyMsg(line string) (tea.KeyMsg, bool) {
	var in struct {
		Key string `json:"key"`
	}
	if err := json.Unmarshal([]byte(line), &in); err != nil {
		return tea.KeyMsg{}, false
	}
	if kind, ok := keyNames[in.Key]; ok {
		return tea.KeyMsg{Type: kind}, true
	}
	runes := []rune(in.Key)
	if len(runes) != 1 || !unicode.IsPrint(runes[0]) {
		return tea.KeyMsg{}, false
	}
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: runes}, true
}

func sendKeys(program *tea.Program, keys <-chan string) {
	for line := range keys {
		if msg, ok := keyMsg(line); ok {
			program.Send(msg)
		}
	}
}

func eventLine(kind string) []byte {
	line, _ := json.Marshal(map[string]string{"type": kind})
	return line
}
