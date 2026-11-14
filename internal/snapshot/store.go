package snapshot

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

const databaseName = "tesseract"

const legacyDatabaseName = "tm"

const importedSnapshotKey = "imported_snapshot"

const metaModel = "model meta\n" +
	"  id integer primary key\n" +
	"  key text not null\n" +
	"  value text\n"

const sessionModel = "model session\n" +
	"  id integer primary key\n" +
	"  name text not null\n" +
	"  last_accessed integer\n" +
	"  active_window_position integer not null\n" +
	"  position integer not null\n"

const windowModel = "model window\n" +
	"  id integer primary key\n" +
	"  session_name text not null\n" +
	"  position integer not null\n" +
	"  name text not null\n" +
	"  layout text not null\n" +
	"  active_pane_position integer not null\n"

const paneModel = "model pane\n" +
	"  id integer primary key\n" +
	"  session_name text not null\n" +
	"  window_position integer not null\n" +
	"  position integer not null\n" +
	"  cwd text not null\n" +
	"  kind text not null\n" +
	"  codex_thread_id text\n" +
	"  codex_profile text\n" +
	"  screen text\n"

type sessionRow struct {
	Name                 string `json:"name"`
	LastAccessed         *int64 `json:"last_accessed"`
	ActiveWindowPosition int    `json:"active_window_position"`
	Position             int    `json:"position"`
}

type windowRow struct {
	SessionName        string `json:"session_name"`
	Position           int    `json:"position"`
	Name               string `json:"name"`
	Layout             string `json:"layout"`
	ActivePanePosition int    `json:"active_pane_position"`
}

type paneRow struct {
	SessionName    string  `json:"session_name"`
	WindowPosition int     `json:"window_position"`
	Position       int     `json:"position"`
	CWD            string  `json:"cwd"`
	Kind           string  `json:"kind"`
	CodexThreadID  *string `json:"codex_thread_id"`
	CodexProfile   *string `json:"codex_profile"`
	Screen         *string `json:"screen"`
}

func openDB(home string) (string, error) {
	if err := migrateLegacyDatabase(home); err != nil {
		return "", err
	}
	if err := ensure(); err != nil {
		return "", err
	}
	return databaseName, nil
}

// migrateLegacyDatabase renames the sqlx database tm to tesseract once.
// Rows, including a live Grok session, stay in the database.
func migrateLegacyDatabase(home string) error {
	_ = home
	if _, err := sqlxCall("rename", "database", legacyDatabaseName, databaseName); err != nil {
		return fmt.Errorf("Unable to move the tm sql database: %w", err)
	}
	return nil
}

