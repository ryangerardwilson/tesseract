package app

import (
	"errors"
	"fmt"
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

// TestMain keeps every test in this package off the operator's database and
// off the detached worker, even when a lane op runs for real.
func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "musica-app-test-")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	_ = os.Setenv("MUSICA_DB", filepath.Join(dir, "musica.sqlite"))
	_ = os.Setenv("MUSICA_NO_SPAWN", "1")
	code := m.Run()
	_ = os.RemoveAll(dir)
	os.Exit(code)
}

// land runs a lane command and hands its answer back to the model, as the
// bubbletea loop would.
func land(t *testing.T, m model, cmd tea.Cmd) model {
	t.Helper()
	if cmd == nil {
		t.Fatal("no command to land")
	}
	next, _ := m.Update(cmd())
	return next.(model)
}

// stubDownloads answers every list with jobs and counts the reads.
func stubDownloads(t *testing.T, jobs []store.Job) *int {
	t.Helper()
	reads := new(int)
	oldList, oldWorker := listDownloads, ensureWorker
	listDownloads = func() ([]store.Job, error) {
		*reads++
		return append([]store.Job(nil), jobs...), nil
	}
	ensureWorker = func() {}
	t.Cleanup(func() { listDownloads, ensureWorker = oldList, oldWorker })
	return reads
}

func stubCancel(t *testing.T, err error) *[]string {
	t.Helper()
	cancelled := &[]string{}
	old := cancelDownload
	cancelDownload = func(id string) error {
		*cancelled = append(*cancelled, id)
		return err
	}
	t.Cleanup(func() { cancelDownload = old })
	return cancelled
}

func stubEnqueue(t *testing.T, err error) *[]string {
	t.Helper()
	queued := &[]string{}
	old := enqueueDownload
	enqueueDownload = func(_, name, _ string) error {
		*queued = append(*queued, name)
		return err
	}
	t.Cleanup(func() { enqueueDownload = old })
	return queued
}

// recordingPlayer logs every Play and Stop, so a test can tell whether
// Update reached ffplay.
type recordingPlayer struct {
	fakeAudioPlayer
	calls []string
}

func (p *recordingPlayer) Play(path string, offset time.Duration) error {
	p.calls = append(p.calls, "play "+path)
	return p.fakeAudioPlayer.Play(path, offset)
}

func (p *recordingPlayer) Stop() {
	p.calls = append(p.calls, "stop")
	p.fakeAudioPlayer.Stop()
}

func key(r rune) tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}}
}

func runningJobs() []store.Job {
	return []store.Job{
		{ID: "a", Title: "Blue Monday", Status: "running", Percent: 40},
		{ID: "b", Title: "Other", Status: "queued"},
	}
}

func TestTheFrameTickDoesNotListDownloads(t *testing.T) {
	reads := stubDownloads(t, runningJobs())
	m := model{cfg: config.AppConfig{FPS: 20}, trackDownloads: true, showDownloads: true, downloads: runningJobs()}
	for range 5 {
		next, _ := m.Update(tickMsg(time.Now()))
		m = next.(model)
	}
	if *reads != 0 || m.downloadBusy {
		t.Fatalf("the tick read downloads: reads %d busy %v", *reads, m.downloadBusy)
	}
}

func TestThePollReadsOnTheLaneAndReplacesTheCache(t *testing.T) {
	answer := []store.Job{{ID: "a", Title: "Blue Monday", Status: "running"}, {ID: "c", Title: "Done", Status: "done"}}
	reads := stubDownloads(t, answer)
	m := model{trackDownloads: true, downloads: []store.Job{{ID: "old", Title: "Old", Status: "queued"}}}

	next, cmd := m.Update(downloadPollMsg{})
	m = next.(model)
	if *reads != 0 || !m.downloadBusy {
		t.Fatalf("poll read inside Update: reads %d busy %v", *reads, m.downloadBusy)
	}
	batch, ok := cmd().(tea.BatchMsg)
	if !ok || len(batch) != 2 {
		t.Fatalf("poll = %T %v, want the read and the next poll", cmd(), batch)
	}

	next, again := m.Update(downloadPollMsg{})
	if again == nil || len(next.(model).downloadOps) != 0 || *reads != 0 {
		t.Fatal("a poll while a read runs did more than book the next poll")
	}

	landed := land(t, m, batch[0])
	if *reads != 1 || landed.downloadBusy {
		t.Fatalf("reads %d busy %v", *reads, landed.downloadBusy)
	}
	if len(landed.downloads) != 2 || landed.downloads[0].ID != "a" {
		t.Fatalf("cache %+v, want the store's answer whole", landed.downloads)
	}
	if rows := landed.downloadRows(); len(rows) != 1 || rows[0].ID != "a" {
		t.Fatalf("rows %+v", rows)
	}
}

