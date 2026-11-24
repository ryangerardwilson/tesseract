package snapshot

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const databaseName = "tesseract"

const legacyDatabaseName = "tm"

type sessionRow struct {
	ID                   int64  `json:"id"`
	Name                 string `json:"name"`
	LastAccessed         *int64 `json:"last_accessed"`
	ActiveWindowPosition int    `json:"active_window_position"`
}

type windowRow struct {
	ID                 int64  `json:"id"`
	SessionID          int64  `json:"session_id"`
	Name               string `json:"name"`
	Layout             string `json:"layout"`
	ActivePanePosition int    `json:"active_pane_position"`
}

type paneRow struct {
	WindowID     int64   `json:"window_id"`
	CWD          string  `json:"cwd"`
	Kind         string  `json:"kind"`
	ResumeID     *string `json:"resume_id"`
	CodexProfile *string `json:"codex_profile"`
	Screen       *string `json:"screen"`
}

func openDB(home string) (string, error) {
	_ = home
	path := dbFile(databaseName)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return "", err
	}
	if err := migrateLegacyDatabase(path); err != nil {
		return "", err
	}
	if err := ensure(path); err != nil {
		return "", err
	}
	return path, nil
}

func migrateLegacyDatabase(path string) error {
	if err := adoptSQLite(filepath.Dir(path), legacyDatabaseName, path); err != nil {
		return fmt.Errorf("Unable to move the tm sqlite database: %w", err)
	}
	return nil
}

func importLegacySnapshot(home, dbPath string) error {
	imported, err := snapshotSaved(dbPath)
	if err != nil {
		return fmt.Errorf("Unable to read saved sessions: %w", err)
	}
	if imported {
		return nil
	}
	count, err := sessionCount(dbPath)
	if err != nil {
		return fmt.Errorf("Unable to read saved sessions: %w", err)
	}
	if count > 0 {
		return nil
	}
	data, err := os.ReadFile(Path(home))
	if err != nil && os.IsNotExist(err) {
		data, err = os.ReadFile(filepath.Join(home, ".tm", "state", FileName))
	}
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("Unable to read saved sessions: %w", err)
	}
	var saved SavedSnapshot
	if err := json.Unmarshal(data, &saved); err != nil {
		return fmt.Errorf("Unable to read saved sessions: %w", err)
	}
	if saved.Version != Version {
		return fmt.Errorf("Unable to read saved sessions: unsupported snapshot version %d", saved.Version)
	}
	if err := replaceSnapshot(dbPath, &saved); err != nil {
		return fmt.Errorf("Unable to read saved sessions: %w", err)
	}
	return nil
}

// replaceSnapshot saves snapshot as the one saved snapshot. A session that
// stays saved keeps its row, so the agent state RecordAgentActivity keeps for
// it stays too. Windows and panes are written again.
func replaceSnapshot(dbPath string, snapshot *SavedSnapshot) error {
	var b strings.Builder
	b.WriteString("BEGIN IMMEDIATE;\n")
	fmt.Fprintf(&b, "INSERT INTO saved_snapshot(id, version, captured_at) VALUES (1, %d, %d)\n"+
		"ON CONFLICT(id) DO UPDATE SET version = excluded.version, captured_at = excluded.captured_at;\n",
		snapshot.Version, snapshot.CapturedAt)
	b.WriteString("DELETE FROM window_snapshot;\n")
	writeSessionRows(&b, snapshot)
	writeWindowRows(&b, snapshot)
	writePaneRows(&b, snapshot)
	b.WriteString("COMMIT;\n")
	return execSQL(dbPath, b.String())
}

