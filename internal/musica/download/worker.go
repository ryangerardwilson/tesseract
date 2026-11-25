package download

import (
	"bufio"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/ryangerardwilson/tesseract/internal/musica/library"
	"github.com/ryangerardwilson/tesseract/internal/musica/store"
)

var errWorkerBusy = errors.New("download worker is already running")

var workerLock *os.File

// Enqueue records a YouTube audio download and starts the detached worker.
func Enqueue(destDir, title, rawURL string) (store.Job, error) {
	if !library.IsYouTubeURL(rawURL) {
		return store.Job{}, errors.New("not a YouTube URL")
	}
	filename, label, err := audioName(title)
	if err != nil {
		return store.Job{}, err
	}
	db, err := store.Open()
	if err != nil {
		return store.Job{}, err
	}
	job, err := db.InsertJob(rawURL, label, destDir, filename)
	if err != nil {
		return store.Job{}, err
	}
	EnsureWorker()
	return job, nil
}

func audioName(title string) (filename, label string, err error) {
	title = strings.TrimSpace(title)
	if title == "" {
		return "", "", errors.New("missing song name")
	}
	filename = filepath.Base(title)
	if filepath.Ext(filename) == "" {
		filename += ".mp3"
	}
	label = strings.TrimSuffix(filename, filepath.Ext(filename))
	if label == "" || filename == ".mp3" {
		return "", "", errors.New("missing song name")
	}
	return filename, label, nil
}

// List returns the saved downloads, newest last.
func List() ([]store.Job, error) {
	db, err := store.Open()
	if err != nil {
		return nil, err
	}
	return db.List()
}

// Cancel stops one download. Playback of other tracks is left alone.
func Cancel(id string) error {
	db, err := store.Open()
	if err != nil {
		return err
	}
	job, ok, err := db.Get(id)
	if err != nil || !ok {
		return err
	}
	if job.Status == "done" || job.Status == "cancelled" {
		return nil
	}
	if err := db.MarkStatus(id, "cancelled", ""); err != nil {
		return err
	}
	killProcess(job.PID)
	removeIncomplete(filepath.Join(job.DestDir, job.Filename))
	return nil
}

// Resume starts the detached worker when a download is still queued or running.
func Resume() {
	db, err := store.Open()
	if err != nil {
		return
	}
	jobs, err := db.List()
	if err != nil {
		return
	}
	for _, job := range jobs {
		if job.Status == "queued" || job.Status == "running" {
			EnsureWorker()
			return
		}
	}
}

var (
	spawnGate sync.Mutex
	lastSpawn time.Time
)

// EnsureWorker starts a worker in its own session. Closing the TUI does not signal it.
func EnsureWorker() {
	if os.Getenv("MUSICA_NO_SPAWN") == "1" {
		return
	}
	spawnGate.Lock()
	defer spawnGate.Unlock()
	if workerAlive() {
		return
	}
	if !lastSpawn.IsZero() && time.Since(lastSpawn) < time.Second {
		return
	}
	if spawnDetached() == nil {
		lastSpawn = time.Now()
	}
}

func workerAlive() bool {
	err := lockWorker()
	if errors.Is(err, errWorkerBusy) {
		return true
	}
	if err != nil {
		return false
	}
	unlockWorker()
	return false
}

// lookPath finds the worker binary. Tests replace it.
var lookPath = exec.LookPath

// workerPath is musica on PATH. The player can run inside r2d2, so the
// current executable is not the worker and is never a fallback.
func workerPath() (string, error) {
	return lookPath("musica")
}