// importLegacySnapshot copies ~/.tesseract/state/session_snapshot.json once.
// imported_snapshot stays set so a later empty database does not read that file again.
func importLegacySnapshot(home, dbPath string) error {
	imported, err := metaHas(dbPath, importedSnapshotKey)
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

func replaceSnapshot(dbPath string, snapshot *SavedSnapshot) error {
	var b strings.Builder
	b.WriteString("BEGIN IMMEDIATE;\n")
	b.WriteString("DELETE FROM pane;\nDELETE FROM \"window\";\nDELETE FROM session;\nDELETE FROM meta;\n")
	writeSessionRows(&b, snapshot)
	writeWindowRows(&b, snapshot)
	writePaneRows(&b, snapshot)
	fmt.Fprintf(&b, "INSERT INTO meta(key, value) VALUES (%s, %s), (%s, %s), (%s, '1');\n",
		sqlString("version"), sqlString(strconv.Itoa(snapshot.Version)),
		sqlString("captured_at"), sqlString(strconv.FormatInt(snapshot.CapturedAt, 10)),
		sqlString(importedSnapshotKey),
	)
	b.WriteString("COMMIT;\n")
	return execSQL(dbPath, b.String())
}

func writeSessionRows(b *strings.Builder, snapshot *SavedSnapshot) {
	if len(snapshot.Sessions) == 0 {
		return
	}
	b.WriteString("INSERT INTO session(name, last_accessed, active_window_position, position) VALUES\n")
	for position, session := range snapshot.Sessions {
		if position > 0 {
			b.WriteString(",\n")
		}
		fmt.Fprintf(b, "(%s, %s, %d, %d)",
			sqlString(session.Name),
			sqlIntOrNull(session.LastAccessed),
			session.ActiveWindowPosition,
			position,
		)
	}
	b.WriteString(";\n")
}

func writeWindowRows(b *strings.Builder, snapshot *SavedSnapshot) {
	first := true
	for _, session := range snapshot.Sessions {
		for position, window := range session.Windows {
			if first {
				b.WriteString("INSERT INTO \"window\"(session_name, position, name, layout, active_pane_position) VALUES\n")
				first = false
			} else {
				b.WriteString(",\n")
			}
			fmt.Fprintf(b, "(%s, %d, %s, %s, %d)",
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
					b.WriteString("INSERT INTO pane(session_name, window_position, position, cwd, kind, codex_thread_id, codex_profile, screen) VALUES\n")
					first = false
				} else {
					b.WriteString(",\n")
				}
				fmt.Fprintf(b, "(%s, %d, %d, %s, %s, %s, %s, %s)",
					sqlString(session.Name),
					windowPosition,
					position,
					sqlString(pane.CWD),
					sqlString(pane.Kind),
					sqlTextOrNull(pane.CodexThreadID),
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
		Key   string  `json:"key"`
		Value *string `json:"value"`
	}
	if err := queryJSON(dbPath, "SELECT key, value FROM meta WHERE key IN ('version', 'captured_at')", &rows); err != nil {
		return 0, 0, false, fmt.Errorf("Unable to read saved sessions: %w", err)
	}
	var versionText, capturedText string
	hasVersion := false
	for _, row := range rows {
		if row.Value == nil {
			continue
		}
		switch row.Key {
		case "version":
			versionText = *row.Value
			hasVersion = true
		case "captured_at":
			capturedText = *row.Value
		}
	}
	if !hasVersion {
		return 0, 0, false, nil
	}
	version, err := strconv.Atoi(versionText)
	if err != nil {
		return 0, 0, false, fmt.Errorf("Unable to read saved sessions: unsupported snapshot version %s", versionText)
	}
	var capturedAt int64
	if capturedText != "" {
		capturedAt, err = strconv.ParseInt(capturedText, 10, 64)
		if err != nil {
			return 0, 0, false, fmt.Errorf("Unable to read saved sessions: %w", err)
		}
	}
	return version, capturedAt, true, nil
}

func readSessions(dbPath string) ([]SessionSnapshot, error) {
	var sessions []sessionRow
	if err := queryJSON(dbPath, "SELECT name, last_accessed, active_window_position, position FROM session ORDER BY position, id", &sessions); err != nil {
		return nil, fmt.Errorf("Unable to read saved sessions: %w", err)
	}
	var windows []windowRow
	if err := queryJSON(dbPath, "SELECT session_name, position, name, layout, active_pane_position FROM \"window\" ORDER BY session_name, position, id", &windows); err != nil {
		return nil, fmt.Errorf("Unable to read saved sessions: %w", err)
	}
	var panes []paneRow
	if err := queryJSON(dbPath, "SELECT session_name, window_position, position, cwd, kind, codex_thread_id, codex_profile, screen FROM pane ORDER BY session_name, window_position, position, id", &panes); err != nil {
		return nil, fmt.Errorf("Unable to read saved sessions: %w", err)
	}
	panesByWindow := map[string][]PaneSnapshot{}
	for _, row := range panes {
		key := windowKey(row.SessionName, row.WindowPosition)
		panesByWindow[key] = append(panesByWindow[key], PaneSnapshot{
			CWD:           row.CWD,
			Kind:          row.Kind,
			CodexThreadID: deref(row.CodexThreadID),
			CodexProfile:  deref(row.CodexProfile),
			Screen:        deref(row.Screen),
		})
	}
	windowsBySession := map[string][]WindowSnapshot{}
	for _, row := range windows {
		windowsBySession[row.SessionName] = append(windowsBySession[row.SessionName], WindowSnapshot{
			Name:               row.Name,
			Layout:             row.Layout,
			ActivePanePosition: row.ActivePanePosition,
			Panes:              panesByWindow[windowKey(row.SessionName, row.Position)],
		})
	}
	out := make([]SessionSnapshot, 0, len(sessions))
	for _, row := range sessions {
		lastAccessed := int64(0)
		if row.LastAccessed != nil {
			lastAccessed = *row.LastAccessed
		}
		out = append(out, SessionSnapshot{
			Name:                 row.Name,
			LastAccessed:         lastAccessed,
			ActiveWindowPosition: row.ActiveWindowPosition,
			Windows:              windowsBySession[row.Name],
		})
	}
	return out, nil
}

func windowKey(sessionName string, position int) string {
	return sessionName + "\x00" + strconv.Itoa(position)
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
	if err := queryJSON(dbPath, "SELECT COUNT(*) AS n FROM session", &rows); err != nil {
		return 0, err
	}
	if len(rows) == 0 {
		return 0, nil
	}
	return rows[0].N, nil
}

func metaHas(dbPath, key string) (bool, error) {
	var rows []struct {
		N int `json:"n"`
	}
	if err := queryJSON(dbPath, "SELECT COUNT(*) AS n FROM meta WHERE key = "+sqlString(key), &rows); err != nil {
		return false, err
	}
	if len(rows) == 0 {
		return false, nil
	}
	return rows[0].N > 0, nil
}

func ensure() error {
	if _, err := sqlxCall("create", "database", databaseName); err != nil {
		return err
	}
	return sqlxMigrate(ormText())
}

func ormText() string {
	return metaModel + "\n" + sessionModel + "\n" + windowModel + "\n" + paneModel
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

func queryJSON(dbPath, sql string, dest any) error {
	_ = dbPath
	raw, err := sqlxCall(databaseName, "query", "--json", sql)
	if err != nil {
		return err
	}
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	if err := json.Unmarshal([]byte(raw), dest); err != nil {
		return fmt.Errorf("sqlite json: %w", err)
	}
	return nil
}

func execSQL(dbPath, sql string) error {
	_ = dbPath
	file, err := os.CreateTemp("", databaseName+"-sql-*")
	if err != nil {
		return err
	}
	sqlPath := file.Name()
	defer os.Remove(sqlPath)
	if _, err := file.WriteString(sql); err != nil {
		_ = file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	_, err = sqlxCall(databaseName, "exec", "--file", sqlPath)
	return err
}

func sqlxBin() string {
	if bin := os.Getenv("SQLX_BIN"); bin != "" {
		return bin
	}
	path, err := exec.LookPath("sql")
	if err != nil {
		return "sql"
	}
	return path
}

func sqlxCall(args ...string) (string, error) {
	cmd := exec.Command(sqlxBin(), args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = strings.TrimSpace(stdout.String())
		}
		if msg == "" {
			msg = err.Error()
		}
		return stdout.String(), fmt.Errorf("%s", msg)
	}
	return stdout.String(), nil
}

func sqlxMigrate(body string) error {
	file, err := os.CreateTemp("", databaseName+"-orm-*")
	if err != nil {
		return err
	}
	name := file.Name()
	defer os.Remove(name)
	if _, err := file.WriteString(body); err != nil {
		_ = file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	_, err = sqlxCall(databaseName, "migrate", name)
	return err
}
