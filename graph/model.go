// Package graph implements PatchTrace's Cairn-style provenance graph.
//
// Facts are immutable nodes, intents are explorable hyper-edges over facts,
// and hints are out-of-band observations. All state is persisted in SQLite so
// that the graph is the single source of truth for later reasoning tasks.
package graph

import (
	"database/sql"
	"errors"
	"time"
)

// Sentinel errors returned by Store. Callers use errors.Is to map them onto
// protocol status codes (404/409/403/422/409).
var (
	ErrNotFound  = errors.New("graph: not found")
	ErrConflict  = errors.New("graph: conflict")
	ErrForbidden = errors.New("graph: forbidden")
	ErrInvalid   = errors.New("graph: invalid")
	ErrCycle     = errors.New("graph: cycle")
)

// ReasonLease is the project-level reason worker lease.
type ReasonLease struct {
	Worker          string
	Trigger         string
	StartedAt       time.Time
	LastHeartbeatAt time.Time
}

// Project is one graph instance with a fixed origin and goal fact.
type Project struct {
	ID               string
	Title            string
	Origin           string
	Goal             string
	Status           string
	BootstrapEnabled bool
	CreatedAt        time.Time
	Reason           *ReasonLease
}

// Node is a fact, intent or hint in the provenance graph.
type Node struct {
	ID          string
	ProjectID   string
	Kind        string
	Type        string
	Content     string
	State       string
	From        []string
	To          *string
	Creator     string
	Worker      string
	LeaseUntil  *time.Time
	CreatedAt   *time.Time
	ConcludedAt *time.Time
}

// Edge is a typed semantic relation between two nodes.
type Edge struct {
	ProjectID string
	From      string
	To        string
	Kind      string
}

// Store is a SQLite-backed provenance graph.
type Store struct {
	db *sql.DB
}
