package app

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/ryangerardwilson/tesseract/internal/musica/config"
	"github.com/ryangerardwilson/tesseract/internal/musica/download"
	"github.com/ryangerardwilson/tesseract/internal/musica/library"
	"github.com/ryangerardwilson/tesseract/internal/musica/player"
	"github.com/ryangerardwilson/tesseract/internal/musica/session"
	"github.com/ryangerardwilson/tesseract/internal/musica/store"
	"github.com/ryangerardwilson/tesseract/internal/musica/visualizer"
)

type viewMode int

const (
	rootView viewMode = iota
	tracksView
)

const (
	inputSearch      = "search"
	inputRename      = "rename"
	inputCreate      = "create"
	inputYouTubeURL  = "youtube_url"
	inputYouTubeName = "youtube_name"
)

const (
	visualizerStartupEaseSeconds     = 4
	visualizerStartupIntensityFloor  = 0.02
	visualizerTrackChangeDownSeconds = 1
	visualizerTrackChangeUpSeconds   = 4
	menuModalMinWidth                = 44
	menuModalMaxWidth                = 120
)

var (
	titleStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("15")).Bold(true)
	mutedStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("15"))
	statusStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("15"))
	errorStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("15")).Bold(true)

	cassetteSlotStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("15")).Bold(true)
	cassetteRowStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("15"))
	cassettePlayingStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("15")).Bold(true)
	cassetteSelectedStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("15")).Bold(true)
)

var hideDisplay = func() error {
	command := strings.TrimSpace(os.Getenv("MUSICA_HIDE_COMMAND"))
	if command == "" {
		return fmt.Errorf("MUSICA_HIDE_COMMAND is not configured")
	}
	return exec.Command(command).Run()
}

type audioPlayer interface {
	Play(path string, offset time.Duration) error
	TogglePause() bool
	Stop()
	State() (string, time.Duration)
	IsPlaying() bool
	IsPaused() bool
}

type model struct {
	cfg config.AppConfig

	player     audioPlayer
	visualizer *visualizer.Analyzer

	view                 viewMode
	playlists            []library.Playlist
	rootTracks           []library.Track
	tracks               []library.Track
	selected             int
	cassetteSelected     int
	cassetteSelectedPath string
	currentPlaylist      string
	currentPlaylistName  string

	queue        []library.Track
	currentIndex int
	currentPath  string

	showHelp   bool
	showMenu   bool
	inputKind  string
	input      string
	youtubeURL string

	status      string
	search      string
	deleteArmed bool
	leader      bool
	leaderSeq   string

	width  int
	height int

	visualizerFrame                int
	visualizerStartFrame           int
	visualizerTransitionActive     bool
	visualizerTransitionStartFrame int
	visualizerTransitionLevels     []float64
	visualizerTransitionPeaks      []float64

	startupTrack string

	// The audio lane: one audioOp runs at a time and the newest request waits.
	audioBusy    bool
	audioWaiting bool
	audioNext    audioOp
	audioPending string
	quitting     bool

	showDownloads   bool
	downloads       []store.Job // the store's last answer, replaced whole
	downloadIntents []downloadIntent
	downloadOps     []downloadOp
	downloadBusy    bool
	downloadSeq     int
	downloadIndex   int
	trackDownloads  bool

	// leave is set when musica serve runs the model: m, and Escape from
	// anywhere, ask the Tesseract center to close instead of toggling the
	// menu or hiding a terminal, and h goes back to the cassette list.
	leave func()
	// held is the track an ended session had loaded. musica serve shows it
	// paused until a track is played or playback is stopped, and never starts
	// it.
	held string
	// thump is the hardest kick the analyzer heard by the last update, as
	// musica serve took it for that update's frame.
	thump float64
	// reloading replaces this process with the binary on disk. reloadNote is
	// the place that process continues from.
	reloading  bool
	reloadNote session.Resume
	// resumeArmed starts the noted track from Init after a reload.
	resumeArmed  bool
	resumePath   string
	resumeOffset time.Duration
	resumePaused bool
}

type tickMsg time.Time
type playMsg struct{ path string }
type hideDisplayMsg struct{ err error }
type endMsg struct{}
type reloadMsg struct{}
type resumeMsg struct {
	path   string
	offset time.Duration
	paused bool
}

func Run(cfg config.AppConfig) int {
	control, err := session.Listen()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	defer control.Close()

	m := newModel(cfg)
	m.trackDownloads = true
	download.Resume()
	defer m.close()
	program := tea.NewProgram(m, tea.WithAltScreen())
	go func() {
		<-control.EndRequests()
		program.Send(endMsg{})
	}()
	if _, err := program.Run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return 0
}

func newModel(cfg config.AppConfig) model {
	m := model{
		cfg:          cfg,
		player:       player.New(),
		visualizer:   visualizer.New(32),
		showMenu:     true,
		status:       "ready",
		width:        80,
		height:       24,
		startupTrack: cfg.StartupTrack,
	}
	m.reloadRoot()
	if cfg.StartupTrack != "" {
		m.openTrackContext(cfg.StartupTrack)
	}
	return m
}

func (m model) Init() tea.Cmd {
	cmds := []tea.Cmd{tickCmd(m.cfg.FPS)}
	if m.trackDownloads {
		cmds = append(cmds, func() tea.Msg { return downloadPollMsg{} })
	}
	if m.startupTrack != "" {
		path := m.startupTrack
		cmds = append(cmds, func() tea.Msg { return playMsg{path: path} })
	}
	if m.resumeArmed {
		path, offset, paused := m.resumePath, m.resumeOffset, m.resumePaused
		cmds = append(cmds, func() tea.Msg {
			return resumeMsg{path: path, offset: offset, paused: paused}
		})
	}
	return tea.Batch(cmds...)
}

func tickCmd(fps int) tea.Cmd {
	if fps < 5 {
		fps = config.DefaultFPS
	}
	delay := time.Second / time.Duration(fps)
	return tea.Tick(delay, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil
	case tickMsg:
		return m.tick()
	case playMsg:
		cmd := m.playPath(msg.path, 0)
		return m, cmd
	case audioDoneMsg:
		return m.finishAudio(msg)
	case downloadPollMsg:
		return m.pollDownloads()
	case downloadDoneMsg:
		return m.finishDownloadOp(msg)
	case hideDisplayMsg:
		if msg.err != nil {
			m.status = fmt.Sprintf("hide display: %v", msg.err)
		}
		return m, nil
	case endMsg:
		return m.quit()
	case reloadMsg:
		m.reloading = true
		m.reloadNote = m.captureResume()
		return m.quit()
	case resumeMsg:
		return m, m.startResume(msg)
	case tea.KeyMsg:
		return m.updateKeyMsg(msg)
	default:
		return m, nil
	}
}

