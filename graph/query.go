package graph

import (
	"context"
	"fmt"
	"sort"
	"strings"
)

// provenanceEdgeKinds are the causal edge kinds followed when walking a
// provenance chain back to the project origin.
var provenanceEdgeKinds = map[string]bool{
	"produce": true,
	"derive":  true,
	"support": true,
}

// Provenance returns the ordered causal chain from the project origin to
// nodeID, following produce, derive and support edges backwards. Nodes already
// on the current path are skipped, so cycles terminate. A missing node or a
// node without a chain to origin yields ErrNotFound.
func (s *Store) Provenance(ctx context.Context, projectID, nodeID string) ([]Node, error) {
	projectID = strings.TrimSpace(projectID)
	nodeID = strings.TrimSpace(nodeID)
	if projectID == "" || nodeID == "" {
		return nil, fmt.Errorf("%w: project and node are required", ErrInvalid)
	}

	target, err := s.GetNode(ctx, projectID, nodeID)
	if err != nil {
		return nil, err
	}
	nodes, err := s.Nodes(ctx, projectID)
	if err != nil {
		return nil, err
	}
	edges, err := s.Edges(ctx, projectID)
	if err != nil {
		return nil, err
	}

	byID := make(map[string]Node, len(nodes))
	for _, n := range nodes {
		byID[n.ID] = n
	}

	// predecessors[to] lists the causal sources of to.
	predecessors := make(map[string][]string)
	for _, e := range edges {
		if provenanceEdgeKinds[e.Kind] {
			predecessors[e.To] = append(predecessors[e.To], e.From)
		}
	}
	for id := range predecessors {
		sort.Strings(predecessors[id])
	}

	// Iterative DFS from the target towards the origin. The path slice mirrors
	// the stack from target to the current node and is reversed on success.
	type frame struct {
		id   string
		next int
	}
	path := []string{target.ID}
	onPath := map[string]bool{target.ID: true}
	stack := []frame{{id: target.ID}}
	for len(stack) > 0 {
		top := &stack[len(stack)-1]
		if top.id == "origin" {
			chain := make([]Node, 0, len(path))
			for i := len(path) - 1; i >= 0; i-- {
				n, ok := byID[path[i]]
				if !ok {
					return nil, fmt.Errorf("%w: node %s", ErrNotFound, path[i])
				}
				chain = append(chain, n)
			}
			return chain, nil
		}
		if top.next < len(predecessors[top.id]) {
			next := predecessors[top.id][top.next]
			top.next++
			if onPath[next] {
				continue
			}
			if _, ok := byID[next]; !ok {
				continue
			}
			path = append(path, next)
			onPath[next] = true
			stack = append(stack, frame{id: next})
			continue
		}
		stack = stack[:len(stack)-1]
		onPath[top.id] = false
		path = path[:len(path)-1]
	}
	return nil, fmt.Errorf("%w: no provenance path from origin to %s", ErrNotFound, nodeID)
}

// Gaps returns open intents that no unsettled intent blocks. Results are
// ordered by the number of unmet block dependencies ascending, then created_at
// ascending.
func (s *Store) Gaps(ctx context.Context, projectID string) ([]Node, error) {
	projectID = strings.TrimSpace(projectID)
	if projectID == "" {
		return nil, fmt.Errorf("%w: project is required", ErrInvalid)
	}
	if _, err := s.GetProject(ctx, projectID); err != nil {
		return nil, err
	}
	nodes, err := s.Nodes(ctx, projectID)
	if err != nil {
		return nil, err
	}
	edges, err := s.Edges(ctx, projectID)
	if err != nil {
		return nil, err
	}

	state := make(map[string]string, len(nodes))
	for _, n := range nodes {
		state[n.ID] = n.State
	}

	// unmetBlockers[id] counts block edges pointing at id whose source is not
	// settled yet. A block dependency is met only once the blocker is resolved
	// (open, claimed and unknown states are all unmet). A block edge runs from
	// the blocker to the blocked node.
	unmetBlockers := make(map[string]int)
	for _, e := range edges {
		if e.Kind == "block" && state[e.From] != "resolved" {
			unmetBlockers[e.To]++
		}
	}

	gaps := make([]Node, 0)
	for _, n := range nodes {
		if n.Kind != "intent" || n.State != "open" {
			continue
		}
		if unmetBlockers[n.ID] > 0 {
			continue
		}
		gaps = append(gaps, n)
	}

	sort.SliceStable(gaps, func(i, j int) bool {
		ci, cj := unmetBlockers[gaps[i].ID], unmetBlockers[gaps[j].ID]
		if ci != cj {
			return ci < cj
		}
		ti, tj := gaps[i].CreatedAt, gaps[j].CreatedAt
		switch {
		case ti == nil && tj != nil:
			return true
		case ti != nil && tj == nil:
			return false
		case ti != nil && !ti.Equal(*tj):
			return ti.Before(*tj)
		}
		return gaps[i].ID < gaps[j].ID
	})
	return gaps, nil
}
