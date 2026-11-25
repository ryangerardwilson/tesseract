package player

import (
	"io"
	"os/exec"
	"sync"
	"syscall"
	"time"
)

const analogPlaybackFilter = "highpass=f=45,lowpass=f=11800,equalizer=f=120:t=q:w=0.8:g=1.4,equalizer=f=3200:t=q:w=1.3:g=-1.2,acompressor=threshold=0.18:ratio=1.8:attack=18:release=220:makeup=1.5,asoftclip=type=tanh:threshold=0.95:output=0.96,alimiter=limit=0.96"

type Player struct {
	mu      sync.Mutex
	cmd     *exec.Cmd
	done    chan error
	current string
	paused  bool
	offset  time.Duration
	started time.Time
}

func New() *Player {
	return &Player{}
}

func (p *Player) Play(path string, offset time.Duration) error {
	p.Stop()

	cmd := exec.Command("ffplay", ffplayArgs(path, offset)...)
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	cmd.Stdin = nil
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	if err := cmd.Start(); err != nil {
		return err
	}

	done := make(chan error, 1)
	p.mu.Lock()
	p.cmd = cmd
	p.done = done
	p.current = path
	p.paused = false
	p.offset = offset
	p.started = time.Now()
	p.mu.Unlock()

	go func() {
		err := cmd.Wait()
		done <- err
		p.mu.Lock()
		if p.cmd == cmd {
			p.cmd = nil
			p.done = nil
			p.current = ""
			p.paused = false
			p.offset = 0
			p.started = time.Time{}
		}
		p.mu.Unlock()
	}()

	return nil
}

func (p *Player) TogglePause() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.cmd == nil || p.cmd.Process == nil {
		return false
	}
	signal := syscall.SIGSTOP
	if p.paused {
		signal = syscall.SIGCONT
		p.started = time.Now()
	} else if !p.started.IsZero() {
		p.offset += time.Since(p.started)
	}
	_ = syscall.Kill(-p.cmd.Process.Pid, signal)
	p.paused = !p.paused
	return p.paused
}

func (p *Player) Stop() {
	p.mu.Lock()
	cmd := p.cmd
	done := p.done
	p.cmd = nil
	p.done = nil
	p.current = ""
	p.paused = false
	p.offset = 0
	p.started = time.Time{}
	p.mu.Unlock()

	if cmd == nil || cmd.Process == nil {
		return
	}
	_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGTERM)
	if done != nil {
		select {
		case <-done:
		case <-time.After(700 * time.Millisecond):
			_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
			<-done
		}
	}
}

func (p *Player) Seek(delta time.Duration) error {
	path, offset := p.State()
	if path == "" {
		return nil
	}
	offset += delta
	if offset < 0 {
		offset = 0
	}
	return p.Play(path, offset)
}

func (p *Player) Restart() error {
	path, _ := p.State()
	if path == "" {
		return nil
	}
	return p.Play(path, 0)
}

func ffplayArgs(path string, offset time.Duration) []string {
	args := []string{"-nodisp", "-autoexit", "-loglevel", "quiet"}
	if offset > 0 {
		args = append(args, "-ss", secondsArg(offset))
	}
	return append(args, "-af", analogPlaybackFilter, path)
}

func (p *Player) State() (string, time.Duration) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.current == "" {
		return "", 0
	}
	elapsed := p.offset
	if !p.started.IsZero() && !p.paused {
		elapsed += time.Since(p.started)
	}
	return p.current, elapsed
}

func (p *Player) IsPlaying() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.current != "" && p.cmd != nil
}

func (p *Player) IsPaused() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.paused
}

func secondsArg(duration time.Duration) string {
	seconds := int(duration.Seconds())
	if seconds < 0 {
		seconds = 0
	}
	if seconds == 0 {
		return "0"
	}
	buf := make([]byte, 0, 8)
	for seconds > 0 {
		buf = append(buf, byte('0'+seconds%10))
		seconds /= 10
	}
	for i, j := 0, len(buf)-1; i < j; i, j = i+1, j-1 {
		buf[i], buf[j] = buf[j], buf[i]
	}
	return string(buf)
}
