package app

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/ryangerardwilson/tesseract/internal/musica/download"
	"github.com/ryangerardwilson/tesseract/internal/musica/store"
)

var enqueueDownload = func(playlistDir, filename, rawURL string) error {
	_, err := download.Enqueue(playlistDir, filename, rawURL)
	return err
}

var cancelDownload = download.Cancel

// listDownloads reads the store and ensureWorker starts the detached worker.
// Tests replace them.
var (
	listDownloads = download.List
	ensureWorker  = download.EnsureWorker
)

// downloadPollEvery is how often the download list is read again. The frame
// tick does not read it.
const downloadPollEvery = 500 * time.Millisecond

const (
	opList   = "list"
	opQueue  = "queue"
	opCancel = "cancel"
)

// The download list belongs to the store. model.downloads is the store's last
// answer, replaced whole. A queue or cancel the store has not answered yet is
// a downloadIntent kept beside it, and the rows are derived from both.
type downloadIntent struct {
	key    string // the job id for a cancel, a local key for a new download
	title  string
	cancel bool
}

func (intent downloadIntent) label() string {
	if intent.cancel {
		return "cancelling " + intent.title
	}
	return "queueing " + intent.title
}

// downloadOp is one trip to the store. Ops run one at a time off the update
// loop, and each ends by listing the store when the model tracks downloads.
type downloadOp struct {
	kind  string
	key   string // the intent it answers
	title string
	dir   string
	url   string
	list  bool
}

type downloadPollMsg struct{}

type downloadDoneMsg struct {
	op      downloadOp
	err     error
	jobs    []store.Job
	listErr error
}

func downloadPollCmd() tea.Cmd {
	return tea.Tick(downloadPollEvery, func(time.Time) tea.Msg { return downloadPollMsg{} })
}

func runDownloadOp(op downloadOp) tea.Cmd {
	return func() tea.Msg {
		done := downloadDoneMsg{op: op}
		switch op.kind {
		case opQueue:
			done.err = enqueueDownload(op.dir, op.title, op.url)
		case opCancel:
			done.err = cancelDownload(op.key)
		}
		if !op.list {
			return done
		}
		done.jobs, done.listErr = listDownloads()
		if done.listErr == nil && jobsNeedWorker(done.jobs) {
			ensureWorker()
		}
		return done
	}
}

// pollDownloads reads the list again and books the next read.
func (m model) pollDownloads() (tea.Model, tea.Cmd) {
	if !m.trackDownloads {
		return m, nil
	}
	cmd := m.refreshDownloads()
	return m, tea.Batch(cmd, downloadPollCmd())
}

func (m *model) refreshDownloads() tea.Cmd {
	return m.queueDownloadOp(downloadOp{kind: opList})
}

// queueDownloadOp puts op on the download lane. A list is dropped while any
// op is on the lane, because that op lists when it ends.
func (m *model) queueDownloadOp(op downloadOp) tea.Cmd {
	if op.kind == opList && (!m.trackDownloads || m.downloadBusy || len(m.downloadOps) > 0) {
		return nil
	}
	op.list = m.trackDownloads
	m.downloadOps = append(append([]downloadOp(nil), m.downloadOps...), op)
	return m.nextDownloadOp()
}

func (m *model) nextDownloadOp() tea.Cmd {
	if m.downloadBusy || len(m.downloadOps) == 0 {
		return nil
	}
	op := m.downloadOps[0]
	m.downloadOps = m.downloadOps[1:]
	m.downloadBusy = true
	return runDownloadOp(op)
}

// finishDownloadOp takes the store's answer: the op's intent is dropped, the
// cached list is replaced whole, and the next op starts. A failed write shows
// its error and its row reverts to what the store holds.
func (m model) finishDownloadOp(msg downloadDoneMsg) (tea.Model, tea.Cmd) {
	m.downloadBusy = false
	selected := selectedDownloadID(m.downloadRows(), m.downloadIndex)
	m.downloadIntents = withoutIntent(m.downloadIntents, msg.op.key)
	finished := false
	if msg.op.list && msg.listErr == nil {
		finished = newlyFinished(m.downloads, msg.jobs)
		m.downloads = msg.jobs
	}
	m.downloadIndex = nextDownloadIndex(m.downloadRows(), selected)
	if finished && m.currentPlaylist != "" {
		m.reloadTracks()
	}
	if msg.op.kind != opList {
		m.status = downloadOpStatus(msg)
	}
	// A completion that leaves nothing queued, running, failed, or cancelled
	// closes the download view, so the center returns to the picture.
	if finished && len(m.downloadRows()) == 0 {
		m.showDownloads = false
		if strings.HasPrefix(m.status, "downloading ") {
			m.status = ""
		}
	}
	cmd := m.nextDownloadOp()
	return m, cmd
}

func downloadOpStatus(msg downloadDoneMsg) string {
	switch {
	case msg.err != nil:
		return msg.err.Error()
	case msg.op.kind == opQueue:
		return "downloading " + msg.op.title
	default:
		return "cancelled " + msg.op.title
	}
}

func withoutIntent(intents []downloadIntent, key string) []downloadIntent {
	kept := make([]downloadIntent, 0, len(intents))
	for _, intent := range intents {
		if intent.key != key {
			kept = append(kept, intent)
		}
	}
	return kept
}

