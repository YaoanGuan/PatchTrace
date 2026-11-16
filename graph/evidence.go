package graph

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// AddVerdict records an evidence verdict as a resolved fact and links it to the
// node it judges with a support or refute edge (R10). The target is validated
// inside the same transaction, so an unknown target leaves the graph untouched.
// The project must be active.
//
// The verdict fact carries the description plus, when non-empty, the evidence
// reference so the observation the verdict rests on travels with the fact.
func (s *Store) AddVerdict(ctx context.Context, projectID, targetID, verdict, description, evidenceRef string) (Node, error) {
	projectID = strings.TrimSpace(projectID)
	targetID = strings.TrimSpace(targetID)
	verdict = strings.TrimSpace(verdict)
	description = strings.TrimSpace(description)
	evidenceRef = strings.TrimSpace(evidenceRef)
	if projectID == "" || targetID == "" || description == "" {
		return Node{}, fmt.Errorf("%w: project, target and description are required", ErrInvalid)
	}
	if verdict != "support" && verdict != "refute" {
		return Node{}, fmt.Errorf("%w: verdict must be support or refute", ErrInvalid)
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Node{}, fmt.Errorf("graph: begin add verdict: %w", err)
	}
	defer tx.Rollback()

	if _, err := ensureActive(ctx, tx, projectID); err != nil {
		return Node{}, err
	}
	if _, err := getNodeRaw(ctx, tx, projectID, targetID); err != nil {
		return Node{}, err
	}

	id, err := nextSeq(ctx, tx, projectID, "f")
	if err != nil {
		return Node{}, err
	}
	content := description
	if evidenceRef != "" {
		content += "\n证据引用：" + evidenceRef
	}
	now := time.Now()
	fact := Node{
		ID:        id,
		ProjectID: projectID,
		Kind:      "fact",
		Type:      "verdict",
		Content:   content,
		State:     "resolved",
		CreatedAt: &now,
	}
	if err := insertNode(ctx, tx, fact); err != nil {
		return Node{}, err
	}
	if err := insertEdge(ctx, tx, projectID, fact.ID, targetID, verdict); err != nil {
		return Node{}, err
	}
	if err := tx.Commit(); err != nil {
		return Node{}, fmt.Errorf("graph: commit add verdict: %w", err)
	}
	return fact, nil
}
