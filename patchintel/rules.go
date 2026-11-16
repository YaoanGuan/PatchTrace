// Package patchintel implements the PatchOracle patch-semantic-inversion
// pipeline: patch ingestion, LLM semantic inversion and the versioned rule
// base that feeds confirmed knowledge back into later inversions.
package patchintel

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"AIxVuln/misc"

	_ "modernc.org/sqlite"
)

// Rule is one versioned entry of the rule base.
type Rule struct {
	ID                int64   `json:"id"`
	CVEID             string  `json:"cve_id,omitempty"`
	Pattern           string  `json:"pattern"`
	TriggerFeatures   string  `json:"trigger_features,omitempty"`
	VerificationNotes string  `json:"verification_notes,omitempty"`
	Confidence        float64 `json:"confidence"`
	Version           int     `json:"version"`
	HitCount          int     `json:"hit_count"`
	ConfirmCount      int     `json:"confirm_count"`
	CreatedAt         string  `json:"created_at"`
	UpdatedAt         string  `json:"updated_at"`
}

// RuleBase is a SQLite-backed, versioned rule store.
type RuleBase struct {
	db *sql.DB
}

const ruleSchema = `
CREATE TABLE IF NOT EXISTS rules (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    cve_id TEXT,
    pattern TEXT NOT NULL,
    trigger_features TEXT,
    verification_notes TEXT,
    confidence REAL DEFAULT 0.5,
    version INTEGER DEFAULT 1,
    hit_count INTEGER DEFAULT 0,
    confirm_count INTEGER DEFAULT 0,
    created_at DATETIME,
    updated_at DATETIME
);
CREATE INDEX IF NOT EXISTS idx_rules_pattern ON rules(pattern);
CREATE INDEX IF NOT EXISTS idx_rules_cve ON rules(cve_id);
CREATE TABLE IF NOT EXISTS idea_rule_links (
    idea_id TEXT PRIMARY KEY,
    rule_id INTEGER NOT NULL,
    created_at DATETIME
);
`

var (
	defaultOnce sync.Once
	defaultBase *RuleBase
	defaultErr  error
)

// OpenRuleBase opens (or creates) a rule base at dbPath.
func OpenRuleBase(dbPath string) (*RuleBase, error) {
	if strings.TrimSpace(dbPath) == "" {
		return nil, fmt.Errorf("rule base path is required")
	}
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o755); err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	if _, err := db.Exec(ruleSchema); err != nil {
		db.Close()
		return nil, fmt.Errorf("init rule schema: %w", err)
	}
	return &RuleBase{db: db}, nil
}

// DefaultRuleBase returns the process-wide rule base stored under
// <DATA_DIR>/patchintel/rules.db.
func DefaultRuleBase() (*RuleBase, error) {
	defaultOnce.Do(func() {
		path := filepath.Join(misc.GetDataDir(), "patchintel", "rules.db")
		defaultBase, defaultErr = OpenRuleBase(path)
	})
	return defaultBase, defaultErr
}

// Close releases the database handle.
func (r *RuleBase) Close() error {
	if r == nil || r.db == nil {
		return nil
	}
	return r.db.Close()
}