func TestCancelPaintsCancellingAndRunsOnTheLane(t *testing.T) {
	stubDownloads(t, []store.Job{
		{ID: "a", Title: "Blue Monday", Status: "cancelled"},
		{ID: "b", Title: "Other", Status: "queued"},
	})
	cancelled := stubCancel(t, nil)
	m := model{trackDownloads: true, showDownloads: true, width: 80, downloads: runningJobs()}

	next, cmd := m.Update(key('x'))
	painted := next.(model)
	if cmd == nil || len(*cancelled) != 0 {
		t.Fatalf("cancel ran inside Update: %v", *cancelled)
	}
	if painted.downloads[0].Status != "running" {
		t.Fatal("the cached list was edited")
	}
	if rows := painted.downloadRows(); rows[0].Status != "cancelling" || rows[1].Status != "queued" {
		t.Fatalf("rows %+v", rows)
	}
	if painted.status != "cancelling Blue Monday" || !strings.Contains(painted.renderDownloads(), "cancelling") {
		t.Fatalf("status %q view %s", painted.status, painted.renderDownloads())
	}
	if _, twice := painted.Update(key('x')); twice != nil {
		t.Fatal("a second x cancelled again")
	}

	landed := land(t, painted, cmd)
	if strings.Join(*cancelled, ",") != "a" || len(landed.downloadIntents) != 0 {
		t.Fatalf("cancelled %v intents %v", *cancelled, landed.downloadIntents)
	}
	if landed.downloadRows()[0].Status != "cancelled" || landed.status != "cancelled Blue Monday" {
		t.Fatalf("rows %+v status %q", landed.downloadRows(), landed.status)
	}
}

func TestAFailedCancelRevertsTheRow(t *testing.T) {
	stubDownloads(t, runningJobs())
	stubCancel(t, errors.New("sqlite3: database is locked"))
	m := model{trackDownloads: true, showDownloads: true, downloads: runningJobs()}

	next, cmd := m.Update(key('x'))
	landed := land(t, next.(model), cmd)
	if landed.downloadRows()[0].Status != "running" {
		t.Fatalf("rows %+v, want the store's running row back", landed.downloadRows())
	}
	if landed.status != "sqlite3: database is locked" {
		t.Fatalf("status %q", landed.status)
	}
}

func TestQueuePaintsAQueuedRowAndRunsOnTheLane(t *testing.T) {
	stubDownloads(t, []store.Job{{ID: "1", Title: "Blue Monday", Status: "queued"}})
	queued := stubEnqueue(t, nil)
	m := model{
		trackDownloads:  true,
		view:            tracksView,
		currentPlaylist: t.TempDir(),
		inputKind:       inputYouTubeName,
		input:           "Blue Monday",
		youtubeURL:      "https://youtu.be/example",
		width:           80,
	}

	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	painted := next.(model)
	if cmd == nil || len(*queued) != 0 {
		t.Fatalf("the insert ran inside Update: %v", *queued)
	}
	rows := painted.downloadRows()
	if !painted.showDownloads || len(rows) != 1 || rows[0].Title != "Blue Monday" || rows[0].Status != "queued" {
		t.Fatalf("rows %+v shown %v", rows, painted.showDownloads)
	}
	if painted.status != "queueing Blue Monday" || !strings.Contains(painted.renderDownloads(), "waiting") {
		t.Fatalf("status %q view %s", painted.status, painted.renderDownloads())
	}

	landed := land(t, painted, cmd)
	if strings.Join(*queued, ",") != "Blue Monday" || len(landed.downloadIntents) != 0 {
		t.Fatalf("queued %v intents %v", *queued, landed.downloadIntents)
	}
	if rows := landed.downloadRows(); len(rows) != 1 || rows[0].ID != "1" {
		t.Fatalf("rows %+v, want the store's row", rows)
	}
	if landed.status != "downloading Blue Monday" {
		t.Fatalf("status %q", landed.status)
	}
}

func TestAFailedQueueDropsTheRow(t *testing.T) {
	stubDownloads(t, nil)
	stubEnqueue(t, errors.New("nebula keep restore musica: connection timed out"))
	m := model{
		trackDownloads:  true,
		view:            tracksView,
		currentPlaylist: t.TempDir(),
		inputKind:       inputYouTubeName,
		input:           "Blue Monday",
		youtubeURL:      "https://youtu.be/example",
	}
	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	landed := land(t, next.(model), cmd)
	if rows := landed.downloadRows(); len(rows) != 0 {
		t.Fatalf("rows %+v", rows)
	}
	if !strings.Contains(landed.status, "connection timed out") {
		t.Fatalf("status %q", landed.status)
	}
}

func TestAWriteWaitsBehindARunningRead(t *testing.T) {
	stubDownloads(t, runningJobs())
	cancelled := stubCancel(t, nil)
	m := model{trackDownloads: true, showDownloads: true, downloads: runningJobs()}

	next, poll := m.Update(downloadPollMsg{})
	read := poll().(tea.BatchMsg)[0]
	next, cmd := next.(model).Update(key('x'))
	waiting := next.(model)
	if cmd != nil || len(waiting.downloadOps) != 1 {
		t.Fatalf("the cancel did not wait: cmd %v ops %v", cmd != nil, waiting.downloadOps)
	}
	next, cmd = waiting.Update(read())
	afterRead := next.(model)
	if cmd == nil || len(*cancelled) != 0 {
		t.Fatal("the read's answer did not start the cancel")
	}
	if afterRead.downloadRows()[0].Status != "cancelling" {
		t.Fatalf("the read's answer dropped the cancel: %+v", afterRead.downloadRows())
	}
	landed := land(t, afterRead, cmd)
	if strings.Join(*cancelled, ",") != "a" || landed.downloadBusy {
		t.Fatalf("cancelled %v busy %v", *cancelled, landed.downloadBusy)
	}
}