// tick advances the frame. It reads nothing: downloads arrive by their own
// poll, and a track that ended starts the next one on the audio lane.
func (m model) tick() (tea.Model, tea.Cmd) {
	if m.shouldAdvanceVisualizerFrame() {
		m.visualizerFrame++
		m.clearExpiredVisualizerTransition()
	}
	var next tea.Cmd
	if m.trackEnded() {
		next = m.playNext()
	}
	return m, tea.Batch(tickCmd(m.cfg.FPS), next)
}

func (m model) updateKeyMsg(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()
	if m.quitting {
		if key == "ctrl+c" {
			return m, tea.Quit
		}
		return m, nil
	}
	// q ends the terminal player. musica serve keeps playing: the Tesseract
	// center has no q, and r reloads the binary instead.
	if key == "ctrl+c" || (m.leave == nil && m.inputKind == "" && key == "q") {
		return m.quit()
	}
	// In musica serve, Escape asks the center to close from anywhere and
	// backs out of nothing: a prompt, help, the download view, and a
	// cassette's tracks stay as they are for the next showing.
	if m.leave != nil && key == "esc" {
		return m, m.hideCmd()
	}
	if m.inputKind != "" {
		return m.updateInput(msg)
	}
	return m.updateKey(msg)
}

func (m model) shouldAdvanceVisualizerFrame() bool {
	return m.player == nil || !m.player.IsPaused()
}

func (m *model) togglePause() {
	if m.audioBusy {
		return // the track is starting or stopping
	}
	if m.player == nil {
		m.status = "nothing is playing"
		return
	}
	if path, _ := m.player.State(); path == "" {
		m.status = "nothing is playing"
		if m.held != "" {
			m.status = "nothing to resume; enter plays from the start"
		}
		return
	}
	paused := m.player.TogglePause()
	m.visualizer.SetPaused(paused)
	if !paused {
		m.status = "playing"
		// The center draws the animation only while the menu is closed.
		// Resume puts that picture back. The terminal player leaves the menu.
		if m.leave != nil {
			m.showMenu = false
		}
		return
	}
	m.status = "paused"
	m.showMenu = true
	m.showDownloads = false
	m.showHelp = false
}

func (m model) updateKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()
	if key == "?" {
		m.showHelp = !m.showHelp
		return m, nil
	}
	if m.servedBack(key) {
		return m, nil
	}
	if m.showHelp {
		if key == "esc" {
			m.showHelp = false
		}
		return m, nil
	}
	if m.leader {
		m.updateLeader(key)
		return m, nil
	}
	if m.showDownloads {
		return m.updateDownloadKeys(msg)
	}
	if m.deleteArmed && key != "d" {
		m.deleteArmed = false
	}
	if key == "m" {
		return m.toggleMenu()
	}

	if cmd, ok := m.playbackKey(key); ok {
		return m, cmd
	}
	var cmd tea.Cmd
	switch key {
	case "up", "left", "k", "h":
		m.moveSelection(-1)
	case "down", "right", "j", "l":
		m.moveSelection(1)
	case "ctrl+k":
		m.moveTrack(-1)
	case "ctrl+j":
		m.moveTrack(1)
	case "esc":
		if m.view == rootView {
			return m, m.hideCmd()
		}
		m.goBack()
	case "s":
		m.shuffleCurrent()
	case "/":
		m.beginInput(inputSearch, "/")
	case "u":
		m.showDownloads = true
		cmd = m.refreshDownloads()
	case "y":
		m.beginYouTubeDownload()
	case "n":
		if m.search != "" {
			m.findSearch(1)
		} else if m.view == rootView {
			m.beginInput(inputCreate, "new cassette")
		} else {
			m.status = "press y to download YouTube"
		}
	case "N":
		if m.search != "" {
			m.findSearch(-1)
		}
	case ",":
		m.leader = true
		m.leaderSeq = ""
		m.status = ",rn rename"
	case "d":
		if m.deleteArmed {
			m.deleteArmed = false
			cmd = m.deleteSelected()
		} else {
			m.deleteArmed = true
			m.status = "press d again to delete"
		}
	}
	return m, cmd
}

// toggleMenu shows or hides the cassette menu. musica serve does the same:
// m stays in the Tesseract center and does not ask it to close.
func (m model) toggleMenu() (tea.Model, tea.Cmd) {
	m.showDownloads = false
	m.showHelp = false
	m.showMenu = !m.showMenu
	return m, nil
}

// hideCmd puts the player out of sight and leaves playback going: the
// terminal popup runs MUSICA_HIDE_COMMAND, and musica serve asks the
// Tesseract center to close.
func (m model) hideCmd() tea.Cmd {
	if m.leave != nil {
		return m.leaveCmd()
	}
	return func() tea.Msg { return hideDisplayMsg{err: hideDisplay()} }
}

func (m model) leaveCmd() tea.Cmd {
	leave := m.leave
	return func() tea.Msg {
		leave()
		return nil
	}
}

// playbackKey handles enter and the transport keys. Each one that changes
// ffplay returns its op for the audio lane.
func (m *model) playbackKey(key string) (tea.Cmd, bool) {
	switch key {
	case "enter":
		return m.activateSelected(), true
	case " ":
		m.togglePause()
		return nil, true
	case "f":
		return m.seekCurrent(10*time.Second, "seek +10s"), true
	case "b":
		return m.seekCurrent(-10*time.Second, "seek -10s"), true
	case "r":
		return m.restartCurrent(), true
	case "x":
		return m.stopPlayback("stopped"), true
	default:
		return nil, false
	}
}

func (m *model) updateLeader(key string) {
	m.leaderSeq += key
	switch {
	case m.leaderSeq == "rn":
		m.leader = false
		m.leaderSeq = ""
		m.beginRename()
	case strings.HasPrefix("rn", m.leaderSeq):
		m.status = ",rn rename"
	default:
		m.leader = false
		m.leaderSeq = ""
		m.status = "unknown leader action"
	}
}