// Promote stores a new rule version. Existing rules with the same pattern are
// never overwritten: the new row gets version = previous max + 1.
//
// ConfirmCount is persisted as given (negative values are clamped to 0), which
// lets a caller that promotes a rule from an already-confirmed outcome seed the
// confirmation count; CE4 passes 1 for a confirmed verdict and 0 otherwise.
// HitCount always starts at 0 because a promotion is not a prompt hit.
func (r *RuleBase) Promote(input Rule) (*Rule, error) {
	pattern := strings.TrimSpace(input.Pattern)
	if pattern == "" {
		return nil, fmt.Errorf("rule pattern is required")
	}
	confidence := clampConfidence(input.Confidence)
	if confidence <= 0 {
		confidence = 0.5
	}
	confirmCount := input.ConfirmCount
	if confirmCount < 0 {
		confirmCount = 0
	}
	version, err := r.nextVersion(pattern)
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC().Format(time.RFC3339)
	result, err := r.db.Exec(`
        INSERT INTO rules (cve_id, pattern, trigger_features, verification_notes, confidence, version, confirm_count, created_at, updated_at)
        VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		strings.TrimSpace(input.CVEID), pattern, strings.TrimSpace(input.TriggerFeatures),
		strings.TrimSpace(input.VerificationNotes), confidence, version, confirmCount, now, now)
	if err != nil {
		return nil, err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return nil, err
	}
	return r.Get(id)
}

// UpdateOutcome records a verification outcome.
//
// A confirmed outcome inserts a new rule version with a raised confidence and
// confirm_count+1, preserving the previous version for auditability. A failed
// outcome lowers the confidence of the existing row in place and does not open
// a new version: a negative signal revises the current rule rather than
// establishing a new revision (CE4).
func (r *RuleBase) UpdateOutcome(ruleID int64, confirmed bool, note string) (*Rule, error) {
	current, err := r.Get(ruleID)
	if err != nil {
		return nil, err
	}
	notes := appendRuleNote(current.VerificationNotes, note)
	now := time.Now().UTC().Format(time.RFC3339)

	if !confirmed {
		confidence := clampConfidence(current.Confidence - 0.15)
		if _, err := r.db.Exec(`
            UPDATE rules SET confidence=?, verification_notes=?, updated_at=? WHERE id=?`,
			confidence, notes, now, current.ID); err != nil {
			return nil, err
		}
		return r.Get(current.ID)
	}

	confidence := clampConfidence(current.Confidence + 0.1)
	confirmCount := current.ConfirmCount + 1
	version, err := r.nextVersion(current.Pattern)
	if err != nil {
		return nil, err
	}
	result, err := r.db.Exec(`
        INSERT INTO rules (cve_id, pattern, trigger_features, verification_notes, confidence, version, hit_count, confirm_count, created_at, updated_at)
        VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		current.CVEID, current.Pattern, current.TriggerFeatures, notes, confidence, version,
		current.HitCount, confirmCount, now, now)
	if err != nil {
		return nil, err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return nil, err
	}
	return r.Get(id)
}

// appendRuleNote appends a verification note to the existing notes.
func appendRuleNote(notes, note string) string {
	note = strings.TrimSpace(note)
	if note == "" {
		return notes
	}
	if notes != "" {
		notes += "\n"
	}
	return notes + note
}

// Get returns one rule row by id.
func (r *RuleBase) Get(id int64) (*Rule, error) {
	row := r.db.QueryRow(`
        SELECT id, COALESCE(cve_id, ''), pattern, COALESCE(trigger_features, ''),
               COALESCE(verification_notes, ''), confidence, version, hit_count, confirm_count,
               COALESCE(created_at, ''), COALESCE(updated_at, '')
        FROM rules WHERE id = ?`, id)
	var rule Rule
	if err := row.Scan(&rule.ID, &rule.CVEID, &rule.Pattern, &rule.TriggerFeatures, &rule.VerificationNotes,
		&rule.Confidence, &rule.Version, &rule.HitCount, &rule.ConfirmCount, &rule.CreatedAt, &rule.UpdatedAt); err != nil {
		return nil, err
	}
	return &rule, nil
}

