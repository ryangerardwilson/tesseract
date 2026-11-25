package session

import (
	"errors"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// heldName is the note End leaves beside the lock: the track the ended
// session's ffplay had loaded, playing or paused. The next musica serve takes
// it and opens that track's cassette without playing it.
const heldName = "held"

// endWait is how long End waits for the session to let go. A terminal session
// stops a paused ffplay and ffmpeg with SIGTERM and then SIGKILL, which takes a
// little over a second.
var endWait = 5 * time.Second

// TakeHeld returns the track a session that End ended had loaded, and forgets
// it. It returns "" when End left no track. Only the session holder takes it.
func (s *Server) TakeHeld() string {
	note := filepath.Join(filepath.Dir(s.path), heldName)
	data, err := os.ReadFile(note)
	if err != nil {
		return ""
	}
	_ = os.Remove(note)
	return strings.TrimSpace(string(data))
}

// keepHeld notes the track that the session at the other end of connection
// has loaded, or forgets an older note when it has none. A terminal session
// from an older musica answers only end, so the track is read from its ffplay
// child in /proc before end is sent.
func keepHeld(directory string, connection net.Conn) {
	note := filepath.Join(directory, heldName)
	track := ""
	if pid, ok := peerPID(connection); ok {
		track = loadedTrack(pid)
	}
	if track == "" {
		_ = os.Remove(note)
		return
	}
	_ = os.WriteFile(note, []byte(track+"\n"), 0o600)
}

// peerPID is the process that listens on the control socket.
func peerPID(connection net.Conn) (int, bool) {
	unixConnection, ok := connection.(*net.UnixConn)
	if !ok {
		return 0, false
	}
	raw, err := unixConnection.SyscallConn()
	if err != nil {
		return 0, false
	}
	var credentials *syscall.Ucred
	credentialsErr := errors.New("no credentials")
	if err := raw.Control(func(fd uintptr) {
		credentials, credentialsErr = syscall.GetsockoptUcred(int(fd), syscall.SOL_SOCKET, syscall.SO_PEERCRED)
	}); err != nil || credentialsErr != nil || credentials.Pid <= 0 {
		return 0, false
	}
	return int(credentials.Pid), true
}

// loadedTrack is the track that an ffplay child of pid has loaded. musica
// starts ffplay with the track as its last argument, and a paused ffplay is a
// stopped process that still has it.
func loadedTrack(pid int) string {
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return ""
	}
	for _, entry := range entries {
		child, err := strconv.Atoi(entry.Name())
		if err != nil {
			continue
		}
		name, parent, ok := procStat(child)
		if !ok || parent != pid || name != "ffplay" {
			continue
		}
		if args := procArgs(child); len(args) > 1 && filepath.IsAbs(args[len(args)-1]) {
			return args[len(args)-1]
		}
	}
	return ""
}

// procStat reads a process's name and parent from /proc/<pid>/stat. The name
// sits in parentheses and may hold parentheses itself.
func procStat(pid int) (string, int, bool) {
	data, err := os.ReadFile(filepath.Join("/proc", strconv.Itoa(pid), "stat"))
	if err != nil {
		return "", 0, false
	}
	text := string(data)
	open, closing := strings.IndexByte(text, '('), strings.LastIndexByte(text, ')')
	if open < 0 || closing < open {
		return "", 0, false
	}
	fields := strings.Fields(text[closing+1:])
	if len(fields) < 2 {
		return "", 0, false
	}
	parent, err := strconv.Atoi(fields[1])
	if err != nil {
		return "", 0, false
	}
	return text[open+1 : closing], parent, true
}

func procArgs(pid int) []string {
	data, err := os.ReadFile(filepath.Join("/proc", strconv.Itoa(pid), "cmdline"))
	if err != nil {
		return nil
	}
	return strings.Split(strings.TrimRight(string(data), "\x00"), "\x00")
}

// waitReleased waits until no session holds the lock in directory, so that a
// musica serve started next can take it.
func waitReleased(directory string) error {
	deadline := time.Now().Add(endWait)
	for {
		lock, err := acquireLock(directory)
		if err == nil {
			releaseLock(lock)
			return nil
		}
		if !errors.Is(err, ErrAlreadyRunning) {
			return err
		}
		if time.Now().After(deadline) {
			return errors.New("musica did not end")
		}
		time.Sleep(50 * time.Millisecond)
	}
}
