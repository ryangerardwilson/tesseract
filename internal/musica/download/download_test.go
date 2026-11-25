package download

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/ryangerardwilson/tesseract/internal/musica/store"
)

func TestParseProgressLine(t *testing.T) {
	percent, speed, eta, ok := ParseProgress("[download]  12.5% of    3.40MiB at    1.20MiB/s ETA 00:02")
	if !ok || percent != 12.5 || speed != "1.20MiB/s" || eta != "00:02" {
		t.Fatalf("parsed = %v %q %q ok=%v", percent, speed, eta, ok)
	}
	if _, _, _, ok := ParseProgress("Destination: song.mp3"); ok {
		t.Fatal("destination line parsed as progress")
	}
}

func TestRenderRunningJobChangesFrame(t *testing.T) {
	jobs := []store.Job{{
		ID: "1", Title: "Blue Monday", Status: "running", Percent: 40, Speed: "1MiB/s", ETA: "00:03",
	}}
	first := Render(jobs, 0, 0, 80, "Other Song.mp3")
	second := Render(jobs, 0, 1, 80, "Other Song.mp3")
	if first == second {
		t.Fatalf("running download did not animate:\n%s", first)
	}
	if !strings.Contains(first, "Blue Monday") || !strings.Contains(first, "40%") || !strings.Contains(first, "█") {
		t.Fatalf("scene missing progress: %s", first)
	}
	if !strings.Contains(first, "playing  Other Song.mp3") {
		t.Fatalf("scene missing playback: %s", first)
	}
}

func TestRenderQueuedJobStaysStill(t *testing.T) {
	jobs := []store.Job{{ID: "1", Title: "Waiting", Status: "queued"}}
	if Render(jobs, 0, 0, 80, "") != Render(jobs, 0, 1, 80, "") {
		t.Fatal("queued download changed frames")
	}
	cancelled := Render([]store.Job{{ID: "1", Title: "Stop", Status: "cancelled"}}, 0, 0, 80, "")
	if !strings.Contains(cancelled, "cancelled") {
		t.Fatalf("cancel scene = %s", cancelled)
	}
}

func TestDownloadTextIsWhite(t *testing.T) {
	failed := fmt.Sprint(lineStyle("failed", false).GetForeground())
	running := fmt.Sprint(lineStyle("running", false).GetForeground())
	if failed != "15" || running != "15" {
		t.Fatalf("download text = %q %q, want white", failed, running)
	}
}

func TestWorkerWritesAudioAndRecordsDone(t *testing.T) {
	root := t.TempDir()
	t.Setenv("MUSICA_DB", filepath.Join(root, "musica.sqlite"))
	t.Setenv("MUSICA_NO_SPAWN", "1")
	t.Setenv("MUSICA_YTDLP", fakeDownloader(t, root, false))

	dest := filepath.Join(root, "cassette")
	if _, err := Enqueue(dest, "Blue Monday", "https://youtu.be/example"); err != nil {
		t.Fatal(err)
	}
	if err := RunWorker(); err != nil {
		t.Fatal(err)
	}
	jobs, err := List()
	if err != nil {
		t.Fatal(err)
	}
	if len(jobs) != 1 || jobs[0].Status != "done" || jobs[0].Percent != 100 {
		t.Fatalf("jobs = %+v", jobs)
	}
	body, err := os.ReadFile(filepath.Join(dest, "Blue Monday.mp3"))
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != "audio" {
		t.Fatalf("file = %q", body)
	}
}

func TestCancelStopsWorkerWithoutLeavingAudio(t *testing.T) {
	root := t.TempDir()
	t.Setenv("MUSICA_DB", filepath.Join(root, "musica.sqlite"))
	t.Setenv("MUSICA_NO_SPAWN", "1")
	t.Setenv("MUSICA_YTDLP", fakeDownloader(t, root, true))

	dest := filepath.Join(root, "cassette")
	job, err := Enqueue(dest, "Blue Monday", "https://youtu.be/example")
	if err != nil {
		t.Fatal(err)
	}
	finished := make(chan error, 1)
	go func() { finished <- RunWorker() }()

	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		jobs, listErr := List()
		if listErr == nil && len(jobs) == 1 && jobs[0].Status == "running" && jobs[0].PID > 0 {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if err := Cancel(job.ID); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-finished:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("worker did not finish after cancel")
	}
	jobs, err := List()
	if err != nil {
		t.Fatal(err)
	}
	if len(jobs) != 1 || jobs[0].Status != "cancelled" {
		t.Fatalf("jobs = %+v", jobs)
	}
	if _, err := os.Stat(filepath.Join(dest, "Blue Monday.mp3")); !os.IsNotExist(err) {
		t.Fatalf("cancelled download left a file: %v", err)
	}
}

func TestWorkerCommandStartsNewSession(t *testing.T) {
	cmd := workerCommand("/bin/sleep")
	if cmd.SysProcAttr == nil || !cmd.SysProcAttr.Setsid {
		t.Fatal("worker command must set sid")
	}
	if len(cmd.Args) != 2 || cmd.Args[1] != "download-worker" {
		t.Fatalf("args = %#v", cmd.Args)
	}

	child := exec.Command("/bin/sleep", "30")
	child.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := child.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = child.Process.Kill(); _ = child.Wait() }()

	parentSID := sessionID(t, os.Getpid())
	childSID := sessionID(t, child.Process.Pid)
	if parentSID == "" || childSID == "" || parentSID == childSID {
		t.Fatalf("sessions parent=%q child=%q", parentSID, childSID)
	}
}

func sessionID(t *testing.T, pid int) string {
	t.Helper()
	out, err := exec.Command("ps", "-o", "sid=", "-p", strconv.Itoa(pid)).Output()
	if err != nil {
		t.Fatal(err)
	}
	return strings.TrimSpace(string(out))
}

func fakeDownloader(t *testing.T, root string, linger bool) string {
	t.Helper()
	path := filepath.Join(root, "fake-ytdlp")
	body := "#!/bin/sh\nprev=\nout=\nfor arg in \"$@\"; do\n  if [ \"$prev\" = \"-o\" ]; then out=$arg; fi\n  prev=$arg\ndone\necho \"[download]  40.0% of 1.00MiB at 1.00MiB/s ETA 00:01\"\n"
	if linger {
		body += "sleep 30\n"
	} else {
		body += "echo \"[download] 100% of 1.00MiB at 1.00MiB/s ETA 00:00\"\nmkdir -p \"$(dirname \"$out\")\"\nprintf 'audio' > \"$out\"\n"
	}
	if err := os.WriteFile(path, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}