func spawnDetached() error {
	exe, err := workerPath()
	if err != nil {
		return err
	}
	cmd := workerCommand(exe)
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	if logFile, openErr := os.OpenFile(filepath.Join(runtimeDir(), "download.log"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600); openErr == nil {
		cmd.Stdout = logFile
		cmd.Stderr = logFile
		defer logFile.Close()
	}
	if err := cmd.Start(); err != nil {
		return err
	}
	go func() { _ = cmd.Wait() }()
	return nil
}

func workerCommand(exe string) *exec.Cmd {
	cmd := exec.Command(exe, "download-worker")
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	return cmd
}

func runtimeDir() string {
	return filepath.Dir(store.FilePath())
}

// RunWorker drains the queue. A second worker exits without taking the lock.
func RunWorker() error {
	if err := lockWorker(); err != nil {
		if errors.Is(err, errWorkerBusy) {
			return nil
		}
		return err
	}
	defer unlockWorker()
	db, err := openDB()
	if err != nil {
		return err
	}
	if err := requeue(db); err != nil {
		return err
	}
	for {
		job, ok, err := claimNext(db)
		if err != nil {
			return err
		}
		if !ok {
			return nil
		}
		runJob(db, job)
	}
}

func openDB() (*store.DB, error) {
	db, err := store.Open()
	if err == nil || !store.IsBusy(err) {
		return db, err
	}
	time.Sleep(200 * time.Millisecond)
	return store.Open()
}

func requeue(db *store.DB) error {
	err := db.RequeueDead()
	if err == nil || !store.IsBusy(err) {
		return err
	}
	time.Sleep(200 * time.Millisecond)
	return db.RequeueDead()
}

func claimNext(db *store.DB) (store.Job, bool, error) {
	job, ok, err := db.ClaimNext()
	if err == nil || !store.IsBusy(err) {
		return job, ok, err
	}
	time.Sleep(200 * time.Millisecond)
	return db.ClaimNext()
}

func runJob(db *store.DB, job store.Job) {
	target := filepath.Join(job.DestDir, job.Filename)
	if err := os.MkdirAll(job.DestDir, 0o755); err != nil {
		_ = db.MarkStatus(job.ID, "failed", err.Error())
		return
	}
	cmd := ytdlpCommand(target, job.URL)
	readEnd, writeEnd, err := os.Pipe()
	if err != nil {
		_ = db.MarkStatus(job.ID, "failed", err.Error())
		return
	}
	cmd.Stdout = writeEnd
	cmd.Stderr = writeEnd
	if err := cmd.Start(); err != nil {
		_ = writeEnd.Close()
		_ = readEnd.Close()
		_ = db.MarkStatus(job.ID, "failed", err.Error())
		return
	}
	_ = writeEnd.Close()
	_ = db.SetPID(job.ID, cmd.Process.Pid)
	scanned := make(chan struct{})
	go func() {
		defer close(scanned)
		defer readEnd.Close()
		scanProgress(db, job.ID, readEnd)
	}()
	stop := make(chan struct{})
	go watchCancel(db, job.ID, cmd.Process, stop)
	waitErr := cmd.Wait()
	<-scanned
	close(stop)
	finishJob(db, job, target, waitErr)
}

func scanProgress(db *store.DB, id string, reader io.Reader) {
	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		percent, speed, eta, ok := ParseProgress(scanner.Text())
		if !ok {
			continue
		}
		current, found, err := db.Get(id)
		if err == nil && found && current.Status == "cancelled" {
			continue
		}
		_ = db.MarkProgress(id, percent, speed, eta)
	}
}

func watchCancel(db *store.DB, id string, process *os.Process, stop <-chan struct{}) {
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-stop:
			return
		case <-ticker.C:
			job, ok, err := db.Get(id)
			if err != nil || !ok || job.Status != "cancelled" {
				continue
			}
			killProcess(process.Pid)
			return
		}
	}
}

func finishJob(db *store.DB, job store.Job, target string, waitErr error) {
	current, ok, err := db.Get(job.ID)
	if err != nil || !ok {
		return
	}
	if current.Status == "cancelled" {
		removeIncomplete(target)
		return
	}
	if waitErr != nil {
		_ = db.MarkStatus(job.ID, "failed", waitErr.Error())
		removeIncomplete(target)
		return
	}
	_ = db.MarkProgress(job.ID, 100, current.Speed, "00:00")
	_ = db.MarkStatus(job.ID, "done", "")
}

func ytdlpCommand(target, rawURL string) *exec.Cmd {
	bin := os.Getenv("MUSICA_YTDLP")
	if bin == "" {
		bin = "yt-dlp"
	}
	cmd := exec.Command(
		bin,
		"--newline",
		"--no-playlist",
		"-x",
		"--audio-format", "mp3",
		"--audio-quality", "0",
		"--embed-thumbnail",
		"--add-metadata",
		"--embed-chapters",
		"-o", target,
		rawURL,
	)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	return cmd
}

func killProcess(pid int) {
	if pid <= 0 {
		return
	}
	_ = syscall.Kill(-pid, syscall.SIGKILL)
	_ = syscall.Kill(pid, syscall.SIGKILL)
}

func removeIncomplete(target string) {
	_ = os.Remove(target)
	_ = os.Remove(target + ".part")
	matches, _ := filepath.Glob(target + ".part*")
	for _, match := range matches {
		_ = os.Remove(match)
	}
	base := strings.TrimSuffix(target, filepath.Ext(target))
	fragments, _ := filepath.Glob(base + ".f*")
	for _, match := range fragments {
		_ = os.Remove(match)
	}
}

func lockWorker() error {
	if err := os.MkdirAll(runtimeDir(), 0o700); err != nil {
		return err
	}
	file, err := os.OpenFile(filepath.Join(runtimeDir(), "download.lock"), os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return err
	}
	if err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		_ = file.Close()
		if errors.Is(err, syscall.EWOULDBLOCK) {
			return errWorkerBusy
		}
		return err
	}
	workerLock = file
	return nil
}

func unlockWorker() {
	if workerLock == nil {
		return
	}
	_ = syscall.Flock(int(workerLock.Fd()), syscall.LOCK_UN)
	_ = workerLock.Close()
	workerLock = nil
}
