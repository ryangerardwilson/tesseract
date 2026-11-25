package session

import (
	"bufio"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"
)

const (
	controlSocketName = "control.sock"
	controlLockName   = "control.lock"
	endCommand        = "end"
	watchCommand      = "watch"
	// watchQueue is how many lines a watcher may fall behind before it is
	// dropped, so a stuck client never stalls the player.
	watchQueue = 64
	keyQueue   = 64
)

var ErrAlreadyRunning = errors.New("musica is already running")

type Server struct {
	listener net.Listener
	lock     *os.File
	requests chan struct{}
	reloads  chan struct{}
	path     string

	// keys is nil on a TUI session, which answers watch with unknown command.
	keys     chan string
	closing  chan struct{}
	mu       sync.Mutex
	closed   bool
	watchers map[*watcher]struct{}
}

// watcher is one client that wrote watch: published lines queue for it, and
// its writer sends them in order.
type watcher struct {
	conn  net.Conn
	lines chan []byte
	done  chan struct{}
}

// Listen holds the musica session for the terminal TUI.
func Listen() (*Server, error) {
	return listen(false)
}

// ListenWatched holds the musica session for musica serve. A client that
// writes watch receives every published line, and each line it writes after
// that arrives on Keys.
func ListenWatched() (*Server, error) {
	return listen(true)
}

func listen(watched bool) (*Server, error) {
	path, err := socketPath()
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, fmt.Errorf("create musica runtime directory: %w", err)
	}

	lock, err := acquireLock(filepath.Dir(path))
	if err != nil {
		return nil, err
	}
	if connection, err := net.DialTimeout("unix", path, 100*time.Millisecond); err == nil {
		_ = connection.Close()
		releaseLock(lock)
		return nil, ErrAlreadyRunning
	}
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		releaseLock(lock)
		return nil, fmt.Errorf("remove stale musica control socket: %w", err)
	}

	listener, err := net.Listen("unix", path)
	if err != nil {
		releaseLock(lock)
		return nil, fmt.Errorf("listen on musica control socket: %w", err)
	}
	if err := os.Chmod(path, 0o600); err != nil {
		_ = listener.Close()
		_ = os.Remove(path)
		releaseLock(lock)
		return nil, fmt.Errorf("secure musica control socket: %w", err)
	}

	server := &Server{
		listener: listener,
		lock:     lock,
		requests: make(chan struct{}, 1),
		path:     path,
		closing:  make(chan struct{}),
		watchers: map[*watcher]struct{}{},
	}
	if watched {
		server.keys = make(chan string, keyQueue)
		server.reloads = make(chan struct{}, 1)
	}
	go server.serve()
	return server, nil
}

func (s *Server) EndRequests() <-chan struct{} {
	return s.requests
}

// ReloadRequests is signaled when a client writes reload. It is nil on a
// TUI session, which answers reload with unknown command.
func (s *Server) ReloadRequests() <-chan struct{} {
	return s.reloads
}

// Keys carries the lines watchers write, one per line, without the newline.
// It is nil on a TUI session.
func (s *Server) Keys() <-chan string {
	return s.keys
}

// Watching reports whether any client is watching.
func (s *Server) Watching() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.watchers) > 0
}

// Publish queues line, without its newline, for every watcher. A watcher
// whose queue is full is dropped.
func (s *Server) Publish(line []byte) {
	framed := append(append(make([]byte, 0, len(line)+1), line...), '\n')
	s.mu.Lock()
	defer s.mu.Unlock()
	for each := range s.watchers {
		select {
		case each.lines <- framed:
		default:
			s.dropLocked(each)
		}
	}
}

// Close stops listening, sends each watcher what is queued for it, and
// releases the session.
func (s *Server) Close() {
	if s == nil {
		return
	}
	_ = s.listener.Close()
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return
	}
	s.closed = true
	watchers := make([]*watcher, 0, len(s.watchers))
	for each := range s.watchers {
		watchers = append(watchers, each)
		s.dropLocked(each)
	}
	close(s.closing)
	s.mu.Unlock()
	for _, each := range watchers {
		select {
		case <-each.done:
		case <-time.After(500 * time.Millisecond):
			_ = each.conn.Close()
		}
	}
	_ = os.Remove(s.path)
	releaseLock(s.lock)
}

