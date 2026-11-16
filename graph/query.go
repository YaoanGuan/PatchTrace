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

// Gap is one open intent ranked by GapScore.
//
// OpenBlockers is the number of distinct block-edge blockers of the intent
// whose state is not "resolved" (the same predicate Gaps uses to decide
// readiness). DepthFromOrigin is 1 plus the minimum BFS hop distance from the
// project's "origin" fact to any of the intent's from facts, walking every
// edge in its from -> to direction; when the intent has no from facts or none
// of them is reachable, it is len(nodes)+1. Small values mean the intent sits
// close to the patch.
type Gap struct {
	IntentID        string
	Description     string
	OpenBlockers    int
	DepthFromOrigin int
}

// GapScore ranks every open intent of a project. Results are ordered by
// OpenBlockers ascending, then DepthFromOrigin ascending, then IntentID
// ascending. The ordering is deterministic and intended for prompt context.
func (s *Store) GapScore(ctx context.Context, projectID string) ([]Gap, error) {
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

	// openBlockers[id] is the set of distinct unmet blockers pointing at id. A
	// block edge runs blocker -> blocked and is met only once the blocker is
	// resolved; open, claimed and unknown states are all unmet.
	openBlockers := make(map[string]map[string]struct{})
	adjacency := make(map[string][]string)
	for _, e := range edges {
		if e.Kind == "block" && state[e.From] != "resolved" {
			if openBlockers[e.To] == nil {
				openBlockers[e.To] = make(map[string]struct{})
			}
			openBlockers[e.To][e.From] = struct{}{}
		}
		adjacency[e.From] = append(adjacency[e.From], e.To)
	}

	// BFS from origin over every edge kind. Unreachable ids stay absent.
	distance := map[string]int{"origin": 0}
	queue := []string{"origin"}
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		for _, next := range adjacency[cur] {
			if _, seen := distance[next]; seen {
				continue
			}
			distance[next] = distance[cur] + 1
			queue = append(queue, next)
		}
	}
	unreachableDepth := len(nodes) + 1

	gaps := make([]Gap, 0)
	for _, n := range nodes {
		if n.Kind != "intent" || n.State != "open" {
			continue
		}
		depth := unreachableDepth
		for _, from := range n.From {
			if hops, ok := distance[from]; ok && hops+1 < depth {
				depth = hops + 1
			}
		}
		gaps = append(gaps, Gap{
			IntentID:        n.ID,
			Description:     n.Content,
			OpenBlockers:    len(openBlockers[n.ID]),
			DepthFromOrigin: depth,
		})
	}

	sort.Slice(gaps, func(i, j int) bool {
		if gaps[i].OpenBlockers != gaps[j].OpenBlockers {
			return gaps[i].OpenBlockers < gaps[j].OpenBlockers
		}
		if gaps[i].DepthFromOrigin != gaps[j].DepthFromOrigin {
			return gaps[i].DepthFromOrigin < gaps[j].DepthFromOrigin
		}
		return gaps[i].IntentID < gaps[j].IntentID
	})
	return gaps, nil
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