func playingModel(player *recordingPlayer) model {
	return model{
		view:        tracksView,
		player:      player,
		visualizer:  &visualizer.Analyzer{},
		currentPath: "/music/a.mp3",
		tracks:      []library.Track{{Path: "/music/a.mp3"}, {Path: "/music/b.mp3"}},
		selected:    1,
		width:       80,
		height:      24,
	}
}

func TestPlayKeysReturnCommandsInsteadOfWaiting(t *testing.T) {
	for _, r := range []rune{'f', 'b', 'r', 'x'} {
		player := &recordingPlayer{fakeAudioPlayer: fakeAudioPlayer{path: "/music/a.mp3", playing: true}}
		next, cmd := playingModel(player).Update(key(r))
		painted := next.(model)
		if cmd == nil || len(player.calls) != 0 {
			t.Fatalf("%c reached ffplay inside Update: %v", r, player.calls)
		}
		if !strings.HasSuffix(painted.status, "…") || !painted.audioBusy {
			t.Fatalf("%c painted %q busy %v", r, painted.status, painted.audioBusy)
		}
		landed := land(t, painted, cmd)
		if len(player.calls) == 0 || landed.audioBusy || landed.audioPending != "" {
			t.Fatalf("%c landed with calls %v busy %v", r, player.calls, landed.audioBusy)
		}
	}
}

func TestEnterPaintsFirstAndTheNewestRequestWaits(t *testing.T) {
	player := &recordingPlayer{fakeAudioPlayer: fakeAudioPlayer{path: "/music/a.mp3", playing: true}}
	next, play := playingModel(player).Update(tea.KeyMsg{Type: tea.KeyEnter})
	painted := next.(model)
	if play == nil || len(player.calls) != 0 {
		t.Fatalf("enter reached ffplay inside Update: %v", player.calls)
	}
	if painted.status != "starting…" || painted.playingFieldText() != "starting…" || painted.currentPath != "/music/a.mp3" {
		t.Fatalf("painted %q %q %q", painted.status, painted.playingFieldText(), painted.currentPath)
	}

	next, stop := painted.Update(key('x'))
	waiting := next.(model)
	if stop != nil || waiting.status != "stopping…" || !waiting.audioWaiting {
		t.Fatalf("x did not wait behind the play: %q", waiting.status)
	}

	next, stop = waiting.Update(play())
	played := next.(model)
	if played.currentPath != "/music/b.mp3" || stop == nil || played.status != "stopping…" {
		t.Fatalf("play landed as %q, status %q, next %v", played.currentPath, played.status, stop != nil)
	}
	stopped := land(t, played, stop)
	if strings.Join(player.calls, ",") != "play /music/b.mp3,stop" {
		t.Fatalf("calls %v", player.calls)
	}
	if stopped.currentPath != "" || stopped.status != "stopped" || stopped.audioBusy {
		t.Fatalf("stopped %q %q busy %v", stopped.currentPath, stopped.status, stopped.audioBusy)
	}
}

func TestQuitWaitsForTheRunningAudioOp(t *testing.T) {
	player := &recordingPlayer{fakeAudioPlayer: fakeAudioPlayer{path: "/music/a.mp3", playing: true}}
	next, play := playingModel(player).Update(tea.KeyMsg{Type: tea.KeyEnter})
	next, cmd := next.(model).Update(key('q'))
	quitting := next.(model)
	if cmd != nil || quitting.status != "stopping…" {
		t.Fatalf("q quit before the running op landed: %q", quitting.status)
	}
	next, cmd = quitting.Update(play())
	if !isQuitCommand(cmd) {
		t.Fatal("the landed op did not quit")
	}
	if strings.Join(player.calls, ",") != "play /music/b.mp3,stop" || player.IsPlaying() {
		t.Fatalf("calls %v playing %v", player.calls, player.IsPlaying())
	}
}

func TestTheTickStartsTheNextTrackOnlyWhenTheLaneIsIdle(t *testing.T) {
	player := &recordingPlayer{}
	m := playingModel(player)
	m.cfg = config.AppConfig{FPS: 20}
	m.queue = m.tracks
	m.currentIndex = 0
	m.audioBusy = true

	next, _ := m.Update(tickMsg(time.Now()))
	if busy := next.(model); busy.currentIndex != 0 || busy.audioWaiting {
		t.Fatal("the tick advanced while an op ran")
	}

	m.audioBusy = false
	next, cmd := m.Update(tickMsg(time.Now()))
	advanced := next.(model)
	if cmd == nil || advanced.currentIndex != 1 || !advanced.audioBusy || len(player.calls) != 0 {
		t.Fatalf("index %d busy %v calls %v", advanced.currentIndex, advanced.audioBusy, player.calls)
	}
}