func acquireLock(directory string) (*os.File, error) {
	lock, err := os.OpenFile(filepath.Join(directory, controlLockName), os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, fmt.Errorf("open musica session lock: %w", err)
	}
	if err := syscall.Flock(int(lock.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		_ = lock.Close()
		if errors.Is(err, syscall.EWOULDBLOCK) {
			return nil, ErrAlreadyRunning
		}
		return nil, fmt.Errorf("lock musica session: %w", err)
	}
	return lock, nil
}

func releaseLock(lock *os.File) {
	if lock == nil {
		return
	}
	_ = syscall.Flock(int(lock.Fd()), syscall.LOCK_UN)
	_ = lock.Close()
}

func (s *Server) serve() {
	for {
		connection, err := s.listener.Accept()
		if err != nil {
			return
		}
		go s.handle(connection)
	}
}

func (s *Server) handle(connection net.Conn) {
	_ = connection.SetDeadline(time.Now().Add(2 * time.Second))
	reader := bufio.NewReader(connection)
	command, err := reader.ReadString('\n')
	if err != nil {
		_ = connection.Close()
		return
	}
	switch strings.TrimSpace(command) {
	case endCommand:
		select {
		case s.requests <- struct{}{}:
		default:
		}
		_, _ = fmt.Fprintln(connection, "ok")
	case watchCommand:
		if s.keys != nil {
			s.watch(connection, reader)
			return
		}
		_, _ = fmt.Fprintln(connection, "unknown command")
	case reloadCommand:
		if s.reloads == nil {
			_, _ = fmt.Fprintln(connection, "unknown command")
			break
		}
		select {
		case s.reloads <- struct{}{}:
		default:
		}
		_, _ = fmt.Fprintln(connection, "ok")
	default:
		_, _ = fmt.Fprintln(connection, "unknown command")
	}
	_ = connection.Close()
}

// watch streams published lines to connection and passes the lines it
// writes to Keys until either side closes.
func (s *Server) watch(connection net.Conn, reader *bufio.Reader) {
	_ = connection.SetDeadline(time.Time{})
	each := &watcher{conn: connection, lines: make(chan []byte, watchQueue), done: make(chan struct{})}
	if !s.add(each) {
		_ = connection.Close()
		return
	}
	go each.write()
	for {
		line, err := reader.ReadString('\n')
		if key := strings.TrimSpace(line); key != "" && !s.sendKey(key) {
			break
		}
		if err != nil {
			break
		}
	}
	s.drop(each)
}

func (s *Server) sendKey(key string) bool {
	select {
	case s.keys <- key:
		return true
	case <-s.closing:
		return false
	}
}

func (s *Server) add(each *watcher) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return false
	}
	s.watchers[each] = struct{}{}
	return true
}

func (s *Server) drop(each *watcher) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.dropLocked(each)
}

// dropLocked stops queueing for a watcher. Its writer sends what is queued
// and closes the connection.
func (s *Server) dropLocked(each *watcher) {
	if _, ok := s.watchers[each]; !ok {
		return
	}
	delete(s.watchers, each)
	close(each.lines)
}

func (w *watcher) write() {
	defer close(w.done)
	defer func() { _ = w.conn.Close() }()
	for line := range w.lines {
		_ = w.conn.SetWriteDeadline(time.Now().Add(2 * time.Second))
		if _, err := w.conn.Write(line); err != nil {
			_ = w.conn.Close()
			for range w.lines {
			}
			return
		}
	}
}

// End ends the running session and returns once the session has let go of
// its lock, so that musica serve can start at once. Before it sends end, it
// notes the track the session's ffplay has loaded for the next musica serve.
func End() error {
	path, err := socketPath()
	if err != nil {
		return err
	}
	connection, err := net.DialTimeout("unix", path, 500*time.Millisecond)
	if err != nil {
		return errors.New("musica is not running")
	}
	defer connection.Close()
	keepHeld(filepath.Dir(path), connection)
	_ = connection.SetDeadline(time.Now().Add(2 * time.Second))
	if _, err := fmt.Fprintln(connection, endCommand); err != nil {
		return fmt.Errorf("send musica end command: %w", err)
	}
	response, err := bufio.NewReader(connection).ReadString('\n')
	if err != nil {
		return fmt.Errorf("read musica end response: %w", err)
	}
	if strings.TrimSpace(response) != "ok" {
		return fmt.Errorf("musica rejected end command: %s", strings.TrimSpace(response))
	}
	return waitReleased(filepath.Dir(path))
}

func socketPath() (string, error) {
	runtimeDir := strings.TrimSpace(os.Getenv("XDG_RUNTIME_DIR"))
	if runtimeDir == "" {
		runtimeDir = filepath.Join(os.TempDir(), fmt.Sprintf("musica-%d", os.Getuid()))
	}
	if !filepath.IsAbs(runtimeDir) {
		return "", errors.New("XDG_RUNTIME_DIR must be an absolute path")
	}
	return filepath.Join(runtimeDir, "musica", controlSocketName), nil
}
