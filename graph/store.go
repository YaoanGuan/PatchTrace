package graph

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

// tsLayout is a fixed-width UTC timestamp layout. Fixed width keeps SQLite
// string comparisons (used for lease expiry) chronologically correct.
const tsLayout = "2006-01-02T15:04:05.000000000Z07:00"

// defaultIntentLease is used when an intent is declared already claimed.
const defaultIntentLease = 5 * time.Minute

var validEdgeKinds = map[string]bool{
	"derive":  true,
	"support": true,
	"refute":  true,
	"block":   true,
	"produce": true,
}

// querier is implemented by both *sql.DB and *sql.Tx.
type querier interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

// Open opens (or creates) the graph database at path.
func Open(path string) (*Store, error) {
	if strings.TrimSpace(path) == "" {
		return nil, fmt.Errorf("%w: database path is required", ErrInvalid)
	}
	if dir := filepath.Dir(path); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, fmt.Errorf("graph: create data directory: %w", err)
		}
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("graph: open sqlite: %w", err)
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	if err := migrate(db); err != nil {
		_ = db.Close()
		return nil, err
	}
	return &Store{db: db}, nil
}

// Close releases the database handle. It is safe to call more than once.
func (s *Store) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	err := s.db.Close()
	s.db = nil
	return err
}

func formatTime(t time.Time) string {
	return t.UTC().Format(tsLayout)
}

func parseTime(s string) (time.Time, error) {
	return time.Parse(tsLayout, s)
}

