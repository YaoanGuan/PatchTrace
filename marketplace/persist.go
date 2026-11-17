package marketplace

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	_ "modernc.org/sqlite"
)

// stateSchemaVersion is the on-disk snapshot format version. Bump it whenever
// the persistedState shape changes; OpenStore refuses to load unknown versions
// instead of silently discarding data.
const stateSchemaVersion = 1

// persistedState is the full marketplace snapshot stored as a single row. The
// live Store keeps internal task metadata in unexported fields, so the snapshot
// spells them out explicitly.
type persistedState struct {
	Me           User                         `json:"me"`
	Users        map[string]UserSummary       `json:"users"`
	Tasks        []persistedTaskRecord        `json:"tasks"`
	Traces       map[string]GetTraceResponse  `json:"traces"`
	Wallet       WalletOverview               `json:"wallet"`
	Reputations  map[string]ReputationSummary `json:"reputations"`
	Expert       ExpertState                  `json:"expert"`
	Gateway      GatewayState                 `json:"gateway"`
	NextTaskID   int                          `json:"nextTaskId"`
	NextRatingID int                          `json:"nextRatingId"`
	NextTxID     int                          `json:"nextTxId"`
	NextApiKeyID int                          `json:"nextApiKeyId"`
}

type persistedTaskRecord struct {
	Detail          TaskDetail `json:"detail"`
	AcceptCount     int        `json:"acceptCount"`
	ValidationScore int        `json:"validationScore"`
}

