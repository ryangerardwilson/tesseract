package app

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/ryangerardwilson/tesseract/internal/musica/visualizer"
)

// audioOp is one change to the ffplay and ffmpeg processes. Starting a track
// stops the one before, and a stop waits up to about a second for both to
// exit, so ops run off the update loop one at a time. While one runs, the
// newest request waits and replaces an older waiting one: only the last
// request decides what plays.
type audioOp struct {
	path    string // the track to start; "" stops playback
	offset  time.Duration
	pause   bool   // start the track and pause it, keeping the menu
	seek    bool   // a seek keeps the menu and the visualizer ease as they are
	pending string // the status painted while the op runs; "" paints nothing
	done    string // the status once it lands; "" leaves the status alone
	from    string // the track playing when the op was asked for
	levels  []float64
	peaks   []float64
}

type audioDoneMsg struct {
	op  audioOp
	err error
}

// queueAudio paints op's pending status and runs it, or keeps it as the next
// op while another runs. A play or a stop replaces a held track.
func (m *model) queueAudio(op audioOp) tea.Cmd {
	m.held = ""
	m.audioPending = op.pending
	if op.pending != "" {
		m.status = op.pending
	}
	if m.audioBusy {
		m.audioNext = op
		m.audioWaiting = true
		return nil
	}
	m.audioBusy = true
	return runAudio(m.player, m.visualizer, op)
}

func runAudio(p audioPlayer, v *visualizer.Analyzer, op audioOp) tea.Cmd {
	return func() tea.Msg {
		if op.path == "" {
			stopAudio(p, v)
			return audioDoneMsg{op: op}
		}
		if err := p.Play(op.path, op.offset); err != nil {
			return audioDoneMsg{op: op, err: err}
		}
		if op.pause {
			p.TogglePause()
		}
		if v != nil {
			_ = v.Start(op.path, op.offset)
			if op.pause {
				v.SetPaused(true)
			}
		}
		return audioDoneMsg{op: op}
	}
}

func stopAudio(p audioPlayer, v *visualizer.Analyzer) {
	if p != nil {
		p.Stop()
	}
	if v != nil {
		v.Stop()
	}
}

// finishAudio applies the op that ran and starts the one waiting. A quit that
// came meanwhile stops playback and then ends the program.
func (m model) finishAudio(msg audioDoneMsg) (tea.Model, tea.Cmd) {
	m.audioBusy = false
	if m.quitting {
		return m, m.closeCmd()
	}
	m.applyAudio(msg)
	if !m.audioWaiting {
		m.audioPending = ""
		return m, nil
	}
	next := m.audioNext
	m.audioWaiting = false
	m.audioNext = audioOp{}
	cmd := m.queueAudio(next)
	return m, cmd
}

func (m *model) applyAudio(msg audioDoneMsg) {
	op := msg.op
	if msg.err != nil {
		m.status = msg.err.Error()
		return
	}
	m.currentPath = op.path
	if op.path == "" {
		m.clearVisualizerTransition()
	} else if op.pause {
		m.showMenu = true
	} else if !op.seek {
		m.showMenu = false
		m.visualizerStartFrame = m.visualizerFrame
		if op.from != "" && !samePath(op.from, op.path) {
			m.beginVisualizerTransition(op.levels, op.peaks)
		} else {
			m.clearVisualizerTransition()
		}
	}
	if op.done != "" {
		m.status = op.done
	}
}

// trackEnded reports that the current track stopped on its own. It is false
// while an op runs, because a starting track is briefly not playing.
func (m model) trackEnded() bool {
	return m.currentPath != "" && m.player != nil && !m.audioBusy && !m.quitting && !m.player.IsPlaying()
}

// quit paints "stopping…" and ends the session once playback has stopped. An
// op that is running lands first, so no ffplay it starts outlives the screen.
func (m model) quit() (tea.Model, tea.Cmd) {
	m.quitting = true
	m.audioWaiting = false
	m.audioNext = audioOp{}
	m.audioPending = "stopping…"
	m.status = "stopping…"
	if m.audioBusy {
		return m, nil
	}
	return m, m.closeCmd()
}

// closeCmd stops playback off the update loop and then ends the program.
func (m model) closeCmd() tea.Cmd {
	p, v := m.player, m.visualizer
	return func() tea.Msg {
		stopAudio(p, v)
		return tea.QuitMsg{}
	}
}

// close stops playback after the program has ended.
func (m model) close() {
	stopAudio(m.player, m.visualizer)
}