func nullTime(ns sql.NullString) (*time.Time, error) {
	if !ns.Valid || ns.String == "" {
		return nil, nil
	}
	t, err := parseTime(ns.String)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

func timeValue(t *time.Time) any {
	if t == nil {
		return nil
	}
	return formatTime(*t)
}

// cleanupLeases reopens expired intent claims and clears expired reason
// leases. Every read path calls it so callers never observe a stale owner.
func (s *Store) cleanupLeases(ctx context.Context, projectID string) error {
	now := formatTime(time.Now())

	query := `UPDATE nodes SET worker=NULL, state='open', lease_until=NULL
		WHERE kind='intent' AND state='claimed' AND lease_until IS NOT NULL AND lease_until < ?`
	args := []any{now}
	if projectID != "" {
		query += ` AND project_id=?`
		args = append(args, projectID)
	}
	if _, err := s.db.ExecContext(ctx, query, args...); err != nil {
		return fmt.Errorf("graph: reclaim expired intent leases: %w", err)
	}

	reasonQuery := `UPDATE projects
		SET reason_worker=NULL, reason_trigger=NULL, reason_started_at=NULL,
		    reason_heartbeat_at=NULL, reason_expire_at=NULL
		WHERE reason_worker IS NOT NULL AND reason_worker <> ''
		  AND reason_expire_at IS NOT NULL AND reason_expire_at < ?`
	reasonArgs := []any{now}
	if projectID != "" {
		reasonQuery += ` AND id=?`
		reasonArgs = append(reasonArgs, projectID)
	}
	if _, err := s.db.ExecContext(ctx, reasonQuery, reasonArgs...); err != nil {
		return fmt.Errorf("graph: reclaim expired reason leases: %w", err)
	}
	return nil
}

const projectColumns = `id, title, origin, goal, status, bootstrap_enabled, created_at,
	reason_worker, reason_trigger, reason_started_at, reason_heartbeat_at`

func scanProject(sc interface{ Scan(dest ...any) error }) (Project, error) {
	var (
		p         Project
		bootstrap int
		createdAt string
		rw        sql.NullString
		trigger   sql.NullString
		startedAt sql.NullString
		heartbeat sql.NullString
	)
	if err := sc.Scan(&p.ID, &p.Title, &p.Origin, &p.Goal, &p.Status, &bootstrap,
		&createdAt, &rw, &trigger, &startedAt, &heartbeat); err != nil {
		return Project{}, err
	}
	t, err := parseTime(createdAt)
	if err != nil {
		return Project{}, fmt.Errorf("graph: parse project created_at: %w", err)
	}
	p.CreatedAt = t
	p.BootstrapEnabled = bootstrap != 0
	if rw.Valid && strings.TrimSpace(rw.String) != "" {
		lease := &ReasonLease{Worker: rw.String, Trigger: trigger.String}
		if startedAt.Valid && startedAt.String != "" {
			if lease.StartedAt, err = parseTime(startedAt.String); err != nil {
				return Project{}, fmt.Errorf("graph: parse reason started_at: %w", err)
			}
		}
		if heartbeat.Valid && heartbeat.String != "" {
			if lease.LastHeartbeatAt, err = parseTime(heartbeat.String); err != nil {
				return Project{}, fmt.Errorf("graph: parse reason heartbeat: %w", err)
			}
		}
		p.Reason = lease
	}
	return p, nil
}

func getProjectRaw(ctx context.Context, q querier, projectID string) (Project, error) {
	p, err := scanProject(q.QueryRowContext(ctx,
		`SELECT `+projectColumns+` FROM projects WHERE id=?`, projectID))
	if errors.Is(err, sql.ErrNoRows) {
		return Project{}, fmt.Errorf("%w: project %s", ErrNotFound, projectID)
	}
	if err != nil {
		return Project{}, fmt.Errorf("graph: read project: %w", err)
	}
	return p, nil
}

const nodeColumns = `id, project_id, kind, type, content, state, from_json, to_id,
	creator, worker, lease_until, created_at, concluded_at`

func scanNode(sc interface{ Scan(dest ...any) error }) (Node, error) {
	var (
		n           Node
		fromJSON    string
		toID        sql.NullString
		worker      sql.NullString
		leaseUntil  sql.NullString
		createdAt   string
		concludedAt sql.NullString
	)
	if err := sc.Scan(&n.ID, &n.ProjectID, &n.Kind, &n.Type, &n.Content, &n.State,
		&fromJSON, &toID, &n.Creator, &worker, &leaseUntil, &createdAt, &concludedAt); err != nil {
		return Node{}, err
	}
	if fromJSON != "" && fromJSON != "[]" {
		if err := json.Unmarshal([]byte(fromJSON), &n.From); err != nil {
			return Node{}, fmt.Errorf("graph: parse node from: %w", err)
		}
	}
	if toID.Valid && toID.String != "" {
		to := toID.String
		n.To = &to
	}
	if worker.Valid {
		n.Worker = worker.String
	}
	var err error
	if n.LeaseUntil, err = nullTime(leaseUntil); err != nil {
		return Node{}, fmt.Errorf("graph: parse node lease_until: %w", err)
	}
	t, err := parseTime(createdAt)
	if err != nil {
		return Node{}, fmt.Errorf("graph: parse node created_at: %w", err)
	}
	n.CreatedAt = &t
	if n.ConcludedAt, err = nullTime(concludedAt); err != nil {
		return Node{}, fmt.Errorf("graph: parse node concluded_at: %w", err)
	}
	return n, nil
}

func getNodeRaw(ctx context.Context, q querier, projectID, nodeID string) (Node, error) {
	n, err := scanNode(q.QueryRowContext(ctx,
		`SELECT `+nodeColumns+` FROM nodes WHERE project_id=? AND id=?`, projectID, nodeID))
	if errors.Is(err, sql.ErrNoRows) {
		return Node{}, fmt.Errorf("%w: node %s", ErrNotFound, nodeID)
	}
	if err != nil {
		return Node{}, fmt.Errorf("graph: read node: %w", err)
	}
	return n, nil
}

func insertNode(ctx context.Context, q querier, n Node) error {
	fromJSON, err := json.Marshal(n.From)
	if err != nil {
		return fmt.Errorf("graph: encode node from: %w", err)
	}
	var worker any
	if strings.TrimSpace(n.Worker) != "" {
		worker = n.Worker
	}
	var toID any
	if n.To != nil {
		toID = *n.To
	}
	if _, err := q.ExecContext(ctx, `
		INSERT INTO nodes (id, project_id, kind, type, content, state, from_json, to_id,
			creator, worker, lease_until, created_at, concluded_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		n.ID, n.ProjectID, n.Kind, n.Type, n.Content, n.State, string(fromJSON), toID,
		n.Creator, worker, timeValue(n.LeaseUntil), timeValue(n.CreatedAt), timeValue(n.ConcludedAt)); err != nil {
		return fmt.Errorf("graph: insert node %s: %w", n.ID, err)
	}
	return nil
}

func insertEdge(ctx context.Context, q querier, projectID, from, to, kind string) error {
	if _, err := q.ExecContext(ctx,
		`INSERT OR IGNORE INTO edges (project_id, from_id, to_id, kind) VALUES (?, ?, ?, ?)`,
		projectID, from, to, kind); err != nil {
		return fmt.Errorf("graph: insert edge %s->%s: %w", from, to, err)
	}
	return nil
}

func nextSeq(ctx context.Context, q querier, projectID, prefix string) (string, error) {
	if _, err := q.ExecContext(ctx, `
		INSERT INTO counters (project_id, name, value) VALUES (?, ?, 1)
		ON CONFLICT(project_id, name) DO UPDATE SET value = value + 1`,
		projectID, prefix); err != nil {
		return "", fmt.Errorf("graph: advance counter %s: %w", prefix, err)
	}
	var value int
	if err := q.QueryRowContext(ctx,
		`SELECT value FROM counters WHERE project_id=? AND name=?`, projectID, prefix).Scan(&value); err != nil {
		return "", fmt.Errorf("graph: read counter %s: %w", prefix, err)
	}
	return fmt.Sprintf("%s%03d", prefix, value), nil
}

func newProjectID() (string, error) {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", fmt.Errorf("graph: generate project id: %w", err)
	}
	return "p" + hex.EncodeToString(b[:]), nil
}

func ensureActive(ctx context.Context, q querier, projectID string) (Project, error) {
	p, err := getProjectRaw(ctx, q, projectID)
	if err != nil {
		return Project{}, err
	}
	if p.Status != "active" {
		return Project{}, fmt.Errorf("%w: project %s is %s", ErrForbidden, projectID, p.Status)
	}
	return p, nil
}

// CreateProject persists a project and its reserved origin/goal facts.
func (s *Store) CreateProject(ctx context.Context, p Project) (Project, error) {
	p.Title = strings.TrimSpace(p.Title)
	p.Origin = strings.TrimSpace(p.Origin)
	p.Goal = strings.TrimSpace(p.Goal)
	if p.Title == "" || p.Origin == "" || p.Goal == "" {
		return Project{}, fmt.Errorf("%w: title, origin and goal are required", ErrInvalid)
	}
	var err error
	if strings.TrimSpace(p.ID) == "" {
		if p.ID, err = newProjectID(); err != nil {
			return Project{}, err
		}
	}
	p.ID = strings.TrimSpace(p.ID)
	if p.Status == "" {
		p.Status = "active"
	}
	if p.Status != "active" {
		return Project{}, fmt.Errorf("%w: a new project must be active", ErrInvalid)
	}
	if p.CreatedAt.IsZero() {
		p.CreatedAt = time.Now()
	}
	p.Reason = nil

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Project{}, fmt.Errorf("graph: begin create project: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO projects (id, title, origin, goal, status, bootstrap_enabled, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		p.ID, p.Title, p.Origin, p.Goal, p.Status, boolInt(p.BootstrapEnabled), formatTime(p.CreatedAt)); err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "unique") {
			return Project{}, fmt.Errorf("%w: project %s already exists", ErrConflict, p.ID)
		}
		return Project{}, fmt.Errorf("graph: create project: %w", err)
	}

	now := p.CreatedAt
	for _, fact := range []struct {
		id, typ, content string
	}{
		{"origin", "origin", p.Origin},
		{"goal", "goal", p.Goal},
	} {
		created := now
		if err := insertNode(ctx, tx, Node{
			ID:        fact.id,
			ProjectID: p.ID,
			Kind:      "fact",
			Type:      fact.typ,
			Content:   fact.content,
			State:     "resolved",
			CreatedAt: &created,
		}); err != nil {
			return Project{}, err
		}
	}
	if err := tx.Commit(); err != nil {
		return Project{}, fmt.Errorf("graph: commit create project: %w", err)
	}
	return p, nil
}

func boolInt(v bool) int {
	if v {
		return 1
	}
	return 0
}

// GetProject returns a project, reclaiming expired leases first.
func (s *Store) GetProject(ctx context.Context, projectID string) (Project, error) {
	if err := s.cleanupLeases(ctx, projectID); err != nil {
		return Project{}, err
	}
	return getProjectRaw(ctx, s.db, projectID)
}

// ListProjects returns all projects, reclaiming expired leases first.
func (s *Store) ListProjects(ctx context.Context) ([]Project, error) {
	if err := s.cleanupLeases(ctx, ""); err != nil {
		return nil, err
	}
	rows, err := s.db.QueryContext(ctx,
		`SELECT `+projectColumns+` FROM projects ORDER BY created_at, id`)
	if err != nil {
		return nil, fmt.Errorf("graph: list projects: %w", err)
	}
	defer rows.Close()
	var projects []Project
	for rows.Next() {
		p, err := scanProject(rows)
		if err != nil {
			return nil, err
		}
		projects = append(projects, p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("graph: list projects: %w", err)
	}
	return projects, nil
}

// SetProjectStatus switches a project between active and stopped. Completed
// projects can only be reopened through Reopen.
func (s *Store) SetProjectStatus(ctx context.Context, projectID, status string) (Project, error) {
	status = strings.TrimSpace(status)
	if status != "active" && status != "stopped" {
		return Project{}, fmt.Errorf("%w: status must be active or stopped", ErrInvalid)
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Project{}, fmt.Errorf("graph: begin set status: %w", err)
	}
	defer tx.Rollback()

	p, err := getProjectRaw(ctx, tx, projectID)
	if err != nil {
		return Project{}, err
	}
	if p.Status == "completed" {
		return Project{}, fmt.Errorf("%w: completed projects must be reopened", ErrInvalid)
	}
	if status == "stopped" {
		if _, err := tx.ExecContext(ctx, `
			UPDATE nodes SET worker=NULL, state='open', lease_until=NULL
			WHERE project_id=? AND kind='intent'
			  AND (state='claimed' OR (state='open' AND worker IS NOT NULL AND worker <> ''))`,
			projectID); err != nil {
			return Project{}, fmt.Errorf("graph: clear intent workers: %w", err)
		}
		if _, err := tx.ExecContext(ctx, `
			UPDATE projects SET reason_worker=NULL, reason_trigger=NULL,
				reason_started_at=NULL, reason_heartbeat_at=NULL, reason_expire_at=NULL
			WHERE id=?`, projectID); err != nil {
			return Project{}, fmt.Errorf("graph: clear reason lease: %w", err)
		}
	}
	if _, err := tx.ExecContext(ctx, `UPDATE projects SET status=? WHERE id=?`, status, projectID); err != nil {
		return Project{}, fmt.Errorf("graph: set project status: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return Project{}, fmt.Errorf("graph: commit set status: %w", err)
	}
	p.Status = status
	p.Reason = nil
	return p, nil
}

// AddHint records an out-of-band hint. Hints are allowed in any project state.
func (s *Store) AddHint(ctx context.Context, projectID, content, creator string) (Node, error) {
	content = strings.TrimSpace(content)
	creator = strings.TrimSpace(creator)
	if content == "" || creator == "" {
		return Node{}, fmt.Errorf("%w: hint content and creator are required", ErrInvalid)
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Node{}, fmt.Errorf("graph: begin add hint: %w", err)
	}
	defer tx.Rollback()

	if _, err := getProjectRaw(ctx, tx, projectID); err != nil {
		return Node{}, err
	}
	id, err := nextSeq(ctx, tx, projectID, "h")
	if err != nil {
		return Node{}, err
	}
	now := time.Now()
	hint := Node{
		ID:        id,
		ProjectID: projectID,
		Kind:      "hint",
		Type:      "hint",
		Content:   content,
		Creator:   creator,
		CreatedAt: &now,
	}
	if err := insertNode(ctx, tx, hint); err != nil {
		return Node{}, err
	}
	if err := tx.Commit(); err != nil {
		return Node{}, fmt.Errorf("graph: commit add hint: %w", err)
	}
	return hint, nil
}

// DeclareIntent creates an open intent, optionally claimed by its creator.
func (s *Store) DeclareIntent(ctx context.Context, projectID string, n Node) (Node, error) {
	content := strings.TrimSpace(n.Content)
	creator := strings.TrimSpace(n.Creator)
	worker := strings.TrimSpace(n.Worker)
	typ := strings.TrimSpace(n.Type)
	if typ == "" {
		typ = "gap"
	}
	if content == "" || creator == "" {
		return Node{}, fmt.Errorf("%w: intent content and creator are required", ErrInvalid)
	}
	if worker != "" && worker != creator {
		return Node{}, fmt.Errorf("%w: a newly claimed intent must be claimed by its creator", ErrInvalid)
	}
	if len(n.From) == 0 {
		return Node{}, fmt.Errorf("%w: intent requires at least one from fact", ErrInvalid)
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Node{}, fmt.Errorf("graph: begin declare intent: %w", err)
	}
	defer tx.Rollback()

	if _, err := ensureActive(ctx, tx, projectID); err != nil {
		return Node{}, err
	}
	from := make([]string, 0, len(n.From))
	for _, raw := range n.From {
		fromID := strings.TrimSpace(raw)
		if fromID == "" || fromID == "goal" {
			return Node{}, fmt.Errorf("%w: invalid from fact %q", ErrInvalid, raw)
		}
		fact, err := getNodeRaw(ctx, tx, projectID, fromID)
		if err != nil {
			if errors.Is(err, ErrNotFound) {
				return Node{}, fmt.Errorf("%w: unknown from fact %s", ErrInvalid, fromID)
			}
			return Node{}, err
		}
		if fact.Kind != "fact" {
			return Node{}, fmt.Errorf("%w: from node %s is not a fact", ErrInvalid, fromID)
		}
		from = append(from, fromID)
	}

	id, err := nextSeq(ctx, tx, projectID, "i")
	if err != nil {
		return Node{}, err
	}
	now := time.Now()
	intent := Node{
		ID:        id,
		ProjectID: projectID,
		Kind:      "intent",
		Type:      typ,
		Content:   content,
		State:     "open",
		From:      from,
		Creator:   creator,
		CreatedAt: &now,
	}
	if worker != "" {
		leaseUntil := now.Add(defaultIntentLease)
		intent.Worker = worker
		intent.State = "claimed"
		intent.LeaseUntil = &leaseUntil
	}
	if err := insertNode(ctx, tx, intent); err != nil {
		return Node{}, err
	}
	if err := tx.Commit(); err != nil {
		return Node{}, fmt.Errorf("graph: commit declare intent: %w", err)
	}
	return intent, nil
}

// ClaimIntent atomically claims an open intent using a single conditional
// UPDATE. Exactly one concurrent caller can succeed.
func (s *Store) ClaimIntent(ctx context.Context, projectID, intentID, worker string, lease time.Duration) (Node, error) {
	projectID = strings.TrimSpace(projectID)
	intentID = strings.TrimSpace(intentID)
	worker = strings.TrimSpace(worker)
	if projectID == "" || intentID == "" || worker == "" {
		return Node{}, fmt.Errorf("%w: project, intent and worker are required", ErrInvalid)
	}
	if err := s.cleanupLeases(ctx, projectID); err != nil {
		return Node{}, err
	}
	if _, err := ensureActive(ctx, s.db, projectID); err != nil {
		return Node{}, err
	}

	now := time.Now()
	result, err := s.db.ExecContext(ctx, `
		UPDATE nodes SET worker=?, state='claimed', lease_until=?
		WHERE id=? AND project_id=? AND kind='intent' AND state='open'
		  AND (worker IS NULL OR worker='' OR lease_until < ?)`,
		worker, formatTime(now.Add(lease)), intentID, projectID, formatTime(now))
	if err != nil {
		return Node{}, fmt.Errorf("graph: claim intent: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return Node{}, fmt.Errorf("graph: claim intent rows: %w", err)
	}
	if affected == 0 {
		if _, err := getNodeRaw(ctx, s.db, projectID, intentID); err != nil {
			return Node{}, err
		}
		return Node{}, fmt.Errorf("%w: intent %s is held by another worker", ErrConflict, intentID)
	}
	return getNodeRaw(ctx, s.db, projectID, intentID)
}

// HeartbeatIntent renews an active claim, or claims the intent when it is
// open. An expired or foreign claim yields ErrConflict.
func (s *Store) HeartbeatIntent(ctx context.Context, projectID, intentID, worker string, lease time.Duration) (Node, error) {
	projectID = strings.TrimSpace(projectID)
	intentID = strings.TrimSpace(intentID)
	worker = strings.TrimSpace(worker)
	if projectID == "" || intentID == "" || worker == "" {
		return Node{}, fmt.Errorf("%w: project, intent and worker are required", ErrInvalid)
	}
	if err := s.cleanupLeases(ctx, projectID); err != nil {
		return Node{}, err
	}
	if _, err := ensureActive(ctx, s.db, projectID); err != nil {
		return Node{}, err
	}

	now := time.Now()
	until := formatTime(now.Add(lease))
	result, err := s.db.ExecContext(ctx, `
		UPDATE nodes SET lease_until=?
		WHERE id=? AND project_id=? AND kind='intent' AND state='claimed' AND worker=?`,
		until, intentID, projectID, worker)
	if err != nil {
		return Node{}, fmt.Errorf("graph: heartbeat intent: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return Node{}, fmt.Errorf("graph: heartbeat intent rows: %w", err)
	}
	if affected == 0 {
		result, err = s.db.ExecContext(ctx, `
			UPDATE nodes SET worker=?, state='claimed', lease_until=?
			WHERE id=? AND project_id=? AND kind='intent' AND state='open'
			  AND (worker IS NULL OR worker='' OR lease_until < ?)`,
			worker, until, intentID, projectID, formatTime(now))
		if err != nil {
			return Node{}, fmt.Errorf("graph: claim intent on heartbeat: %w", err)
		}
		if affected, err = result.RowsAffected(); err != nil {
			return Node{}, fmt.Errorf("graph: claim intent on heartbeat rows: %w", err)
		}
	}
	if affected == 0 {
		if _, err := getNodeRaw(ctx, s.db, projectID, intentID); err != nil {
			return Node{}, err
		}
		return Node{}, fmt.Errorf("%w: intent %s is not held by %s", ErrConflict, intentID, worker)
	}
	return getNodeRaw(ctx, s.db, projectID, intentID)
}

// ReleaseIntent returns an un-concluded intent to the open pool.
func (s *Store) ReleaseIntent(ctx context.Context, projectID, intentID, worker string) (Node, error) {
	projectID = strings.TrimSpace(projectID)
	intentID = strings.TrimSpace(intentID)
	worker = strings.TrimSpace(worker)
	if projectID == "" || intentID == "" {
		return Node{}, fmt.Errorf("%w: project and intent are required", ErrInvalid)
	}
	if err := s.cleanupLeases(ctx, projectID); err != nil {
		return Node{}, err
	}
	if _, err := ensureActive(ctx, s.db, projectID); err != nil {
		return Node{}, err
	}

	if worker == "" {
		n, err := getNodeRaw(ctx, s.db, projectID, intentID)
		if err != nil {
			return Node{}, err
		}
		if n.State == "open" && n.Worker == "" {
			return n, nil
		}
		return Node{}, fmt.Errorf("%w: intent %s is held", ErrConflict, intentID)
	}

	result, err := s.db.ExecContext(ctx, `
		UPDATE nodes SET worker=NULL, state='open', lease_until=NULL
		WHERE id=? AND project_id=? AND kind='intent' AND state='claimed' AND worker=?`,
		intentID, projectID, worker)
	if err != nil {
		return Node{}, fmt.Errorf("graph: release intent: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return Node{}, fmt.Errorf("graph: release intent rows: %w", err)
	}
	if affected == 0 {
		n, err := getNodeRaw(ctx, s.db, projectID, intentID)
		if err != nil {
			return Node{}, err
		}
		if n.To != nil {
			return Node{}, fmt.Errorf("%w: intent %s is already concluded", ErrConflict, intentID)
		}
		return Node{}, fmt.Errorf("%w: intent %s is not held by %s", ErrConflict, intentID, worker)
	}
	return getNodeRaw(ctx, s.db, projectID, intentID)
}

// ConcludeIntent atomically creates a conclusion fact and closes the intent.
// It returns the new fact and the concluded intent.
func (s *Store) ConcludeIntent(ctx context.Context, projectID, intentID, worker, description, factType string) (Node, Node, error) {
	projectID = strings.TrimSpace(projectID)
	intentID = strings.TrimSpace(intentID)
	worker = strings.TrimSpace(worker)
	description = strings.TrimSpace(description)
	factType = strings.TrimSpace(factType)
	if factType == "" {
		factType = "observation"
	}
	if projectID == "" || intentID == "" || worker == "" || description == "" {
		return Node{}, Node{}, fmt.Errorf("%w: project, intent, worker and description are required", ErrInvalid)
	}
	if err := s.cleanupLeases(ctx, projectID); err != nil {
		return Node{}, Node{}, err
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Node{}, Node{}, fmt.Errorf("graph: begin conclude: %w", err)
	}
	defer tx.Rollback()

	if _, err := ensureActive(ctx, tx, projectID); err != nil {
		return Node{}, Node{}, err
	}
	intent, err := getNodeRaw(ctx, tx, projectID, intentID)
	if err != nil {
		return Node{}, Node{}, err
	}
	if intent.Kind != "intent" {
		return Node{}, Node{}, fmt.Errorf("%w: node %s is not an intent", ErrInvalid, intentID)
	}
	if intent.To != nil {
		return Node{}, Node{}, fmt.Errorf("%w: intent %s is already concluded", ErrConflict, intentID)
	}
	if intent.Worker != "" && intent.Worker != worker {
		return Node{}, Node{}, fmt.Errorf("%w: intent %s is held by %s", ErrConflict, intentID, intent.Worker)
	}

	factID, err := nextSeq(ctx, tx, projectID, "f")
	if err != nil {
		return Node{}, Node{}, err
	}
	now := time.Now()
	fact := Node{
		ID:        factID,
		ProjectID: projectID,
		Kind:      "fact",
		Type:      factType,
		Content:   description,
		State:     "resolved",
		CreatedAt: &now,
	}
	if err := insertNode(ctx, tx, fact); err != nil {
		return Node{}, Node{}, err
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE nodes SET to_id=?, state='resolved', worker=?, concluded_at=?, lease_until=NULL
		WHERE project_id=? AND id=? AND kind='intent' AND to_id IS NULL`,
		factID, worker, formatTime(now), projectID, intentID); err != nil {
		return Node{}, Node{}, fmt.Errorf("graph: conclude intent: %w", err)
	}
	for _, from := range intent.From {
		if err := insertEdge(ctx, tx, projectID, from, factID, "produce"); err != nil {
			return Node{}, Node{}, err
		}
	}
	if err := tx.Commit(); err != nil {
		return Node{}, Node{}, fmt.Errorf("graph: commit conclude: %w", err)
	}
	intent.To = &factID
	intent.State = "resolved"
	intent.Worker = worker
	intent.ConcludedAt = &now
	intent.LeaseUntil = nil
	return fact, intent, nil
}

// ClaimReason atomically claims the single project-level reason lease.
func (s *Store) ClaimReason(ctx context.Context, projectID, worker, trigger string, lease time.Duration) (Project, error) {
	projectID = strings.TrimSpace(projectID)
	worker = strings.TrimSpace(worker)
	trigger = strings.TrimSpace(trigger)
	if projectID == "" || worker == "" || trigger == "" {
		return Project{}, fmt.Errorf("%w: project, worker and trigger are required", ErrInvalid)
	}
	if err := s.cleanupLeases(ctx, projectID); err != nil {
		return Project{}, err
	}

	now := time.Now()
	nowStr := formatTime(now)
	result, err := s.db.ExecContext(ctx, `
		UPDATE projects SET reason_worker=?, reason_trigger=?, reason_started_at=?,
			reason_heartbeat_at=?, reason_expire_at=?
		WHERE id=? AND status='active'
		  AND (reason_worker IS NULL OR reason_worker='' OR reason_worker=? OR reason_expire_at < ?)`,
		worker, trigger, nowStr, nowStr, formatTime(now.Add(lease)),
		projectID, worker, nowStr)
	if err != nil {
		return Project{}, fmt.Errorf("graph: claim reason: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return Project{}, fmt.Errorf("graph: claim reason rows: %w", err)
	}
	if affected == 0 {
		p, err := getProjectRaw(ctx, s.db, projectID)
		if err != nil {
			return Project{}, err
		}
		if p.Status != "active" {
			return Project{}, fmt.Errorf("%w: project %s is %s", ErrForbidden, projectID, p.Status)
		}
		return Project{}, fmt.Errorf("%w: reason is held by another worker", ErrConflict)
	}
	return getProjectRaw(ctx, s.db, projectID)
}

// HeartbeatReason renews the project reason lease for its current holder.
func (s *Store) HeartbeatReason(ctx context.Context, projectID, worker string, lease time.Duration) (Project, error) {
	projectID = strings.TrimSpace(projectID)
	worker = strings.TrimSpace(worker)
	if projectID == "" || worker == "" {
		return Project{}, fmt.Errorf("%w: project and worker are required", ErrInvalid)
	}
	if err := s.cleanupLeases(ctx, projectID); err != nil {
		return Project{}, err
	}

	now := time.Now()
	nowStr := formatTime(now)
	result, err := s.db.ExecContext(ctx, `
		UPDATE projects SET reason_heartbeat_at=?, reason_expire_at=?
		WHERE id=? AND status='active' AND reason_worker=? AND reason_expire_at >= ?`,
		nowStr, formatTime(now.Add(lease)), projectID, worker, nowStr)
	if err != nil {
		return Project{}, fmt.Errorf("graph: heartbeat reason: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return Project{}, fmt.Errorf("graph: heartbeat reason rows: %w", err)
	}
	if affected == 0 {
		p, err := getProjectRaw(ctx, s.db, projectID)
		if err != nil {
			return Project{}, err
		}
		if p.Status != "active" {
			return Project{}, fmt.Errorf("%w: project %s is %s", ErrForbidden, projectID, p.Status)
		}
		return Project{}, fmt.Errorf("%w: reason is not held by %s", ErrConflict, worker)
	}
	return getProjectRaw(ctx, s.db, projectID)
}

// ReleaseReason drops the reason lease. Releasing an already-free lease is
// idempotent; releasing someone else's lease is a conflict.
func (s *Store) ReleaseReason(ctx context.Context, projectID, worker string) (Project, error) {
	projectID = strings.TrimSpace(projectID)
	worker = strings.TrimSpace(worker)
	if projectID == "" || worker == "" {
		return Project{}, fmt.Errorf("%w: project and worker are required", ErrInvalid)
	}
	if err := s.cleanupLeases(ctx, projectID); err != nil {
		return Project{}, err
	}
	p, err := getProjectRaw(ctx, s.db, projectID)
	if err != nil {
		return Project{}, err
	}
	if p.Reason == nil {
		return p, nil
	}
	if worker != p.Reason.Worker {
		return Project{}, fmt.Errorf("%w: reason is held by %s", ErrConflict, p.Reason.Worker)
	}
	if _, err := s.db.ExecContext(ctx, `
		UPDATE projects SET reason_worker=NULL, reason_trigger=NULL,
			reason_started_at=NULL, reason_heartbeat_at=NULL, reason_expire_at=NULL
		WHERE id=?`, projectID); err != nil {
		return Project{}, fmt.Errorf("graph: release reason: %w", err)
	}
	p.Reason = nil
	return p, nil
}

// Complete atomically records a from[] -> goal completion intent and marks the
// project completed.
func (s *Store) Complete(ctx context.Context, projectID string, from []string, description, worker string) (Project, error) {
	projectID = strings.TrimSpace(projectID)
	description = strings.TrimSpace(description)
	worker = strings.TrimSpace(worker)
	if projectID == "" || description == "" || worker == "" {
		return Project{}, fmt.Errorf("%w: project, description and worker are required", ErrInvalid)
	}
	if len(from) == 0 {
		return Project{}, fmt.Errorf("%w: complete requires at least one from fact", ErrInvalid)
	}
	if err := s.cleanupLeases(ctx, projectID); err != nil {
		return Project{}, err
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Project{}, fmt.Errorf("graph: begin complete: %w", err)
	}
	defer tx.Rollback()

	p, err := ensureActive(ctx, tx, projectID)
	if err != nil {
		return Project{}, err
	}
	fromIDs := make([]string, 0, len(from))
	for _, raw := range from {
		fromID := strings.TrimSpace(raw)
		if fromID == "" || fromID == "goal" {
			return Project{}, fmt.Errorf("%w: invalid from fact %q", ErrInvalid, raw)
		}
		fact, err := getNodeRaw(ctx, tx, projectID, fromID)
		if err != nil {
			if errors.Is(err, ErrNotFound) {
				return Project{}, fmt.Errorf("%w: unknown from fact %s", ErrInvalid, fromID)
			}
			return Project{}, err
		}
		if fact.Kind != "fact" {
			return Project{}, fmt.Errorf("%w: from node %s is not a fact", ErrInvalid, fromID)
		}
		fromIDs = append(fromIDs, fromID)
	}

	intentID, err := nextSeq(ctx, tx, projectID, "i")
	if err != nil {
		return Project{}, err
	}
	now := time.Now()
	goal := "goal"
	completion := Node{
		ID:          intentID,
		ProjectID:   projectID,
		Kind:        "intent",
		Type:        "human_intent",
		Content:     description,
		State:       "resolved",
		From:        fromIDs,
		To:          &goal,
		Creator:     worker,
		Worker:      worker,
		CreatedAt:   &now,
		ConcludedAt: &now,
	}
	if err := insertNode(ctx, tx, completion); err != nil {
		return Project{}, err
	}
	for _, fromID := range fromIDs {
		if err := insertEdge(ctx, tx, projectID, fromID, goal, "produce"); err != nil {
			return Project{}, err
		}
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE projects SET status='completed', reason_worker=NULL, reason_trigger=NULL,
			reason_started_at=NULL, reason_heartbeat_at=NULL, reason_expire_at=NULL
		WHERE id=?`, projectID); err != nil {
		return Project{}, fmt.Errorf("graph: complete project: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return Project{}, fmt.Errorf("graph: commit complete: %w", err)
	}
	p.Status = "completed"
	p.Reason = nil
	return p, nil
}

// Reopen undoes a completion: it removes the completion edge, writes a
// correction fact and records an external_feedback intent.
func (s *Store) Reopen(ctx context.Context, projectID, creator, description string) (Project, error) {
	projectID = strings.TrimSpace(projectID)
	creator = strings.TrimSpace(creator)
	description = strings.TrimSpace(description)
	if projectID == "" || creator == "" || description == "" {
		return Project{}, fmt.Errorf("%w: project, creator and description are required", ErrInvalid)
	}
	if err := s.cleanupLeases(ctx, projectID); err != nil {
		return Project{}, err
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Project{}, fmt.Errorf("graph: begin reopen: %w", err)
	}
	defer tx.Rollback()

	p, err := getProjectRaw(ctx, tx, projectID)
	if err != nil {
		return Project{}, err
	}
	if p.Status != "completed" {
		return Project{}, fmt.Errorf("%w: only completed projects can be reopened", ErrInvalid)
	}

	completion, err := scanNode(tx.QueryRowContext(ctx,
		`SELECT `+nodeColumns+` FROM nodes WHERE project_id=? AND kind='intent' AND to_id='goal'
		 ORDER BY created_at DESC, id DESC LIMIT 1`, projectID))
	if errors.Is(err, sql.ErrNoRows) {
		return Project{}, fmt.Errorf("%w: completion intent", ErrNotFound)
	}
	if err != nil {
		return Project{}, fmt.Errorf("graph: read completion intent: %w", err)
	}

	if _, err := tx.ExecContext(ctx, `
		DELETE FROM edges WHERE project_id=? AND (from_id=? OR to_id=? OR to_id='goal')`,
		projectID, completion.ID, completion.ID); err != nil {
		return Project{}, fmt.Errorf("graph: delete completion edges: %w", err)
	}
	if _, err := tx.ExecContext(ctx,
		`DELETE FROM nodes WHERE project_id=? AND id=?`, projectID, completion.ID); err != nil {
		return Project{}, fmt.Errorf("graph: delete completion intent: %w", err)
	}

	factID, err := nextSeq(ctx, tx, projectID, "f")
	if err != nil {
		return Project{}, err
	}
	now := time.Now()
	correction := Node{
		ID:        factID,
		ProjectID: projectID,
		Kind:      "fact",
		Type:      "correction",
		Content:   description,
		State:     "resolved",
		CreatedAt: &now,
	}
	if err := insertNode(ctx, tx, correction); err != nil {
		return Project{}, err
	}

	intentID, err := nextSeq(ctx, tx, projectID, "i")
	if err != nil {
		return Project{}, err
	}
	feedback := Node{
		ID:          intentID,
		ProjectID:   projectID,
		Kind:        "intent",
		Type:        "human_intent",
		Content:     "external_feedback",
		State:       "resolved",
		From:        completion.From,
		To:          &factID,
		Creator:     creator,
		Worker:      creator,
		CreatedAt:   &now,
		ConcludedAt: &now,
	}
	if err := insertNode(ctx, tx, feedback); err != nil {
		return Project{}, err
	}
	for _, fromID := range completion.From {
		if err := insertEdge(ctx, tx, projectID, fromID, factID, "produce"); err != nil {
			return Project{}, err
		}
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE projects SET status='active', reason_worker=NULL, reason_trigger=NULL,
			reason_started_at=NULL, reason_heartbeat_at=NULL, reason_expire_at=NULL
		WHERE id=?`, projectID); err != nil {
		return Project{}, fmt.Errorf("graph: reopen project: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return Project{}, fmt.Errorf("graph: commit reopen: %w", err)
	}
	p.Status = "active"
	p.Reason = nil
	return p, nil
}

// AddEdge inserts a typed edge. Block edges are rejected when they would
// create a cycle (including self-loops).
func (s *Store) AddEdge(ctx context.Context, projectID, from, to, kind string) error {
	projectID = strings.TrimSpace(projectID)
	from = strings.TrimSpace(from)
	to = strings.TrimSpace(to)
	kind = strings.TrimSpace(kind)
	if projectID == "" || from == "" || to == "" {
		return fmt.Errorf("%w: project, from and to are required", ErrInvalid)
	}
	if !validEdgeKinds[kind] {
		return fmt.Errorf("%w: unknown edge kind %q", ErrInvalid, kind)
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("graph: begin add edge: %w", err)
	}
	defer tx.Rollback()

	if _, err := ensureActive(ctx, tx, projectID); err != nil {
		return err
	}
	for _, nodeID := range []string{from, to} {
		if _, err := getNodeRaw(ctx, tx, projectID, nodeID); err != nil {
			if errors.Is(err, ErrNotFound) {
				return fmt.Errorf("%w: unknown edge endpoint %s", ErrInvalid, nodeID)
			}
			return err
		}
	}
	if kind == "block" {
		cycle, err := wouldCycle(ctx, tx, projectID, from, to)
		if err != nil {
			return err
		}
		if cycle {
			return fmt.Errorf("%w: block edge %s -> %s", ErrCycle, from, to)
		}
	}
	if err := insertEdge(ctx, tx, projectID, from, to, kind); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("graph: commit add edge: %w", err)
	}
	return nil
}

// wouldCycle reports whether adding from -> to closes a cycle in the block
// edge subgraph, i.e. whether `to` can already reach `from`.
func wouldCycle(ctx context.Context, q querier, projectID, from, to string) (bool, error) {
	rows, err := q.QueryContext(ctx,
		`SELECT from_id, to_id FROM edges WHERE project_id=? AND kind='block'`, projectID)
	if err != nil {
		return false, fmt.Errorf("graph: read block edges: %w", err)
	}
	defer rows.Close()

	adj := make(map[string][]string)
	for rows.Next() {
		var f, t string
		if err := rows.Scan(&f, &t); err != nil {
			return false, fmt.Errorf("graph: scan block edge: %w", err)
		}
		adj[f] = append(adj[f], t)
	}
	if err := rows.Err(); err != nil {
		return false, fmt.Errorf("graph: read block edges: %w", err)
	}
	if from == to {
		return true, nil
	}

	// Iterative DFS from `to`, looking for `from`.
	stack := []string{to}
	seen := map[string]bool{to: true}
	for len(stack) > 0 {
		cur := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		for _, next := range adj[cur] {
			if next == from {
				return true, nil
			}
			if !seen[next] {
				seen[next] = true
				stack = append(stack, next)
			}
		}
	}
	return false, nil
}

// GetNode returns one node, reclaiming expired leases first.
func (s *Store) GetNode(ctx context.Context, projectID, nodeID string) (Node, error) {
	if err := s.cleanupLeases(ctx, projectID); err != nil {
		return Node{}, err
	}
	return getNodeRaw(ctx, s.db, projectID, nodeID)
}

// Nodes returns every node of a project in creation order.
func (s *Store) Nodes(ctx context.Context, projectID string) ([]Node, error) {
	if err := s.cleanupLeases(ctx, projectID); err != nil {
		return nil, err
	}
	rows, err := s.db.QueryContext(ctx,
		`SELECT `+nodeColumns+` FROM nodes WHERE project_id=? ORDER BY created_at, id`, projectID)
	if err != nil {
		return nil, fmt.Errorf("graph: list nodes: %w", err)
	}
	defer rows.Close()
	var nodes []Node
	for rows.Next() {
		n, err := scanNode(rows)
		if err != nil {
			return nil, err
		}
		nodes = append(nodes, n)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("graph: list nodes: %w", err)
	}
	return nodes, nil
}

// Edges returns every edge of a project.
func (s *Store) Edges(ctx context.Context, projectID string) ([]Edge, error) {
	if err := s.cleanupLeases(ctx, projectID); err != nil {
		return nil, err
	}
	rows, err := s.db.QueryContext(ctx,
		`SELECT project_id, from_id, to_id, kind FROM edges WHERE project_id=?
		 ORDER BY from_id, to_id, kind`, projectID)
	if err != nil {
		return nil, fmt.Errorf("graph: list edges: %w", err)
	}
	defer rows.Close()
	var edges []Edge
	for rows.Next() {
		var e Edge
		if err := rows.Scan(&e.ProjectID, &e.From, &e.To, &e.Kind); err != nil {
			return nil, fmt.Errorf("graph: scan edge: %w", err)
		}
		edges = append(edges, e)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("graph: list edges: %w", err)
	}
	return edges, nil
}