// OpenStore opens (or creates) the SQLite-backed marketplace store at path.
// On first use it writes the deterministic seed state; on later opens it loads
// the previously persisted snapshot. A version mismatch or a corrupt payload is
// reported as an error and the existing database is left untouched.
func OpenStore(path string) (*Store, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open marketplace db %s: %w", path, err)
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)

	if _, err := db.Exec(`PRAGMA journal_mode=WAL; PRAGMA busy_timeout=5000;`); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("configure marketplace db %s: %w", path, err)
	}
	if _, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS marketplace_state (
			id         INTEGER PRIMARY KEY CHECK(id=1),
			version    INTEGER NOT NULL,
			payload    BLOB NOT NULL,
			updated_at TEXT NOT NULL
		);
	`); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("create marketplace state table: %w", err)
	}

	var (
		version int
		payload []byte
	)
	err = db.QueryRow(`SELECT version, payload FROM marketplace_state WHERE id = 1`).Scan(&version, &payload)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		store := NewStore()
		store.db = db

		store.mu.Lock()
		persistErr := store.persistLocked()
		store.mu.Unlock()
		if persistErr != nil {
			_ = db.Close()
			return nil, fmt.Errorf("seed marketplace state: %w", persistErr)
		}
		return store, nil
	case err != nil:
		_ = db.Close()
		return nil, fmt.Errorf("read marketplace state: %w", err)
	}

	if version != stateSchemaVersion {
		_ = db.Close()
		return nil, fmt.Errorf("unsupported marketplace state version %d (want %d)", version, stateSchemaVersion)
	}

	var state persistedState
	if err := json.Unmarshal(payload, &state); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("decode marketplace state: %w", err)
	}
	store, err := storeFromPersisted(state)
	if err != nil {
		_ = db.Close()
		return nil, err
	}
	store.db = db

	// Older snapshots predate the trust layer, API gateway and VIP fields.
	// Fill only the missing sections (never touch tasks, balances or ledgers)
	// and write the upgraded snapshot back without changing the version.
	store.mu.Lock()
	store.ensureSeededLocked()
	persistErr := store.persistLocked()
	store.mu.Unlock()
	if persistErr != nil {
		_ = db.Close()
		return nil, fmt.Errorf("upgrade marketplace state: %w", persistErr)
	}
	return store, nil
}

func storeFromPersisted(state persistedState) (*Store, error) {
	if state.Me.ID == "" {
		return nil, fmt.Errorf("decode marketplace state: missing current user")
	}
	if state.Users == nil || state.Tasks == nil || state.Traces == nil || state.Reputations == nil {
		return nil, fmt.Errorf("decode marketplace state: incomplete collections")
	}
	if state.NextTaskID <= 0 || state.NextRatingID <= 0 || state.NextTxID <= 0 {
		return nil, fmt.Errorf("decode marketplace state: invalid next-id counters")
	}

	tasks := make([]*taskRecord, len(state.Tasks))
	for index, item := range state.Tasks {
		if item.Detail.ID == "" {
			return nil, fmt.Errorf("decode marketplace state: task at index %d has no id", index)
		}
		tasks[index] = &taskRecord{
			detail:          item.Detail,
			acceptCount:     item.AcceptCount,
			validationScore: item.ValidationScore,
		}
	}

	return &Store{
		me:           state.Me,
		users:        state.Users,
		tasks:        tasks,
		traces:       state.Traces,
		wallet:       state.Wallet,
		reputations:  state.Reputations,
		expert:       state.Expert,
		gateway:      state.Gateway,
		nextTaskID:   state.NextTaskID,
		nextRatingID: state.NextRatingID,
		nextTxID:     state.NextTxID,
		nextApiKeyID: state.NextApiKeyID,
	}, nil
}

// ensureSeededLocked fills sections that older snapshots did not persist. It is
// intentionally field-granular so a partially populated snapshot keeps whatever
// is already there; tasks, wallet balances and transactions are never touched.
func (s *Store) ensureSeededLocked() {
	// The expert overview is read-only demo data with no write path, so a
	// persisted snapshot can never contain anything newer than the seed. Always
	// replace it unconditionally; otherwise an old snapshot (for example one
	// whose risk section still says score=58/blocked=false) would shadow the
	// current seed and break the blocked == hasBlock(compliance) invariant.
	s.expert = seedExpert()

	gateway := seedGateway()
	if len(s.gateway.Models) == 0 {
		s.gateway.Models = gateway.Models
	}
	if len(s.gateway.Keys) == 0 {
		s.gateway.Keys = gateway.Keys
	}
	if s.gateway.Usage.Period == "" {
		s.gateway.Usage = gateway.Usage
	}

	if s.wallet.Vip.Level == "" {
		s.wallet.Vip = seedWalletVip()
	}
	if s.nextApiKeyID <= 0 {
		s.nextApiKeyID = seedNextApiKeyID
	}
}

// Close releases the database handle. It is safe to call more than once.
func (s *Store) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.db == nil {
		return nil
	}
	err := s.db.Close()
	s.db = nil
	return err
}

// persistLocked serializes the whole store and upserts it in a transaction.
// The caller must hold s.mu for writing (or be initializing the store).
func (s *Store) persistLocked() error {
	if s.db == nil {
		return nil
	}

	payload, err := json.Marshal(s.snapshotLocked())
	if err != nil {
		return fmt.Errorf("encode marketplace state: %w", err)
	}

	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("begin marketplace state transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.Exec(`
		INSERT INTO marketplace_state (id, version, payload, updated_at)
		VALUES (1, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			version    = excluded.version,
			payload    = excluded.payload,
			updated_at = excluded.updated_at;
	`, stateSchemaVersion, payload, time.Now().UTC().Format(time.RFC3339Nano)); err != nil {
		return fmt.Errorf("write marketplace state: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit marketplace state: %w", err)
	}
	return nil
}

// snapshotLocked copies the store state into the persisted representation.
// The caller must hold s.mu.
func (s *Store) snapshotLocked() persistedState {
	tasks := make([]persistedTaskRecord, len(s.tasks))
	for index, record := range s.tasks {
		tasks[index] = persistedTaskRecord{
			Detail:          record.detail,
			AcceptCount:     record.acceptCount,
			ValidationScore: record.validationScore,
		}
	}

	return persistedState{
		Me:           s.me,
		Users:        s.users,
		Tasks:        tasks,
		Traces:       s.traces,
		Wallet:       s.wallet,
		Reputations:  s.reputations,
		Expert:       s.expert,
		Gateway:      s.gateway,
		NextTaskID:   s.nextTaskID,
		NextRatingID: s.nextRatingID,
		NextTxID:     s.nextTxID,
		NextApiKeyID: s.nextApiKeyID,
	}
}
