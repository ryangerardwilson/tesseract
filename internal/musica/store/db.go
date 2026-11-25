package store

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"time"
)

const createSQL = `
CREATE TABLE IF NOT EXISTS downloads (
  id TEXT PRIMARY KEY,
  url TEXT NOT NULL,
  title TEXT NOT NULL,
  dest_dir TEXT NOT NULL,
  filename TEXT NOT NULL,
  status TEXT NOT NULL,
  percent REAL NOT NULL DEFAULT 0,
  speed TEXT NOT NULL DEFAULT '',
  eta TEXT NOT NULL DEFAULT '',
  error TEXT NOT NULL DEFAULT '',
  pid INTEGER NOT NULL DEFAULT 0,
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS settings (
  key TEXT PRIMARY KEY,
  value TEXT NOT NULL
);
`

// Job is one YouTube audio download. Status is queued, running, done, failed, or cancelled.
type Job struct {
	ID        string  `json:"id"`
	URL       string  `json:"url"`
	Title     string  `json:"title"`
	DestDir   string  `json:"dest_dir"`
	Filename  string  `json:"filename"`
	Status    string  `json:"status"`
	Percent   float64 `json:"percent"`
	Speed     string  `json:"speed"`
	ETA       string  `json:"eta"`
	Error     string  `json:"error"`
	PID       int     `json:"pid"`
	CreatedAt string  `json:"created_at"`
	UpdatedAt string  `json:"updated_at"`
}

// DB is the application sqlite file. When sync is set, nebula keep stores a copy.
type DB struct {
	path string
	sync bool
}

func Open() (*DB, error) {
	path, syncRemote := locate()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, err
	}
	if err := ensureFile(path, syncRemote); err != nil {
		return nil, err
	}
	db := &DB{path: path, sync: syncRemote}
	if err := sqliteExec(path, createSQL); err != nil {
		return nil, err
	}
	if err := os.Chmod(path, 0o600); err != nil {
		return nil, err
	}
	return db, nil
}

// ensureFile makes a missing database file. The runtime file comes back
// through nebula keep, which makes an empty one only when no backup exists.
// When the backup is out of reach keep makes no file and neither does musica,
// so an empty database never stands in for the backup. MUSICA_DB and
// SQLX_ROOT start empty.
func ensureFile(path string, syncRemote bool) error {
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		return err
	}
	if !syncRemote || keepSkipped() {
		return os.WriteFile(path, nil, 0o600)
	}
	if err := keepRestore(path); err != nil {
		return err
	}
	if _, err := os.Stat(path); err != nil {
		return fmt.Errorf("nebula keep restore %s: no file", keepName)
	}
	return nil
}

func FilePath() string {
	path, _ := locate()
	return path
}

func locate() (string, bool) {
	if path := os.Getenv("MUSICA_DB"); path != "" {
		return path, false
	}
	if root := os.Getenv("SQLX_ROOT"); root != "" {
		return filepath.Join(root, "data", "musica.sqlite"), false
	}
	base := os.Getenv("XDG_RUNTIME_DIR")
	if base == "" {
		base = filepath.Join("/dev/shm", fmt.Sprintf("musica-%d", os.Getuid()))
	} else {
		base = filepath.Join(base, "musica")
	}
	return filepath.Join(base, "musica.sqlite"), true
}

// publish tells nebula keep the file changed. The upload follows in nebula's
// own worker, and the commit stands whether or not the mark lands.
func (db *DB) publish() {
	if db == nil || !db.sync {
		return
	}
	_ = keepMark(db.path)
}

func (db *DB) InsertJob(url, title, destDir, filename string) (Job, error) {
	now := stamp()
	id := fmt.Sprintf("%d", time.Now().UnixNano())
	statement := fmt.Sprintf(
		`INSERT INTO downloads (id, url, title, dest_dir, filename, status, percent, speed, eta, error, pid, created_at, updated_at) VALUES (%s, %s, %s, %s, %s, 'queued', 0, '', '', '', 0, %s, %s)`,
		quote(id), quote(url), quote(title), quote(destDir), quote(filename), quote(now), quote(now),
	)
	if err := sqliteExec(db.path, statement); err != nil {
		return Job{}, err
	}
	db.publish()
	job, ok, err := db.Get(id)
	if err != nil {
		return Job{}, err
	}
	if !ok {
		return Job{}, fmt.Errorf("inserted download %s was not readable", id)
	}
	return job, nil
}