func writeSessionRows(b *strings.Builder, snapshot *SavedSnapshot) {
	names := make([]string, 0, len(snapshot.Sessions))
	for _, session := range snapshot.Sessions {
		names = append(names, sqlString(session.Name))
	}
	fmt.Fprintf(b, "DELETE FROM session_snapshot WHERE name NOT IN (%s);\n", strings.Join(names, ", "))
	if len(snapshot.Sessions) == 0 {
		return
	}
	b.WriteString("INSERT INTO session_snapshot(name, position, active_window_position, last_accessed) VALUES\n")
	for position, session := range snapshot.Sessions {
		if position > 0 {
			b.WriteString(",\n")
		}
		fmt.Fprintf(b, "(%s, %d, %d, %s)",
			sqlString(session.Name),
			position,
			session.ActiveWindowPosition,
			sqlIntOrNull(session.LastAccessed),
		)
	}
	b.WriteString("\nON CONFLICT(name) DO UPDATE SET position = excluded.position, " +
		"active_window_position = excluded.active_window_position, last_accessed = excluded.last_accessed;\n")
}

func writeWindowRows(b *strings.Builder, snapshot *SavedSnapshot) {
	first := true
	for _, session := range snapshot.Sessions {
		for position, window := range session.Windows {
			if first {
				b.WriteString("INSERT INTO window_snapshot(session_id, position, name, layout, active_pane_position) VALUES\n")
				first = false
			} else {
				b.WriteString(",\n")
			}
			fmt.Fprintf(b, "((SELECT id FROM session_snapshot WHERE name = %s), %d, %s, %s, %d)",
				sqlString(session.Name),
				position,
				sqlString(window.Name),
				sqlString(window.Layout),
				window.ActivePanePosition,
			)
		}
	}
	if !first {
		b.WriteString(";\n")
	}
}

func writePaneRows(b *strings.Builder, snapshot *SavedSnapshot) {
	first := true
	for _, session := range snapshot.Sessions {
		for windowPosition, window := range session.Windows {
			for position, pane := range window.Panes {
				if first {
					b.WriteString("INSERT INTO pane_snapshot(window_id, position, cwd, kind, resume_id, codex_profile, screen) VALUES\n")
					first = false
				} else {
					b.WriteString(",\n")
				}
				fmt.Fprintf(b, "(%s, %d, %s, %s, %s, %s, %s)",
					windowIDSQL(session.Name, windowPosition),
					position,
					sqlString(pane.CWD),
					sqlString(pane.Kind),
					sqlTextOrNull(pane.ResumeID),
					sqlTextOrNull(pane.CodexProfile),
					sqlTextOrNull(pane.Screen),
				)
			}
		}
	}
	if !first {
		b.WriteString(";\n")
	}
}

func windowIDSQL(sessionName string, position int) string {
	return fmt.Sprintf("(SELECT w.id FROM window_snapshot AS w JOIN session_snapshot AS s ON s.id = w.session_id WHERE s.name = %s AND w.position = %d)",
		sqlString(sessionName), position)
}

func readSavedSnapshot(dbPath string) (*SavedSnapshot, error) {
	version, capturedAt, hasHeader, err := readHeader(dbPath)
	if err != nil {
		return nil, err
	}
	sessions, err := readSessions(dbPath)
	if err != nil {
		return nil, err
	}
	if !hasHeader && len(sessions) == 0 {
		return nil, nil
	}
	if !hasHeader {
		return nil, fmt.Errorf("Unable to read saved sessions: missing snapshot header")
	}
	if version != Version {
		return nil, fmt.Errorf("Unable to read saved sessions: unsupported snapshot version %d", version)
	}
	return &SavedSnapshot{
		Version:    version,
		CapturedAt: capturedAt,
		Sessions:   sessions,
	}, nil
}

func readHeader(dbPath string) (int, int64, bool, error) {
	var rows []struct {
		Version    int   `json:"version"`
		CapturedAt int64 `json:"captured_at"`
	}
	if err := sqliteQueryJSON(dbPath, "SELECT version, captured_at FROM saved_snapshot WHERE id = 1", &rows); err != nil {
		return 0, 0, false, fmt.Errorf("Unable to read saved sessions: %w", err)
	}
	if len(rows) == 0 {
		return 0, 0, false, nil
	}
	return rows[0].Version, rows[0].CapturedAt, true, nil
}

