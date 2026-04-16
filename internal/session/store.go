package session

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	_ "modernc.org/sqlite"
)

type EventKind string

const (
	KindUserMsg      EventKind = "user_msg"
	KindLLMRequest   EventKind = "llm_req"
	KindLLMResponse  EventKind = "llm_resp"
	KindToolCall     EventKind = "tool_call"
	KindToolResult   EventKind = "tool_result"
	KindFileSnapshot EventKind = "file_snapshot"
	KindClock        EventKind = "clock"
	KindRand         EventKind = "rand"
	KindAgentFinal   EventKind = "agent_final"
)

type Session struct {
	ID        string
	Seed      int64
	Model     string
	CreatedAt time.Time
	Cwd       string
	Env       map[string]string
	Goal      string
}

type Event struct {
	Step    int
	Kind    EventKind
	Payload []byte
	TS      time.Time
}

type Store struct {
	db   *sql.DB
	path string
}

const schema = `
CREATE TABLE IF NOT EXISTS sessions (
    id TEXT PRIMARY KEY,
    seed INTEGER NOT NULL,
    model TEXT NOT NULL,
    created_at INTEGER NOT NULL,
    cwd TEXT NOT NULL,
    env_json TEXT NOT NULL,
    goal TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS events (
    session_id TEXT NOT NULL,
    step INTEGER NOT NULL,
    kind TEXT NOT NULL,
    payload BLOB NOT NULL,
    ts INTEGER NOT NULL,
    PRIMARY KEY (session_id, step)
);
CREATE INDEX IF NOT EXISTS idx_events_kind ON events(session_id, kind);
`

func Open(path string) (*Store, error) {
	db, err := sql.Open("sqlite", path+"?_pragma=journal_mode(WAL)&_pragma=synchronous(NORMAL)")
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, fmt.Errorf("init schema: %w", err)
	}
	return &Store{db: db, path: path}, nil
}

func (s *Store) Close() error { return s.db.Close() }

func (s *Store) Path() string { return s.path }

func (s *Store) CreateSession(sess Session) error {
	envJSON, err := json.Marshal(sess.Env)
	if err != nil {
		return err
	}
	_, err = s.db.Exec(
		`INSERT INTO sessions(id, seed, model, created_at, cwd, env_json, goal) VALUES (?,?,?,?,?,?,?)`,
		sess.ID, sess.Seed, sess.Model, sess.CreatedAt.UnixNano(), sess.Cwd, string(envJSON), sess.Goal,
	)
	return err
}

func (s *Store) GetSession(id string) (*Session, error) {
	row := s.db.QueryRow(`SELECT id, seed, model, created_at, cwd, env_json, goal FROM sessions WHERE id = ?`, id)
	var sess Session
	var createdAtNs int64
	var envJSON string
	if err := row.Scan(&sess.ID, &sess.Seed, &sess.Model, &createdAtNs, &sess.Cwd, &envJSON, &sess.Goal); err != nil {
		return nil, err
	}
	sess.CreatedAt = time.Unix(0, createdAtNs)
	if err := json.Unmarshal([]byte(envJSON), &sess.Env); err != nil {
		return nil, err
	}
	return &sess, nil
}

func (s *Store) LatestSessionID() (string, error) {
	row := s.db.QueryRow(`SELECT id FROM sessions ORDER BY created_at DESC LIMIT 1`)
	var id string
	if err := row.Scan(&id); err != nil {
		return "", err
	}
	return id, nil
}

func (s *Store) ListSessions() ([]Session, error) {
	rows, err := s.db.Query(`SELECT id, seed, model, created_at, cwd, env_json, goal FROM sessions ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Session
	for rows.Next() {
		var sess Session
		var createdAtNs int64
		var envJSON string
		if err := rows.Scan(&sess.ID, &sess.Seed, &sess.Model, &createdAtNs, &sess.Cwd, &envJSON, &sess.Goal); err != nil {
			return nil, err
		}
		sess.CreatedAt = time.Unix(0, createdAtNs)
		_ = json.Unmarshal([]byte(envJSON), &sess.Env)
		out = append(out, sess)
	}
	return out, rows.Err()
}

func (s *Store) AppendEvent(sessionID string, ev Event) error {
	_, err := s.db.Exec(
		`INSERT INTO events(session_id, step, kind, payload, ts) VALUES (?,?,?,?,?)`,
		sessionID, ev.Step, string(ev.Kind), ev.Payload, ev.TS.UnixNano(),
	)
	return err
}

func (s *Store) LoadEvents(sessionID string) ([]Event, error) {
	rows, err := s.db.Query(`SELECT step, kind, payload, ts FROM events WHERE session_id = ? ORDER BY step ASC`, sessionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Event
	for rows.Next() {
		var ev Event
		var kind string
		var tsNs int64
		if err := rows.Scan(&ev.Step, &kind, &ev.Payload, &tsNs); err != nil {
			return nil, err
		}
		ev.Kind = EventKind(kind)
		ev.TS = time.Unix(0, tsNs)
		out = append(out, ev)
	}
	return out, rows.Err()
}

func (s *Store) NextStep(sessionID string) (int, error) {
	row := s.db.QueryRow(`SELECT COALESCE(MAX(step)+1, 0) FROM events WHERE session_id = ?`, sessionID)
	var step int
	if err := row.Scan(&step); err != nil {
		return 0, err
	}
	return step, nil
}