func (m model) updateInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.inputKind = ""
		m.input = ""
		m.youtubeURL = ""
		m.status = "cancelled"
		return m, nil
	case "enter":
		cmd := m.commitInput()
		return m, cmd
	case "backspace", "ctrl+h":
		m.input = trimLastRune(m.input)
		return m, nil
	case "ctrl+w":
		m.input = trimPreviousWord(m.input)
		return m, nil
	default:
		if len(msg.Runes) > 0 {
			m.input += string(msg.Runes)
		}
		return m, nil
	}
}

func (m *model) commitInput() tea.Cmd {
	value := strings.TrimSpace(m.input)
	kind := m.inputKind
	m.inputKind = ""
	m.input = ""

	switch kind {
	case inputSearch:
		if value == "" {
			m.status = "empty search"
			return nil
		}
		m.search = value
		if !m.findSearch(1) {
			m.status = "no match: " + value
		}
	case inputCreate:
		m.createPlaylist(value)
	case inputRename:
		m.renameSelected(value)
	case inputYouTubeURL:
		m.commitYouTubeURL(value)
	case inputYouTubeName:
		return m.commitYouTubeName(value)
	}
	return nil
}

func (m *model) commitYouTubeURL(value string) {
	if value == "/exit" {
		m.youtubeURL = ""
		m.status = "cancelled"
		return
	}
	if value == "" {
		m.keepInput(inputYouTubeURL, value, "missing YouTube URL")
		return
	}
	if !library.IsYouTubeURL(value) {
		m.keepInput(inputYouTubeURL, value, "not a YouTube URL")
		return
	}
	m.youtubeURL = value
	m.inputKind = inputYouTubeName
	m.input = ""
	m.status = "song name"
}

func (m *model) commitYouTubeName(value string) tea.Cmd {
	if value == "/exit" {
		m.youtubeURL = ""
		m.status = "cancelled"
		return nil
	}
	name := safeBaseName(value)
	if name == "" {
		m.keepInput(inputYouTubeName, value, "missing song name")
		return nil
	}
	cmd := m.downloadYouTube(name, m.youtubeURL)
	m.youtubeURL = ""
	return cmd
}

func (m *model) keepInput(kind, value, status string) {
	m.inputKind = kind
	m.input = value
	m.status = status
}

func (m *model) reloadRoot() {
	m.playlists = library.ListPlaylists(m.cfg.MusicDir)
	m.rootTracks = library.ListRootTracks(m.cfg.MusicDir)
	m.clampSelection()
}

func (m *model) reloadTracks() {
	if m.currentPlaylist == "" {
		return
	}
	m.tracks = library.ScanPlaylistTracks(m.currentPlaylist)
	m.clampSelection()
}

func (m *model) openTrackContext(trackPath string) {
	dir := filepath.Dir(trackPath)
	m.currentPlaylist = dir
	m.currentPlaylistName = filepath.Base(dir)
	m.tracks = library.ScanPlaylistTracks(dir)
	m.view = tracksView
	m.selected = 0
	for i, track := range m.tracks {
		if samePath(track.Path, trackPath) {
			m.selected = i
			break
		}
	}
}

func (m *model) moveSelection(delta int) {
	count := m.itemCount()
	if count == 0 {
		return
	}
	m.selected = (m.selected + delta + count) % count
}

func (m *model) moveTrack(delta int) {
	if m.view != tracksView || len(m.tracks) < 2 {
		return
	}
	next := m.selected + delta
	if next < 0 || next >= len(m.tracks) {
		return
	}
	m.tracks[m.selected], m.tracks[next] = m.tracks[next], m.tracks[m.selected]
	m.selected = next
	m.status = "moved"
}

func (m *model) activateSelected() tea.Cmd {
	switch m.view {
	case rootView:
		if m.selected < len(m.playlists) {
			playlist := m.playlists[m.selected]
			m.cassetteSelected = m.selected
			m.cassetteSelectedPath = playlist.Path
			m.currentPlaylist = playlist.Path
			m.currentPlaylistName = playlist.Name
			m.tracks = library.ScanPlaylistTracks(playlist.Path)
			m.view = tracksView
			m.selected = 0
			m.status = "cassette " + playlist.Name
			return nil
		}
		rootTrackIndex := m.selected - len(m.playlists)
		if rootTrackIndex >= 0 && rootTrackIndex < len(m.rootTracks) {
			m.queue = append([]library.Track(nil), m.rootTracks...)
			m.currentIndex = rootTrackIndex
			return m.playPath(m.rootTracks[rootTrackIndex].Path, 0)
		}
	case tracksView:
		if m.selected >= 0 && m.selected < len(m.tracks) {
			m.queue = append([]library.Track(nil), m.tracks...)
			m.currentIndex = m.selected
			return m.playPath(m.tracks[m.selected].Path, 0)
		}
	}
	return nil
}

func (m *model) goBack() {
	if m.view == tracksView {
		m.view = rootView
		m.currentPlaylist = ""
		m.currentPlaylistName = ""
		m.tracks = nil
		m.restoreCassetteSelection()
		m.clampSelection()
		m.status = "cassettes"
	}
}

// servedBack is h in musica serve from a cassette's tracks, help, or the
// download view: it shows the cassette list with the open cassette selected.
// Anywhere else, on the cassette list or over the picture, it reports false
// and h moves the selection as it does in the TUI.
func (m *model) servedBack(key string) bool {
	if key != "h" || m.leave == nil || m.leader {
		return false
	}
	if !m.showHelp && !m.showDownloads && (!m.showMenu || m.view != tracksView) {
		return false
	}
	m.showHelp = false
	m.showDownloads = false
	m.deleteArmed = false
	m.goBack()
	m.showMenu = true
	return true
}

func (m *model) restoreCassetteSelection() {
	for index, playlist := range m.playlists {
		if samePath(playlist.Path, m.cassetteSelectedPath) {
			m.selected = index
			m.cassetteSelected = index
			return
		}
	}
	m.selected = m.cassetteSelected
}

// playPath starts path on the audio lane. The track change fade starts from
// the levels showing now.
func (m *model) playPath(path string, offset time.Duration) tea.Cmd {
	return m.startTrack(path, offset, "playing")
}

// armResume opens the noted track and starts it from Init at its offset.
// A missing file is skipped.
func (m *model) armResume(note session.Resume) {
	if note.Path == "" {
		return
	}
	info, err := os.Stat(note.Path)
	if err != nil || info.IsDir() {
		return
	}
	m.openTrackContext(note.Path)
	m.showMenu = note.Menu
	m.showHelp = false
	m.showDownloads = false
	m.resumeArmed = true
	m.resumePath = note.Path
	m.resumeOffset = note.Offset
	m.resumePaused = note.Paused
	if note.Paused {
		m.status = "paused"
	}
}

