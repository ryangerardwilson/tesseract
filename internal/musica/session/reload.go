package session

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"time"
)

const reloadCommand = "reload"

var errNotRunning = errors.New("musica is not running")

// Reload replaces the running session with this binary and continues
// playback where it was. A session that does not understand reload, such as
// an older musica serve, is ended after its place is noted, and this binary
// starts musica serve.
func Reload() error {
	path, err := socketPath()
	if err != nil {
		return err
	}
	response, err := roundTrip(path, reloadCommand)
	if errors.Is(err, errNotRunning) {
		return err
	}
	if err == nil && response == "ok" {
		return waitServing(path)
	}
	return reloadLegacy()
}

// ExecServe replaces this process with musica serve from the binary on disk.
func ExecServe() {
	bin, err := os.Executable()
	if err != nil {
		os.Exit(1)
	}
	_ = syscall.Exec(bin, []string{"musica", "serve"}, os.Environ())
	os.Exit(1)
}

func roundTrip(path, command string) (string, error) {
	connection, err := net.DialTimeout("unix", path, 500*time.Millisecond)
	if err != nil {
		return "", errNotRunning
	}
	defer connection.Close()
	_ = connection.SetDeadline(time.Now().Add(2 * time.Second))
	if _, err := fmt.Fprintln(connection, command); err != nil {
		return "", err
	}
	response, err := bufio.NewReader(connection).ReadString('\n')
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(response), nil
}

func reloadLegacy() error {
	note := snapshotSession()
	if note.Path != "" {
		if err := WriteResume("", note); err != nil {
			return err
		}
	}
	if err := End(); err != nil {
		return err
	}
	return startServe()
}

func snapshotSession() Resume {
	path, err := socketPath()
	if err != nil {
		return Resume{}
	}
	connection, err := net.DialTimeout("unix", path, 500*time.Millisecond)
	if err != nil {
		return Resume{}
	}
	defer connection.Close()
	note := Resume{Paused: true, Menu: true}
	if pid, ok := peerPID(connection); ok {
		note.Path = loadedTrack(pid)
	}
	_ = connection.SetDeadline(time.Now().Add(2 * time.Second))
	if _, err := fmt.Fprintln(connection, watchCommand); err != nil {
		return note
	}
	line, err := bufio.NewReader(connection).ReadString('\n')
	if err != nil || strings.TrimSpace(line) == "unknown command" {
		return note
	}
	var frame struct {
		State   string `json:"state"`
		Elapsed string `json:"elapsed"`
		Menu    bool   `json:"menu"`
	}
	if json.Unmarshal([]byte(line), &frame) != nil {
		return note
	}
	note.Menu = frame.Menu
	note.Paused = frame.State != "playing"
	note.Offset = parseClock(frame.Elapsed)
	return note
}

func parseClock(value string) time.Duration {
	parts := strings.Split(value, ":")
	if len(parts) != 2 {
		return 0
	}
	minutes, minutesErr := strconv.Atoi(parts[0])
	seconds, secondsErr := strconv.Atoi(parts[1])
	if minutesErr != nil || secondsErr != nil || minutes < 0 || seconds < 0 {
		return 0
	}
	return time.Duration(minutes)*time.Minute + time.Duration(seconds)*time.Second
}

func startServe() error {
	bin, err := os.Executable()
	if err != nil {
		return err
	}
	command := exec.Command(bin, "serve")
	command.Args = []string{"musica", "serve"}
	command.Stdin = nil
	command.Stdout = nil
	command.Stderr = nil
	command.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	return command.Start()
}

func waitServing(path string) error {
	deadline := time.Now().Add(endWait)
	down := false
	for time.Now().Before(deadline) {
		connection, err := net.DialTimeout("unix", path, 100*time.Millisecond)
		if err != nil {
			down = true
			time.Sleep(20 * time.Millisecond)
			continue
		}
		_ = connection.Close()
		if down {
			return nil
		}
		time.Sleep(20 * time.Millisecond)
	}
	return errors.New("musica did not reload")
}