func (m model) downloadIntent(key string) (downloadIntent, bool) {
	for _, intent := range m.downloadIntents {
		if intent.key == key {
			return intent, true
		}
	}
	return downloadIntent{}, false
}

func (m *model) addDownloadIntent(intent downloadIntent) {
	m.downloadIntents = append(append([]downloadIntent(nil), m.downloadIntents...), intent)
	m.status = intent.label()
}

// downloadRows is the cached list without finished jobs, with the pending
// intents laid over it: a cancel shows as cancelling and a new download as a
// queued row at the end.
func (m model) downloadRows() []store.Job {
	rows := visibleDownloads(m.downloads)
	for _, intent := range m.downloadIntents {
		if !intent.cancel {
			rows = append(rows, store.Job{ID: intent.key, Title: intent.title, Status: "queued"})
			continue
		}
		for i := range rows {
			if rows[i].ID == intent.key {
				rows[i].Status = "cancelling"
			}
		}
	}
	return rows
}

func newlyFinished(previous, next []store.Job) bool {
	if len(previous) == 0 {
		return false
	}
	was := make(map[string]string, len(previous))
	for _, job := range previous {
		was[job.ID] = job.Status
	}
	for _, job := range next {
		if job.Status == "done" && was[job.ID] != "" && was[job.ID] != "done" {
			return true
		}
	}
	return false
}

func runningIndex(jobs []store.Job) int {
	for i, job := range jobs {
		if job.Status == "running" {
			return i
		}
	}
	return 0
}

func jobsNeedWorker(jobs []store.Job) bool {
	for _, job := range jobs {
		if job.Status == "queued" || job.Status == "running" {
			return true
		}
	}
	return false
}

func visibleDownloads(jobs []store.Job) []store.Job {
	visible := make([]store.Job, 0, len(jobs))
	for _, job := range jobs {
		if job.Status == "done" {
			continue
		}
		visible = append(visible, job)
	}
	return visible
}

func selectedDownloadID(jobs []store.Job, index int) string {
	if index < 0 || index >= len(jobs) {
		return ""
	}
	return jobs[index].ID
}

func nextDownloadIndex(jobs []store.Job, selectedID string) int {
	if selectedID != "" {
		for i, job := range jobs {
			if job.ID == selectedID {
				return i
			}
		}
	}
	if len(jobs) == 0 {
		return 0
	}
	index := runningIndex(jobs)
	if index < 0 || index >= len(jobs) {
		return 0
	}
	return index
}

func (m model) renderDownloads() string {
	playing := ""
	if m.player != nil {
		if path, _ := m.player.State(); path != "" {
			playing = filepath.Base(path)
		}
	}
	return download.Render(m.downloadRows(), m.downloadIndex, m.visualizerFrame, m.width, playing)
}

func (m model) downloadStatus() string {
	for _, job := range m.downloadRows() {
		if job.Status != "running" && job.Status != "queued" {
			continue
		}
		if job.Status == "running" && job.Percent > 0 {
			return fmt.Sprintf("downloading %s %.0f%%", job.Title, job.Percent)
		}
		return "downloading " + job.Title
	}
	return ""
}

func (m model) updateDownloadKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.showDownloads = false
		return m, nil
	case "j", "down":
		m.moveDownload(1)
		return m, nil
	case "k", "up":
		m.moveDownload(-1)
		return m, nil
	case "x":
		cmd := m.cancelSelectedDownload()
		return m, cmd
	case "y":
		m.beginYouTubeDownload()
		return m, nil
	case " ":
		m.togglePause()
		return m, nil
	case "m":
		return m.toggleMenu()
	default:
		return m, nil
	}
}

func (m *model) moveDownload(delta int) {
	n := len(m.downloadRows())
	if n == 0 {
		return
	}
	if m.downloadIndex < 0 {
		m.downloadIndex = 0
	} else if m.downloadIndex >= n {
		m.downloadIndex = n - 1
	}
	m.downloadIndex = (m.downloadIndex + delta%n + n) % n
}

// cancelSelectedDownload paints the row as cancelling and hands the cancel to
// the download lane. A row that is still waiting on the store is left alone.
func (m *model) cancelSelectedDownload() tea.Cmd {
	rows := m.downloadRows()
	if m.downloadIndex < 0 || m.downloadIndex >= len(rows) {
		m.status = "no download selected"
		return nil
	}
	job := rows[m.downloadIndex]
	if intent, ok := m.downloadIntent(job.ID); ok {
		m.status = intent.label()
		return nil
	}
	m.addDownloadIntent(downloadIntent{key: job.ID, title: job.Title, cancel: true})
	return m.queueDownloadOp(downloadOp{kind: opCancel, key: job.ID, title: job.Title})
}

// queueYouTube paints a queued row for the new download and hands the insert
// to the download lane.
func (m *model) queueYouTube(dir, name, rawURL string) tea.Cmd {
	m.downloadSeq++
	key := "pending-" + strconv.Itoa(m.downloadSeq)
	m.addDownloadIntent(downloadIntent{key: key, title: name})
	m.showDownloads = true
	return m.queueDownloadOp(downloadOp{kind: opQueue, key: key, title: name, dir: dir, url: rawURL})
}
