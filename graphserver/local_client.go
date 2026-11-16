package graphserver

import (
	"context"
	"time"

	"AIxVuln/graph"
)

// localClient delegates every Client call directly to a graph.Store. It is used
// by in-process tests and by embedders that do not need the HTTP loopback.
type localClient struct {
	store *graph.Store
}

var _ Client = (*localClient)(nil)

// NewLocalClient returns a Client backed directly by store.
func NewLocalClient(store *graph.Store) Client {
	return &localClient{store: store}
}

func (c *localClient) CreateProject(ctx context.Context, p graph.Project) (graph.Project, error) {
	return c.store.CreateProject(ctx, p)
}

func (c *localClient) GetProject(ctx context.Context, projectID string) (graph.Project, error) {
	return c.store.GetProject(ctx, projectID)
}

func (c *localClient) ListProjects(ctx context.Context) ([]graph.Project, error) {
	return c.store.ListProjects(ctx)
}

func (c *localClient) DeclareIntent(ctx context.Context, projectID string, n graph.Node) (graph.Node, error) {
	return c.store.DeclareIntent(ctx, projectID, n)
}

func (c *localClient) ClaimIntent(ctx context.Context, projectID, intentID, worker string, lease time.Duration) (graph.Node, error) {
	return c.store.ClaimIntent(ctx, projectID, intentID, worker, lease)
}

func (c *localClient) HeartbeatIntent(ctx context.Context, projectID, intentID, worker string, lease time.Duration) (graph.Node, error) {
	return c.store.HeartbeatIntent(ctx, projectID, intentID, worker, lease)
}

func (c *localClient) ReleaseIntent(ctx context.Context, projectID, intentID, worker string) (graph.Node, error) {
	return c.store.ReleaseIntent(ctx, projectID, intentID, worker)
}

func (c *localClient) ConcludeIntent(ctx context.Context, projectID, intentID, worker, description, factType string) (graph.Node, graph.Node, error) {
	return c.store.ConcludeIntent(ctx, projectID, intentID, worker, description, factType)
}

func (c *localClient) Complete(ctx context.Context, projectID string, from []string, description, worker string) (graph.Project, error) {
	return c.store.Complete(ctx, projectID, from, description, worker)
}

func (c *localClient) ClaimReason(ctx context.Context, projectID, worker, trigger string, lease time.Duration) (graph.Project, error) {
	return c.store.ClaimReason(ctx, projectID, worker, trigger, lease)
}

func (c *localClient) HeartbeatReason(ctx context.Context, projectID, worker string, lease time.Duration) (graph.Project, error) {
	return c.store.HeartbeatReason(ctx, projectID, worker, lease)
}

func (c *localClient) ReleaseReason(ctx context.Context, projectID, worker string) (graph.Project, error) {
	return c.store.ReleaseReason(ctx, projectID, worker)
}

func (c *localClient) AddHint(ctx context.Context, projectID, content, creator string) (graph.Node, error) {
	return c.store.AddHint(ctx, projectID, content, creator)
}

func (c *localClient) ExportYAML(ctx context.Context, projectID string) (string, error) {
	return c.store.ExportYAML(ctx, projectID)
}

// GapScore exposes the store's authoritative gap ranking to dispatcher tasks
// through the optional type assertion they already use. It is deliberately not
// part of Client: the HTTP protocol has no edge endpoint.
func (c *localClient) GapScore(ctx context.Context, projectID string) ([]graph.Gap, error) {
	return c.store.GapScore(ctx, projectID)
}

// AddVerdict exposes store-backed evidence verdicts (R10) to explore tasks
// through the optional type assertion they already use.
func (c *localClient) AddVerdict(ctx context.Context, projectID, targetID, verdict, description, evidenceSourceID, evidenceRef string) (graph.Node, error) {
	return c.store.AddVerdict(ctx, projectID, targetID, verdict, description, evidenceSourceID, evidenceRef)
}
