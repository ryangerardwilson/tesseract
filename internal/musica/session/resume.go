package session

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

// resumeName is the note a reload leaves beside the lock: the track, how far
// into it, whether it was paused, and whether the cassette menu was open.
// The next musica serve takes it and continues from there.
const resumeName = "resume"

// Resume is where playback continues after a reload onto the latest binary.
type Resume struct {
	Path   string        `json:"path"`
	Offset time.Duration `json:"offset"`
	Paused bool          `json:"paused"`
	Menu   bool          `json:"menu"`
}

// TakeResume returns the note a reload left, and forgets it. It returns
// false when there is none.
func (s *Server) TakeResume() (Resume, bool) {
	directory := filepath.Dir(s.path)
	note, ok := takeResume(directory)
	if ok {
		_ = os.Remove(filepath.Join(directory, heldName))
	}
	return note, ok
}

// SaveResume notes where this session is, for the next musica serve. It
// forgets an older held-track note so that resume wins.
func (s *Server) SaveResume(note Resume) error {
	return writeResume(filepath.Dir(s.path), note)
}

// WriteResume notes where playback continues. An empty directory uses the
// musica runtime directory. It forgets an older held-track note.
func WriteResume(directory string, note Resume) error {
	if directory == "" {
		path, err := socketPath()
		if err != nil {
			return err
		}
		directory = filepath.Dir(path)
	}
	return writeResume(directory, note)
}

func writeResume(directory string, note Resume) error {
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return err
	}
	_ = os.Remove(filepath.Join(directory, heldName))
	data, err := json.Marshal(note)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(directory, resumeName), append(data, '\n'), 0o600)
}

func takeResume(directory string) (Resume, bool) {
	notePath := filepath.Join(directory, resumeName)
	data, err := os.ReadFile(notePath)
	if err != nil {
		return Resume{}, false
	}
	_ = os.Remove(notePath)
	var note Resume
	if err := json.Unmarshal(data, &note); err != nil || note.Path == "" {
		return Resume{}, false
	}
	return note, true
}