// captureResume is the place a reload continues from: the loaded track, how
// far in, whether it was paused, and whether the cassette menu was open.
func (m model) captureResume() session.Resume {
	path, at := "", time.Duration(0)
	paused := true
	if m.player != nil {
		path, at = m.player.State()
		if path != "" {
			paused = m.player.IsPaused() || !m.player.IsPlaying()
		}
	}
	if path == "" {
		path = m.held
		at = 0
		paused = true
	}
	if path == "" {
		path = m.currentPath
	}
	return session.Resume{Path: path, Offset: at, Paused: paused, Menu: m.showMenu}
}

// startResume starts the reloaded track at its offset. A paused track starts
// paused, with the menu as armResume left it.
func (m *model) startResume(msg resumeMsg) tea.Cmd {
	if msg.paused {
		return m.queueAudio(audioOp{
			path: msg.path, offset: msg.offset, pause: true,
			pending: "starting…", done: "paused",
		})
	}
	return m.startTrack(msg.path, msg.offset, "playing")
}

func (m *model) startTrack(path string, offset time.Duration, done string) tea.Cmd {
	if m.player == nil {
		m.status = "player unavailable"
		return nil
	}
	levels, peaks := m.visualizerLevels()
	return m.queueAudio(audioOp{
		path:    path,
		offset:  offset,
		pending: "starting…",
		done:    done,
		from:    m.currentPath,
		levels:  levels,
		peaks:   peaks,
	})
}

func (m *model) playNext() tea.Cmd {
	if len(m.queue) == 0 {
		return m.queueAudio(audioOp{})
	}
	next := m.currentIndex + 1
	if next >= len(m.queue) {
		return m.queueAudio(audioOp{done: "queue finished"})
	}
	m.currentIndex = next
	return m.playPath(m.queue[next].Path, 0)
}

// seekCurrent starts the playing track again delta from where it is. While
// an op runs the position is not known, so the key does nothing.
func (m *model) seekCurrent(delta time.Duration, done string) tea.Cmd {
	if m.audioBusy {
		return nil
	}
	m.status = done
	if m.player == nil {
		return nil
	}
	path, elapsed := m.player.State()
	if path == "" {
		return nil
	}
	offset := elapsed + delta
	if offset < 0 {
		offset = 0
	}
	return m.queueAudio(audioOp{path: path, offset: offset, seek: true, pending: "seeking…", done: done})
}

func (m *model) restartCurrent() tea.Cmd {
	if m.audioBusy {
		return nil
	}
	m.status = "restart"
	if m.player == nil {
		return nil
	}
	path, _ := m.player.State()
	if path == "" {
		return nil
	}
	return m.startTrack(path, 0, "restart")
}

// stopPlayback paints "stopping…" and stops ffplay and ffmpeg on the audio
// lane; status shows once they have exited.
func (m *model) stopPlayback(status string) tea.Cmd {
	return m.queueAudio(audioOp{pending: "stopping…", done: status})
}

func (m *model) shuffleCurrent() {
	switch m.view {
	case rootView:
		if len(m.rootTracks) < 2 {
			m.status = "nothing to shuffle"
			return
		}
		shuffleTracks(m.rootTracks)
	case tracksView:
		if len(m.tracks) < 2 {
			m.status = "nothing to shuffle"
			return
		}
		shuffleTracks(m.tracks)
	}
	m.selected = 0
	m.status = "shuffled"
}

func shuffleTracks(tracks []library.Track) {
	for i := len(tracks) - 1; i > 0; i-- {
		j := int(time.Now().UnixNano() % int64(i+1))
		tracks[i], tracks[j] = tracks[j], tracks[i]
	}
}

func (m *model) beginInput(kind, placeholder string) {
	m.inputKind = kind
	m.input = ""
	m.status = placeholder
}

func (m *model) beginYouTubeDownload() {
	if m.view != tracksView || m.currentPlaylist == "" {
		m.status = "open a cassette first"
		return
	}
	m.inputKind = inputYouTubeURL
	m.input = ""
	m.youtubeURL = ""
	m.status = "YouTube URL"
}

func (m *model) beginRename() {
	path, ok := m.selectedPath()
	if !ok {
		m.status = "nothing to rename"
		return
	}
	m.inputKind = inputRename
	m.input = filepath.Base(path)
	m.status = "rename"
}

func (m *model) createPlaylist(name string) {
	name = safeBaseName(name)
	if name == "" {
		m.status = "missing cassette name"
		return
	}
	path := filepath.Join(m.cfg.MusicDir, name)
	if err := os.MkdirAll(path, 0o755); err != nil {
		m.status = err.Error()
		return
	}
	m.reloadRoot()
	m.status = "created cassette " + name
}

func (m *model) renameSelected(name string) {
	name = safeBaseName(name)
	if name == "" {
		m.status = "missing name"
		return
	}
	path, ok := m.selectedPath()
	if !ok {
		m.status = "nothing to rename"
		return
	}
	if m.view == tracksView && filepath.Ext(name) == "" {
		name += filepath.Ext(path)
	}
	target := filepath.Join(filepath.Dir(path), name)
	if samePath(path, target) {
		m.status = "unchanged"
		return
	}
	if err := os.Rename(path, target); err != nil {
		m.status = err.Error()
		return
	}
	if m.view == rootView {
		m.reloadRoot()
	} else {
		m.reloadTracks()
	}
	m.status = "renamed"
}

func (m *model) downloadYouTube(name, rawURL string) tea.Cmd {
	if m.view != tracksView || m.currentPlaylist == "" {
		m.status = "open a cassette first"
		return nil
	}
	if name == "" {
		m.status = "missing song name"
		return nil
	}
	return m.queueYouTube(m.currentPlaylist, name, rawURL)
}

func (m *model) deleteSelected() tea.Cmd {
	path, ok := m.selectedPath()
	if !ok {
		m.status = "nothing to delete"
		return nil
	}
	info, err := os.Stat(path)
	if err != nil {
		m.status = err.Error()
		return nil
	}
	kind := "track"
	if info.IsDir() {
		kind = "cassette"
		err = os.RemoveAll(path)
	} else {
		err = os.Remove(path)
	}
	if err != nil {
		m.status = err.Error()
		return nil
	}
	if m.view == rootView {
		m.reloadRoot()
	} else {
		m.reloadTracks()
	}
	m.status = "deleted " + kind + " " + filepath.Base(path)
	if samePath(m.currentPath, path) {
		return m.stopPlayback(m.status)
	}
	return nil
}

