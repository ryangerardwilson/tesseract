package app

import (
	"encoding/json"
	"math"
	"time"

	"github.com/ryangerardwilson/tesseract/internal/musica/library"
)

const (
	// frameRows is how many rows of the list a frame carries around the
	// selection.
	frameRows = 9
	// frameBands is how many visualizer bands a frame carries.
	frameBands = 32
)

// frame is what the Tesseract Musica center draws, one per player update.
type frame struct {
	Type      string         `json:"type"`
	State     string         `json:"state"`
	Elapsed   string         `json:"elapsed"`
	Title     string         `json:"title"`
	Status    string         `json:"status"`
	View      string         `json:"view"`
	Cassette  string         `json:"cassette"`
	Menu      bool           `json:"menu"`
	Help      bool           `json:"help"`
	Selected  int            `json:"selected"`
	Count     int            `json:"count"`
	Rows      []frameRow     `json:"rows"`
	Prompt    *framePrompt   `json:"prompt"`
	Downloads frameDownloads `json:"downloads"`
	Levels    []float64      `json:"levels"`
	Peaks     []float64      `json:"peaks"`
	Bass      float64        `json:"bass"`
	Energy    float64        `json:"energy"`
	Thump     float64        `json:"thump"`
}

type frameRow struct {
	Index int    `json:"index"`
	Slot  string `json:"slot"`
	Kind  string `json:"kind"`
	Name  string `json:"name"`
	Live  bool   `json:"live"`
}

type framePrompt struct {
	Kind  string `json:"kind"`
	Label string `json:"label"`
	Text  string `json:"text"`
	URL   string `json:"url,omitempty"`
}

type frameDownloads struct {
	Shown    bool       `json:"shown"`
	Selected int        `json:"selected"`
	Rows     []frameJob `json:"rows"`
}

type frameJob struct {
	Title   string  `json:"title"`
	Status  string  `json:"status"`
	Percent float64 `json:"percent"`
	Error   string  `json:"error,omitempty"`
}

// promptLabels name each prompt the way the center shows it.
var promptLabels = map[string]string{
	inputSearch:      "SEARCH",
	inputRename:      "RENAME",
	inputCreate:      "NEW CASSETTE",
	inputYouTubeURL:  "YOUTUBE URL",
	inputYouTubeName: "SONG NAME",
}

func (m model) frameLine() []byte {
	line, err := json.Marshal(m.frame())
	if err != nil {
		return eventLine("frame")
	}
	return line
}

func (m model) frame() frame {
	state, elapsed, title := m.nowPlaying()
	levels, peaks := m.visualizerLevels()
	f := frame{
		Type:      "frame",
		State:     state,
		Elapsed:   elapsed,
		Title:     title,
		Status:    m.frameStatus(),
		View:      "cassettes",
		Menu:      m.showMenu,
		Help:      m.showHelp,
		Selected:  m.selected,
		Count:     m.itemCount(),
		Rows:      m.frameRows(),
		Prompt:    m.framePrompt(),
		Downloads: m.frameDownloads(),
		Levels:    bandValues(levels),
		Peaks:     bandValues(peaks),
	}
	if m.view == tracksView {
		f.View = "tracks"
		f.Cassette = m.currentPlaylistName
	}
	f.Bass = roundLevel(meanLevel(f.Levels[:frameBands/5]))
	f.Energy = roundLevel(meanLevel(f.Levels))
	f.Thump = roundLevel(m.thump)
	return f
}

// nowPlaying is the playing state, the elapsed time, and the title. While an
// audio op runs, the state is the op's pending word. A held track shows paused
// with no time, because it was never started.
func (m model) nowPlaying() (string, string, string) {
	path, at := "", time.Duration(0)
	if m.player != nil {
		path, at = m.player.State()
	}
	state := "idle"
	switch {
	case m.audioPending != "":
		state = m.audioPending
	case path == "" && m.held != "":
		state = "paused"
	case path == "":
	case m.player.IsPaused():
		state = "paused"
	default:
		state = "playing"
	}
	if path == "" && m.held != "" {
		return state, "", library.DisplayName(m.held)
	}
	if path == "" {
		return state, "", ""
	}
	return state, library.FormatDuration(at), library.DisplayName(path)
}

// frameStatus is the status line as the TUI shows it: a quiet status gives
// way to a running download.
func (m model) frameStatus() string {
	if isQuietStatus(m.status) {
		return m.downloadStatus()
	}
	return m.status
}

// frameRows are up to frameRows rows of the showing list around the
// selection.
func (m model) frameRows() []frameRow {
	count := m.itemCount()
	start := minInt(maxInt(0, m.selected-frameRows/2), maxInt(0, count-frameRows))
	end := minInt(count, start+frameRows)
	rows := make([]frameRow, 0, maxInt(0, end-start))
	for index := start; index < end; index++ {
		kind := "TRACK"
		if m.view == rootView && index < len(m.playlists) {
			kind = "CASSETTE"
		}
		path, ok := m.itemPath(index)
		rows = append(rows, frameRow{
			Index: index,
			Slot:  cassetteSlot(index),
			Kind:  kind,
			Name:  m.itemName(index),
			Live:  ok && samePath(m.currentPath, path),
		})
	}
	return rows
}

func (m model) framePrompt() *framePrompt {
	if m.inputKind == "" {
		return nil
	}
	prompt := &framePrompt{Kind: m.inputKind, Label: promptLabels[m.inputKind], Text: m.input}
	if m.inputKind == inputYouTubeName {
		prompt.URL = m.youtubeURL
	}
	return prompt
}

func (m model) frameDownloads() frameDownloads {
	rows := m.downloadRows()
	jobs := make([]frameJob, 0, len(rows))
	for _, job := range rows {
		jobs = append(jobs, frameJob{
			Title:   job.Title,
			Status:  job.Status,
			Percent: math.Round(job.Percent*10) / 10,
			Error:   job.Error,
		})
	}
	return frameDownloads{Shown: m.showDownloads, Selected: m.downloadIndex, Rows: jobs}
}

// bandValues is frameBands values from 0 to 1, rounded to three places; no
// signal is silence.
func bandValues(values []float64) []float64 {
	out := make([]float64, frameBands)
	for i := range out {
		if i < len(values) {
			out[i] = roundLevel(values[i])
		}
	}
	return out
}

func roundLevel(value float64) float64 {
	if math.IsNaN(value) {
		return 0
	}
	return math.Round(math.Min(1, math.Max(0, value))*1000) / 1000
}

func meanLevel(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	total := 0.0
	for _, value := range values {
		total += value
	}
	return total / float64(len(values))
}