func readSessions(dbPath string) ([]SessionSnapshot, error) {
	var sessions []sessionRow
	if err := sqliteQueryJSON(dbPath, "SELECT id, name, last_accessed, active_window_position FROM session_snapshot ORDER BY position, id", &sessions); err != nil {
		return nil, fmt.Errorf("Unable to read saved sessions: %w", err)
	}
	var windows []windowRow
	if err := sqliteQueryJSON(dbPath, "SELECT id, session_id, name, layout, active_pane_position FROM window_snapshot ORDER BY session_id, position", &windows); err != nil {
		return nil, fmt.Errorf("Unable to read saved sessions: %w", err)
	}
	var panes []paneRow
	if err := sqliteQueryJSON(dbPath, "SELECT window_id, cwd, kind, resume_id, codex_profile, screen FROM pane_snapshot ORDER BY window_id, position", &panes); err != nil {
		return nil, fmt.Errorf("Unable to read saved sessions: %w", err)
	}
	panesByWindow := map[int64][]PaneSnapshot{}
	for _, row := range panes {
		panesByWindow[row.WindowID] = append(panesByWindow[row.WindowID], PaneSnapshot{
			CWD:          row.CWD,
			Kind:         row.Kind,
			ResumeID:     deref(row.ResumeID),
			CodexProfile: deref(row.CodexProfile),
			Screen:       deref(row.Screen),
		})
	}
	windowsBySession := map[int64][]WindowSnapshot{}
	for _, row := range windows {
		windowsBySession[row.SessionID] = append(windowsBySession[row.SessionID], WindowSnapshot{
			Name:               row.Name,
			Layout:             row.Layout,
			ActivePanePosition: row.ActivePanePosition,
			Panes:              panesByWindow[row.ID],
		})
	}
	out := make([]SessionSnapshot, 0, len(sessions))
	for _, row := range sessions {
		out = append(out, SessionSnapshot{
			Name:                 row.Name,
			LastAccessed:         derefInt(row.LastAccessed),
			ActiveWindowPosition: row.ActiveWindowPosition,
			Windows:              windowsBySession[row.ID],
		})
	}
	return out, nil
}

func deref(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func sessionCount(dbPath string) (int, error) {
	var rows []struct {
		N int `json:"n"`
	}
	if err := sqliteQueryJSON(dbPath, "SELECT COUNT(*) AS n FROM session_snapshot", &rows); err != nil {
		return 0, err
	}
	if len(rows) == 0 {
		return 0, nil
	}
	return rows[0].N, nil
}

// snapshotSaved reports whether any snapshot, captured or imported, was saved.
func snapshotSaved(dbPath string) (bool, error) {
	var rows []struct {
		N int `json:"n"`
	}
	if err := sqliteQueryJSON(dbPath, "SELECT COUNT(*) AS n FROM saved_snapshot", &rows); err != nil {
		return false, err
	}
	return len(rows) > 0 && rows[0].N > 0, nil
}

func ensure(dbPath string) error {
	if err := prepareAppDB(dbPath, databaseName, ormText()); err != nil {
		return err
	}
	return linkTablesByID(dbPath)
}

func sqlIntOrNull(value int64) string {
	if value == 0 {
		return "NULL"
	}
	return strconv.FormatInt(value, 10)
}

func sqlTextOrNull(value string) string {
	if value == "" {
		return "NULL"
	}
	return sqlString(value)
}

func sqlString(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "''") + "'"
}

func execSQL(dbPath, sql string) error {
	if err := sqliteExec(dbPath, sql); err != nil {
		return err
	}
	count, err := sessionCount(dbPath)
	if err != nil {
		return err
	}
	if count == 0 {
		return nil
	}
	publishObject(dbPath, databaseName)
	return nil
}