func (m *model) findSearch(direction int) bool {
	query := strings.TrimSpace(strings.ToLower(m.search))
	if query == "" {
		return false
	}
	count := m.itemCount()
	if count == 0 {
		return false
	}
	start := m.selected
	for step := 0; step < count; step++ {
		index := start + direction*(step+1)
		for index < 0 {
			index += count
		}
		index %= count
		if matchName(strings.ToLower(m.itemName(index)), query) {
			m.selected = index
			m.status = "match " + m.itemName(index)
			return true
		}
	}
	if matchName(strings.ToLower(m.itemName(m.selected)), query) {
		m.status = "match " + m.itemName(m.selected)
		return true
	}
	return false
}

func matchName(name, query string) bool {
	if strings.HasPrefix(name, query) || strings.Contains(name, query) {
		return true
	}
	position := 0
	for _, ch := range query {
		found := false
		for position < len(name) {
			current, size := utf8.DecodeRuneInString(name[position:])
			position += size
			if current == ch {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

func (m model) itemCount() int {
	if m.view == tracksView {
		return len(m.tracks)
	}
	return len(m.playlists) + len(m.rootTracks)
}

func (m model) itemName(index int) string {
	if m.view == tracksView {
		if index >= 0 && index < len(m.tracks) {
			return m.tracks[index].Name
		}
		return ""
	}
	if index >= 0 && index < len(m.playlists) {
		return m.playlists[index].Name
	}
	rootTrackIndex := index - len(m.playlists)
	if rootTrackIndex >= 0 && rootTrackIndex < len(m.rootTracks) {
		return m.rootTracks[rootTrackIndex].Name
	}
	return ""
}

func (m model) itemPath(index int) (string, bool) {
	if m.view == tracksView {
		if index >= 0 && index < len(m.tracks) {
			return m.tracks[index].Path, true
		}
		return "", false
	}
	if index >= 0 && index < len(m.playlists) {
		return m.playlists[index].Path, true
	}
	rootTrackIndex := index - len(m.playlists)
	if rootTrackIndex >= 0 && rootTrackIndex < len(m.rootTracks) {
		return m.rootTracks[rootTrackIndex].Path, true
	}
	return "", false
}

func (m model) selectedPath() (string, bool) {
	if m.view == tracksView {
		if m.selected >= 0 && m.selected < len(m.tracks) {
			return m.tracks[m.selected].Path, true
		}
		return "", false
	}
	if m.selected >= 0 && m.selected < len(m.playlists) {
		return m.playlists[m.selected].Path, true
	}
	rootTrackIndex := m.selected - len(m.playlists)
	if rootTrackIndex >= 0 && rootTrackIndex < len(m.rootTracks) {
		return m.rootTracks[rootTrackIndex].Path, true
	}
	return "", false
}

func (m *model) clampSelection() {
	count := m.itemCount()
	if count == 0 {
		m.selected = 0
		return
	}
	if m.selected < 0 {
		m.selected = 0
	}
	if m.selected >= count {
		m.selected = count - 1
	}
}

func (m model) View() string {
	if m.showHelp {
		return m.fitHeight(m.renderHelp())
	}
	if m.isYouTubeInput() {
		return m.renderYouTubeModal()
	}
	if m.showDownloads {
		return m.fitHeight(m.renderDownloads())
	}
	if m.showMenu {
		return strings.Join(m.renderMenuOverlayLines(), "\n")
	}
	return strings.Join(m.renderNormalViewLines(), "\n")
}

func (m model) renderNormalViewLines() []string {
	statusLine, showStatus := m.renderStatusLine()
	reservedLines := 0
	if showStatus {
		reservedLines++
	}
	contentLines := 0
	if m.height > reservedLines {
		contentLines = m.height - reservedLines
	}

	lines := splitRenderedLines(m.renderPlayerDeck())
	if contentLines > 0 {
		if len(lines) > contentLines {
			lines = lines[:contentLines]
		}
		for len(lines) < contentLines {
			lines = append(lines, "")
		}
	}
	if showStatus {
		lines = append(lines, statusLine)
	}
	return m.fitBottom(lines)
}

func (m model) renderStatusLine() (string, bool) {
	if m.inputKind != "" && !m.isYouTubeInput() {
		return statusStyle.Render(m.inputPrompt()), true
	}
	if isQuietStatus(m.status) {
		if line := m.downloadStatus(); line != "" {
			return statusStyle.Render(line), true
		}
		return "", false
	}
	return statusStyle.Render(m.status), true
}

func isQuietStatus(status string) bool {
	switch strings.TrimSpace(status) {
	case "", "ready", "playing", "paused":
		return true
	default:
		return false
	}
}

func (m model) fitBottom(lines []string) []string {
	if m.height <= 0 {
		return lines
	}
	if len(lines) > m.height {
		lines = lines[len(lines)-m.height:]
	}
	for len(lines) < m.height {
		lines = append([]string{""}, lines...)
	}
	return lines
}

func splitRenderedLines(value string) []string {
	if value == "" {
		return nil
	}
	return strings.Split(value, "\n")
}

func (m model) renderHelp() string {
	lines := []string{
		titleStyle.Render("musica shortcuts"),
		"",
		mutedStyle.Render("h/l, j/k, arrows    move selection"),
		mutedStyle.Render("enter               open cassette or play track"),
		mutedStyle.Render("esc                 return to cassettes; from cassettes quit"),
		mutedStyle.Render("m                   toggle cassette and track menu"),
		mutedStyle.Render("space               pause and open the menu; resume keeps it open"),
		mutedStyle.Render("f / b               seek forward/backward 10 seconds"),
		mutedStyle.Render("r                   restart current track"),
		mutedStyle.Render("x                   stop playback"),
		mutedStyle.Render("s                   shuffle current track list"),
		mutedStyle.Render("/                   search"),
		mutedStyle.Render("n / N               next/previous search result; n creates a cassette when no search is active"),
		mutedStyle.Render("u                   show downloads"),
		mutedStyle.Render("y                   download YouTube audio into the open cassette"),
		mutedStyle.Render(",rn                 rename selected cassette or track"),
		mutedStyle.Render("dd                  delete selected cassette or track"),
		mutedStyle.Render("?                   toggle this view"),
		mutedStyle.Render("q                   stop playback and quit"),
		mutedStyle.Render("ctrl+c              quit"),
	}
	return strings.Join(lines, "\n")
}

func (m model) isYouTubeInput() bool {
	return m.inputKind == inputYouTubeURL || m.inputKind == inputYouTubeName
}

func (m model) renderYouTubeModal() string {
	width := maxInt(1, m.width)
	height := maxInt(1, m.height)
	modalWidth := minInt(64, maxInt(32, width-8))
	if modalWidth > width {
		modalWidth = width
	}
	if modalWidth < 8 {
		return m.fitHeight(m.inputPrompt())
	}

	lines := []string{
		"YouTube download",
		m.youtubeModalLabel(),
		"> " + m.input,
	}
	if m.inputKind == inputYouTubeName && m.youtubeURL != "" {
		lines = append(lines, "URL "+m.youtubeURL)
	}
	if m.status != "" && m.status != "YouTube URL" && m.status != "song name" {
		lines = append(lines, m.status)
	}

	box := titleStyle.Render(boxedLines(lines, modalWidth))
	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, box)
}

func (m model) youtubeModalLabel() string {
	if m.inputKind == inputYouTubeName {
		return "Song name"
	}
	return "YouTube URL"
}

func (m model) renderMenuModal() string {
	return strings.Join(m.renderMenuOverlayLines(), "\n")
}

func (m model) renderMenuOverlayLines() []string {
	width := maxInt(1, m.width)
	height := maxInt(1, m.height)
	cells := m.visualizerFrameCells(width, height)
	modalWidth := minInt(menuModalMaxWidth, maxInt(menuModalMinWidth, width-8))
	if modalWidth > width {
		modalWidth = width
	}
	if modalWidth < 8 {
		overlayText(cells, m.menuDialLine(width), 0, height/2)
		return renderVisualizerCellRows(cells, height)
	}

	box := m.renderMenuBox(modalWidth)
	x := maxInt(0, (width-modalWidth)/2)
	y := maxInt(0, (height-lipgloss.Height(box))/2)
	overlayText(cells, box, x, y)
	return renderVisualizerCellRows(cells, height)
}

func (m model) renderMenuBox(modalWidth int) string {
	contentWidth := modalWidth - 4
	lines := []string{m.menuDialLine(contentWidth)}
	if status, ok := m.menuStatusLine(); ok {
		lines = append(lines, status)
	}
	return boxedLines(lines, modalWidth)
}

func (m model) menuStatusLine() (string, bool) {
	if m.inputKind != "" && !m.isYouTubeInput() {
		return m.inputPrompt(), true
	}
	return "", false
}

func (m model) menuDialLine(width int) string {
	if width < 1 {
		return ""
	}
	count := m.itemCount()
	if count == 0 {
		message := "No cassettes or root tracks found."
		if m.view == tracksView {
			message = "No tracks in this cassette."
		}
		return padRight(truncateMiddle(message, width), width)
	}

	selected := m.itemFieldText(m.selected, true)
	playing := m.playingFieldText()
	if width < 28 {
		return padRight(truncateMiddle(selected, width), width)
	}

	rightWidth := 12
	if playing != "idle" && width >= 64 {
		rightWidth = playingColumnWidth(len(playing), width)
	}
	leftWidth := width - rightWidth - 2
	if leftWidth < 1 {
		return padRight(truncateMiddle(selected, width), width)
	}
	shown := playing
	if playing != "idle" {
		shown = fitPlaying(playing, rightWidth)
	}
	return padRight(truncateMiddle(selected, leftWidth), leftWidth) + "  " + padRight(shown, rightWidth)
}

func playingColumnWidth(playingLen, width int) int {
	const selectedMin = 36
	maxRight := width - selectedMin - 2
	if maxRight < 20 {
		maxRight = 20
	}
	if playingLen < 20 {
		return 20
	}
	if playingLen > maxRight {
		return maxRight
	}
	return playingLen
}

func fitPlaying(playing string, width int) string {
	if width <= 0 || len(playing) <= width {
		return playing
	}
	state, rest, ok := strings.Cut(playing, " ")
	clock, title, split := strings.Cut(rest, " ")
	if !ok || !split {
		return truncateEnd(playing, width)
	}
	prefix := state + " " + clock + " "
	if len(prefix) >= width {
		return truncateEnd(playing, width)
	}
	return prefix + truncateEnd(title, width-len(prefix))
}

func truncateEnd(value string, limit int) string {
	if limit <= 0 {
		return ""
	}
	if len(value) <= limit {
		return value
	}
	if limit <= 3 {
		return value[:limit]
	}
	return value[:limit-3] + "..."
}

func boxedLines(lines []string, width int) string {
	if width < 4 {
		return strings.Join(lines, "\n")
	}
	contentWidth := width - 4
	border := "+" + strings.Repeat("-", width-2) + "+"
	out := []string{border}
	for _, line := range lines {
		out = append(out, "| "+padRight(truncateMiddle(line, contentWidth), contentWidth)+" |")
	}
	out = append(out, border)
	return strings.Join(out, "\n")
}

func (m model) visualizerFrameRows(width, height int) []string {
	levels, peaks := m.visualizerLevels()
	return visualizer.RenderFrameWithIntensity(maxInt(1, width), maxInt(1, height), levels, peaks, m.visualizerFrame, 1)
}

func (m model) visualizerFrameCells(width, height int) [][]visualizer.Cell {
	levels, peaks := m.visualizerLevels()
	return visualizer.RenderCellsWithIntensity(maxInt(1, width), maxInt(1, height), levels, peaks, m.visualizerFrame, 1)
}

func (m model) visualizerLevels() ([]float64, []float64) {
	if m.visualizer == nil {
		return nil, nil
	}
	levels := m.visualizer.Levels()
	peaks := m.visualizer.Peaks()
	if m.visualizerTransitionActive {
		return m.visualizerTransitionSignal(levels, peaks)
	}
	return scaleVisualizerSignal(levels, peaks, m.visualizerStartupScale())
}

func (m *model) beginVisualizerTransition(levels []float64, peaks []float64) {
	m.visualizerTransitionActive = true
	m.visualizerTransitionStartFrame = m.visualizerFrame
	m.visualizerTransitionLevels = append([]float64(nil), levels...)
	m.visualizerTransitionPeaks = append([]float64(nil), peaks...)
}

func (m *model) clearExpiredVisualizerTransition() {
	if !m.visualizerTransitionActive {
		return
	}
	elapsed := m.visualizerFrame - m.visualizerTransitionStartFrame
	totalFrames := visualizerTrackChangeDownFrames(m.cfg.FPS) + visualizerTrackChangeUpFrames(m.cfg.FPS)
	if elapsed >= totalFrames {
		m.clearVisualizerTransition()
	}
}

func (m *model) clearVisualizerTransition() {
	m.visualizerTransitionActive = false
	m.visualizerTransitionStartFrame = 0
	m.visualizerTransitionLevels = nil
	m.visualizerTransitionPeaks = nil
}

func (m model) visualizerTransitionSignal(levels []float64, peaks []float64) ([]float64, []float64) {
	elapsed := m.visualizerFrame - m.visualizerTransitionStartFrame
	if elapsed < 0 {
		elapsed = 0
	}
	downFrames := visualizerTrackChangeDownFrames(m.cfg.FPS)
	upFrames := visualizerTrackChangeUpFrames(m.cfg.FPS)
	if elapsed < downFrames {
		progress := smootherstep(float64(elapsed) / float64(downFrames))
		return scaleVisualizerSignal(m.visualizerTransitionLevels, m.visualizerTransitionPeaks, 1-progress)
	}
	if elapsed >= downFrames+upFrames {
		return levels, peaks
	}
	progress := smootherstep(float64(elapsed-downFrames) / float64(upFrames))
	scale := visualizerStartupIntensityFloor + progress*(1-visualizerStartupIntensityFloor)
	return scaleVisualizerSignal(levels, peaks, scale)
}

func (m model) visualizerStartupScale() float64 {
	if m.currentPath == "" {
		return 1
	}
	easeFrames := visualizerStartupEaseFrames(m.cfg.FPS)
	elapsed := m.visualizerFrame - m.visualizerStartFrame
	if elapsed >= easeFrames {
		return 1
	}
	if elapsed < 0 {
		elapsed = 0
	}
	t := float64(elapsed) / float64(easeFrames)
	eased := smootherstep(t)
	return visualizerStartupIntensityFloor + eased*(1-visualizerStartupIntensityFloor)
}

func visualizerStartupEaseFrames(fps int) int {
	if fps < 5 {
		fps = config.DefaultFPS
	}
	return fps * visualizerStartupEaseSeconds
}

func visualizerTrackChangeDownFrames(fps int) int {
	if fps < 5 {
		fps = config.DefaultFPS
	}
	return maxInt(1, fps*visualizerTrackChangeDownSeconds)
}

func visualizerTrackChangeUpFrames(fps int) int {
	if fps < 5 {
		fps = config.DefaultFPS
	}
	return maxInt(1, fps*visualizerTrackChangeUpSeconds)
}

func smootherstep(t float64) float64 {
	if t <= 0 {
		return 0
	}
	if t >= 1 {
		return 1
	}
	return t * t * t * (t*(t*6-15) + 10)
}

func scaleVisualizerSignal(levels []float64, peaks []float64, scale float64) ([]float64, []float64) {
	if scale >= 1 {
		return levels, peaks
	}
	if scale < 0 {
		scale = 0
	}
	scaledLevels := scaleFloatSlice(levels, scale)
	scaledPeaks := scaleFloatSlice(peaks, scale)
	return scaledLevels, scaledPeaks
}

func scaleFloatSlice(values []float64, scale float64) []float64 {
	if len(values) == 0 {
		return values
	}
	scaled := make([]float64, len(values))
	for i, value := range values {
		scaled[i] = value * scale
	}
	return scaled
}

func (m model) visualizerFieldHeight() int {
	height := m.visualizerHeight()
	if height < 1 && m.height > 0 {
		height = m.height
	}
	return maxInt(12, height)
}

func (m model) renderCassetteDialRow(index int) string {
	width := m.width
	if width < 1 {
		width = 1
	}
	line := m.itemFieldText(index, index == m.selected)
	line = padRight(truncateMiddle(line, width), width)

	if index == m.selected {
		return cassetteSelectedStyle.Render(line)
	}

	path, ok := m.itemPath(index)
	isPlaying := ok && samePath(m.currentPath, path)
	if isPlaying {
		return cassettePlayingStyle.Render(line)
	}

	prefix := "  "
	prefixEnd := minInt(len(line), len(prefix))
	slotEnd := minInt(len(line), len(prefix)+len("["+cassetteSlot(index)+"]"))
	return cassetteRowStyle.Render(line[:prefixEnd]) +
		cassetteSlotStyle.Render(line[prefixEnd:slotEnd]) +
		cassetteRowStyle.Render(line[slotEnd:])
}

func (m model) itemFieldText(index int, selected bool) string {
	slot := cassetteSlot(index)
	subject, isPlaying := m.itemSubject(index)
	if selected {
		position := fmt.Sprintf("%02d/%02d", index+1, maxInt(1, m.itemCount()))
		return fmt.Sprintf("[%s] [%s] %s", slot, position, subject)
	}

	prefix := "  "
	if isPlaying {
		prefix = "* "
	}
	return fmt.Sprintf("%s[%s] %s", prefix, slot, subject)
}

func (m model) itemSubject(index int) (string, bool) {
	name := m.itemName(index)
	if name == "" {
		name = "(empty)"
	}

	kind := "TRACK"
	path, ok := m.itemPath(index)
	if m.view == rootView && index >= 0 && index < len(m.playlists) {
		kind = "CASSETTE"
	}
	isPlaying := ok && samePath(m.currentPath, path)
	if isPlaying {
		kind = "LIVE"
	}

	subject := fmt.Sprintf("%s  %s", kind, name)
	if m.view == tracksView && m.currentPlaylistName != "" {
		subject = fmt.Sprintf("CASSETTE %s  %s  %s", m.currentPlaylistName, kind, name)
	}
	return subject, isPlaying
}

func (m model) playingFieldText() string {
	if m.audioPending != "" {
		return m.audioPending
	}
	if m.player == nil {
		return "idle"
	}
	path, elapsed := m.player.State()
	if path == "" {
		return "idle"
	}
	state := "playing"
	if m.player.IsPaused() {
		state = "paused"
	}
	return state + " " + library.FormatDuration(elapsed) + " " + library.DisplayName(path)
}

func cassetteSlot(index int) string {
	if index < 0 {
		index = 0
	}
	letter := byte('A' + (index/10)%26)
	number := index%10 + 1
	return fmt.Sprintf("%c%02d", letter, number)
}

func (m model) renderPlayerDeck() string {
	width := m.width
	if width < 1 {
		width = 1
	}
	visHeight := m.visualizerHeight()
	if visHeight == 0 {
		return ""
	}

	fieldHeight := m.visualizerFieldHeight()
	frameCells := m.visualizerFrameCells(width, fieldHeight)
	end := minInt(len(frameCells), visHeight)
	bars := frameCells[:end]
	lines := make([]string, 0, len(bars))
	for row, cells := range bars {
		lines = append(lines, renderVisualizerCells(cells, row, fieldHeight))
	}
	return strings.Join(lines, "\n")
}

func renderVisualizerCellRows(rows [][]visualizer.Cell, total int) []string {
	lines := make([]string, 0, len(rows))
	for row, cells := range rows {
		lines = append(lines, renderVisualizerCells(cells, row, total))
	}
	return lines
}

func overlayText(cells [][]visualizer.Cell, text string, x, y int) {
	if len(cells) == 0 {
		return
	}
	lines := strings.Split(text, "\n")
	for row, line := range lines {
		targetY := y + row
		if targetY < 0 || targetY >= len(cells) {
			continue
		}
		for col := 0; col < len(line); col++ {
			targetX := x + col
			if targetX < 0 || targetX >= len(cells[targetY]) {
				continue
			}
			glyph := line[col]
			tone := visualizer.ToneRainBright
			if glyph == ' ' {
				tone = visualizer.ToneDark
			} else if glyph == '+' || glyph == '-' || glyph == '|' {
				tone = visualizer.ToneRainSoft
			}
			cells[targetY][targetX] = visualizer.Cell{Glyph: glyph, Tone: tone}
		}
	}
}

func renderVisualizerCells(cells []visualizer.Cell, row, total int) string {
	if len(cells) == 0 {
		return ""
	}
	var rendered strings.Builder
	start := 0
	tone := cells[0].Tone
	for i := 1; i <= len(cells); i++ {
		if i < len(cells) && cells[i].Tone == tone {
			continue
		}
		rendered.WriteString(visualizerCellStyle(tone, row, total).Render(visualizerCellText(cells[start:i])))
		if i < len(cells) {
			start = i
			tone = cells[i].Tone
		}
	}
	return rendered.String()
}

func visualizerCellStyle(tone visualizer.CellTone, row, total int) lipgloss.Style {
	if tone == visualizer.ToneDark {
		return visualizerGrayStyle("240")
	}
	return visualizerRowStyle(row, total)
}

func visualizerCellText(cells []visualizer.Cell) string {
	text := make([]byte, len(cells))
	for i, cell := range cells {
		text[i] = cell.Glyph
	}
	return string(text)
}

func visualizerDarkCoreStyle() lipgloss.Style {
	return visualizerGrayStyle("240")
}

func visualizerRowStyle(_, _ int) lipgloss.Style {
	return visualizerGrayStyle("15")
}

func visualizerGrayStyle(color string) lipgloss.Style {
	return lipgloss.NewStyle().Foreground(lipgloss.Color(color))
}

func (m model) playerDeckLineCount() int {
	return m.visualizerHeight()
}

func (m model) visualizerHeight() int {
	return m.visualizerHeightFor(true)
}

func (m model) visualizerHeightFor(_ bool) int {
	if m.width < 24 {
		return 0
	}
	height := m.height - visualizerChromeLines()
	if height < 3 {
		return 0
	}
	return height
}

func visualizerChromeLines() int {
	return 0
}

func (m model) fitHeight(value string) string {
	if m.height <= 0 {
		return value
	}
	lines := strings.Split(value, "\n")
	if len(lines) > m.height {
		lines = lines[:m.height]
	}
	for len(lines) < m.height {
		lines = append(lines, "")
	}
	return strings.Join(lines, "\n")
}

func fixedLines(lines []string, count int) string {
	if count < 1 {
		count = 1
	}
	if len(lines) > count {
		lines = lines[:count]
	}
	for len(lines) < count {
		lines = append(lines, "")
	}
	return strings.Join(lines, "\n")
}

func (m model) inputPrompt() string {
	prefix := ">"
	switch m.inputKind {
	case inputSearch:
		prefix = "/"
	case inputRename:
		prefix = "rename"
	case inputCreate:
		prefix = "new"
	case inputYouTubeURL:
		prefix = "url"
	case inputYouTubeName:
		prefix = "name"
	}
	return prefix + " " + m.input
}

func safeBaseName(value string) string {
	value = strings.TrimSpace(value)
	value = filepath.Base(value)
	if value == "." || value == string(filepath.Separator) {
		return ""
	}
	return value
}

func trimLastRune(value string) string {
	if value == "" {
		return value
	}
	_, size := utf8.DecodeLastRuneInString(value)
	return value[:len(value)-size]
}

func trimPreviousWord(value string) string {
	value = strings.TrimRightFunc(value, unicode.IsSpace)
	for len(value) > 0 {
		r, size := utf8.DecodeLastRuneInString(value)
		if unicode.IsSpace(r) {
			return strings.TrimRightFunc(value[:len(value)-size], unicode.IsSpace)
		}
		value = value[:len(value)-size]
	}
	return ""
}

func truncateMiddle(value string, limit int) string {
	if limit <= 0 || len(value) <= limit {
		return value
	}
	if limit < 5 {
		return value[:limit]
	}
	half := (limit - 3) / 2
	return value[:half] + "..." + value[len(value)-(limit-3-half):]
}

func samePath(left, right string) bool {
	if left == "" || right == "" {
		return false
	}
	leftAbs, leftErr := filepath.Abs(left)
	rightAbs, rightErr := filepath.Abs(right)
	if leftErr == nil {
		left = leftAbs
	}
	if rightErr == nil {
		right = rightAbs
	}
	return left == right
}

func maxInt(left, right int) int {
	if left > right {
		return left
	}
	return right
}

func minInt(left, right int) int {
	if left < right {
		return left
	}
	return right
}

func padRight(value string, width int) string {
	if width <= 0 || len(value) >= width {
		return value
	}
	return value + strings.Repeat(" ", width-len(value))
}