// Search returns the latest version of each matching pattern, ranked by keyword
// overlap first and confidence second.
func (r *RuleBase) Search(query string, limit int) ([]Rule, error) {
	if limit <= 0 {
		limit = 5
	}
	if limit > 50 {
		limit = 50
	}
	rows, err := r.db.Query(`
        SELECT r.id, COALESCE(r.cve_id, ''), r.pattern, COALESCE(r.trigger_features, ''),
               COALESCE(r.verification_notes, ''), r.confidence, r.version, r.hit_count, r.confirm_count,
               COALESCE(r.created_at, ''), COALESCE(r.updated_at, '')
        FROM rules r
        WHERE r.version = (SELECT MAX(version) FROM rules r2 WHERE r2.pattern = r.pattern)`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	terms := searchTerms(query)
	rules := make([]Rule, 0, 32)
	for rows.Next() {
		var rule Rule
		if err := rows.Scan(&rule.ID, &rule.CVEID, &rule.Pattern, &rule.TriggerFeatures, &rule.VerificationNotes,
			&rule.Confidence, &rule.Version, &rule.HitCount, &rule.ConfirmCount, &rule.CreatedAt, &rule.UpdatedAt); err != nil {
			return nil, err
		}
		rules = append(rules, rule)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	type scored struct {
		rule  Rule
		score int
	}
	ranked := make([]scored, 0, len(rules))
	for _, rule := range rules {
		ranked = append(ranked, scored{rule: rule, score: scoreRule(rule, terms)})
	}
	sort.SliceStable(ranked, func(i, j int) bool {
		if ranked[i].score != ranked[j].score {
			return ranked[i].score > ranked[j].score
		}
		if ranked[i].rule.Confidence != ranked[j].rule.Confidence {
			return ranked[i].rule.Confidence > ranked[j].rule.Confidence
		}
		return ranked[i].rule.ID > ranked[j].rule.ID
	})
	if len(ranked) > limit {
		ranked = ranked[:limit]
	}
	out := make([]Rule, 0, len(ranked))
	for _, item := range ranked {
		out = append(out, item.rule)
	}
	return out, nil
}

// RecordHits increments hit_count for the rules injected into a prompt.
func (r *RuleBase) RecordHits(ids []int64) error {
	if len(ids) == 0 {
		return nil
	}
	placeholders := make([]string, 0, len(ids))
	args := make([]interface{}, 0, len(ids))
	for _, id := range ids {
		placeholders = append(placeholders, "?")
		args = append(args, id)
	}
	_, err := r.db.Exec("UPDATE rules SET hit_count = hit_count + 1 WHERE id IN ("+strings.Join(placeholders, ",")+")", args...)
	return err
}

// LatestPatterns returns the newest version of every pattern, optionally
// filtered by CVE id.
func (r *RuleBase) LatestPatterns(cveID string, limit int) ([]Rule, error) {
	rules, err := r.Search("", 50)
	if err != nil {
		return nil, err
	}
	cveID = strings.TrimSpace(cveID)
	out := make([]Rule, 0, len(rules))
	for _, rule := range rules {
		if cveID != "" && !strings.EqualFold(rule.CVEID, cveID) {
			continue
		}
		out = append(out, rule)
	}
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

// FindLatestByPattern returns the newest version of an exact pattern, or nil
// when the pattern is not in the rule base yet.
func (r *RuleBase) FindLatestByPattern(pattern string) (*Rule, error) {
	pattern = strings.TrimSpace(pattern)
	if pattern == "" {
		return nil, nil
	}
	var id int64
	err := r.db.QueryRow(
		"SELECT id FROM rules WHERE pattern = ? ORDER BY version DESC LIMIT 1", pattern).Scan(&id)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return r.Get(id)
}

// LinkIdea associates a PatchOracle exploit idea with the rule it was derived
// from, so the verification outcome can flow back into the rule base.
func (r *RuleBase) LinkIdea(ideaID string, ruleID int64) error {
	ideaID = strings.TrimSpace(ideaID)
	if ideaID == "" || ruleID <= 0 {
		return nil
	}
	_, err := r.db.Exec(`
        INSERT INTO idea_rule_links (idea_id, rule_id, created_at)
        VALUES (?, ?, ?)
        ON CONFLICT(idea_id) DO UPDATE SET rule_id = excluded.rule_id`,
		ideaID, ruleID, time.Now().UTC().Format(time.RFC3339))
	return err
}

// RecordIdeaOutcome applies a verification outcome to the rule linked with the
// idea. Ideas without a link are ignored, so ordinary (non-PatchOracle) ideas
// never touch the rule base.
func (r *RuleBase) RecordIdeaOutcome(ideaID string, confirmed bool, note string) (*Rule, error) {
	var ruleID int64
	err := r.db.QueryRow("SELECT rule_id FROM idea_rule_links WHERE idea_id = ?", strings.TrimSpace(ideaID)).Scan(&ruleID)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	updated, err := r.UpdateOutcome(ruleID, confirmed, note)
	if err != nil {
		return nil, err
	}
	if err := r.LinkIdea(ideaID, updated.ID); err != nil {
		return updated, err
	}
	return updated, nil
}

func (r *RuleBase) nextVersion(pattern string) (int, error) {
	var version int
	err := r.db.QueryRow("SELECT COALESCE(MAX(version), 0) FROM rules WHERE pattern = ?", pattern).Scan(&version)
	if err != nil {
		return 0, err
	}
	return version + 1, nil
}

func clampConfidence(value float64) float64 {
	if value < 0 {
		return 0
	}
	if value > 1 {
		return 1
	}
	return value
}

func searchTerms(query string) []string {
	fields := strings.Fields(strings.ToLower(strings.TrimSpace(query)))
	seen := make(map[string]struct{}, len(fields))
	out := make([]string, 0, len(fields))
	for _, field := range fields {
		field = strings.Trim(field, " \t,;:!?\"'()[]{}<>|\\")
		if len([]rune(field)) < 2 {
			continue
		}
		if _, ok := seen[field]; ok {
			continue
		}
		seen[field] = struct{}{}
		out = append(out, field)
	}
	return out
}

func scoreRule(rule Rule, terms []string) int {
	if len(terms) == 0 {
		return 0
	}
	haystack := strings.ToLower(rule.Pattern + "\n" + rule.TriggerFeatures + "\n" + rule.CVEID + "\n" + rule.VerificationNotes)
	score := 0
	for _, term := range terms {
		if strings.Contains(haystack, term) {
			score++
		}
	}
	return score
}