func (db *DB) List() ([]Job, error) {
	return queryJobs(db.path, `SELECT * FROM downloads ORDER BY created_at ASC`)
}

func (db *DB) Get(id string) (Job, bool, error) {
	jobs, err := queryJobs(db.path, `SELECT * FROM downloads WHERE id = `+quote(id))
	if err != nil {
		return Job{}, false, err
	}
	if len(jobs) == 0 {
		return Job{}, false, nil
	}
	return jobs[0], true, nil
}

func (db *DB) MarkProgress(id string, percent float64, speed, eta string) error {
	statement := fmt.Sprintf(
		`UPDATE downloads SET percent = %f, speed = %s, eta = %s, updated_at = %s WHERE id = %s`,
		percent, quote(speed), quote(eta), quote(stamp()), quote(id),
	)
	return sqliteExec(db.path, statement)
}

func (db *DB) SetPID(id string, pid int) error {
	statement := fmt.Sprintf(
		`UPDATE downloads SET pid = %d, updated_at = %s WHERE id = %s`,
		pid, quote(stamp()), quote(id),
	)
	return sqliteExec(db.path, statement)
}

func (db *DB) MarkStatus(id, status, errText string) error {
	statement := fmt.Sprintf(
		`UPDATE downloads SET status = %s, error = %s, updated_at = %s WHERE id = %s`,
		quote(status), quote(errText), quote(stamp()), quote(id),
	)
	if err := sqliteExec(db.path, statement); err != nil {
		return err
	}
	db.publish()
	return nil
}

func (db *DB) ClaimNext() (Job, bool, error) {
	statement := fmt.Sprintf(
		`UPDATE downloads SET status = 'running', updated_at = %s WHERE id = (SELECT id FROM downloads WHERE status = 'queued' ORDER BY created_at ASC LIMIT 1)`,
		quote(stamp()),
	)
	if err := sqliteExec(db.path, statement); err != nil {
		return Job{}, false, err
	}
	jobs, err := queryJobs(db.path, `SELECT * FROM downloads WHERE status = 'running' AND pid = 0 ORDER BY updated_at DESC LIMIT 1`)
	if err != nil {
		return Job{}, false, err
	}
	if len(jobs) == 0 {
		return Job{}, false, nil
	}
	return jobs[0], true, nil
}

func (db *DB) RequeueDead() error {
	jobs, err := queryJobs(db.path, `SELECT * FROM downloads WHERE status = 'running'`)
	if err != nil {
		return err
	}
	for _, job := range jobs {
		if processAlive(job.PID) {
			continue
		}
		statement := fmt.Sprintf(
			`UPDATE downloads SET status = 'queued', pid = 0, updated_at = %s WHERE id = %s`,
			quote(stamp()), quote(job.ID),
		)
		if err := sqliteExec(db.path, statement); err != nil {
			return err
		}
	}
	return nil
}

func (db *DB) PutSetting(key, value string) error {
	statement := fmt.Sprintf(
		`INSERT INTO settings (key, value) VALUES (%s, %s) ON CONFLICT(key) DO UPDATE SET value = excluded.value`,
		quote(key), quote(value),
	)
	if err := sqliteExec(db.path, statement); err != nil {
		return err
	}
	db.publish()
	return nil
}

func (db *DB) Setting(key string) (string, bool, error) {
	raw, err := sqliteRun(db.path, `SELECT value FROM settings WHERE key = `+quote(key), true)
	if err != nil {
		return "", false, err
	}
	raw = trimJSON(raw)
	if raw == "" {
		return "", false, nil
	}
	var rows []struct {
		Value string `json:"value"`
	}
	if err := unmarshalSetting(raw, &rows); err != nil {
		return "", false, err
	}
	if len(rows) == 0 {
		return "", false, nil
	}
	return rows[0].Value, true, nil
}

func RememberMusicDir(dir string) error {
	if dir == "" {
		return nil
	}
	db, err := Open()
	if err != nil {
		return err
	}
	return db.PutSetting("music_dir", dir)
}

func SavedMusicDir() (string, bool) {
	db, err := Open()
	if err != nil {
		return "", false
	}
	value, ok, err := db.Setting("music_dir")
	if err != nil || !ok {
		return "", false
	}
	return value, value != ""
}

func processAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	return syscall.Kill(pid, 0) == nil
}

func stamp() string {
	return time.Now().UTC().Format(time.RFC3339Nano)
}
