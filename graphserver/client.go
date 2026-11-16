package graphserver

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"AIxVuln/graph"
)

// Client is the graph protocol boundary used by the dispatcher. Both the HTTP
// client and the in-process local client implement it, so callers can swap one
// for the other without changing task code.
type Client interface {
	CreateProject(ctx context.Context, p graph.Project) (graph.Project, error)
	GetProject(ctx context.Context, projectID string) (graph.Project, error)
	ListProjects(ctx context.Context) ([]graph.Project, error)
	DeclareIntent(ctx context.Context, projectID string, n graph.Node) (graph.Node, error)
	ClaimIntent(ctx context.Context, projectID, intentID, worker string, lease time.Duration) (graph.Node, error)
	HeartbeatIntent(ctx context.Context, projectID, intentID, worker string, lease time.Duration) (graph.Node, error)
	ReleaseIntent(ctx context.Context, projectID, intentID, worker string) (graph.Node, error)
	ConcludeIntent(ctx context.Context, projectID, intentID, worker, description, factType string) (graph.Node, graph.Node, error)
	Complete(ctx context.Context, projectID string, from []string, description, worker string) (graph.Project, error)
	ClaimReason(ctx context.Context, projectID, worker, trigger string, lease time.Duration) (graph.Project, error)
	HeartbeatReason(ctx context.Context, projectID, worker string, lease time.Duration) (graph.Project, error)
	ReleaseReason(ctx context.Context, projectID, worker string) (graph.Project, error)
	AddHint(ctx context.Context, projectID, content, creator string) (graph.Node, error)
	ExportYAML(ctx context.Context, projectID string) (string, error)
}

// httpClient is the Client implementation that talks to a graphserver over HTTP.
type httpClient struct {
	baseURL string
	token   string
	http    *http.Client
}

var _ Client = (*httpClient)(nil)

// NewHTTPClient returns a Client that calls the graph protocol mounted under
// /api/graph. baseURL may be either the server root or the full /api/graph
// prefix.
func NewHTTPClient(baseURL, token string) Client {
	return &httpClient{
		baseURL: normalizeBaseURL(baseURL),
		token:   token,
		http:    &http.Client{Timeout: 30 * time.Second},
	}
}

func normalizeBaseURL(baseURL string) string {
	base := strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if strings.HasSuffix(base, routePrefix) {
		return base
	}
	return base + routePrefix
}

func leaseSeconds(d time.Duration) int {
	if d <= 0 {
		return 0
	}
	seconds := int(d / time.Second)
	if seconds == 0 {
		return 1
	}
	return seconds
}

func projectPath(projectID string) string {
	return "/projects/" + url.PathEscape(projectID)
}

func intentPath(projectID, intentID string) string {
	return projectPath(projectID) + "/intents/" + url.PathEscape(intentID)
}

func (c *httpClient) do(ctx context.Context, method, path string, body, out any) ([]byte, error) {
	var reader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("graphserver: encode request: %w", err)
		}
		reader = bytes.NewReader(data)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, reader)
	if err != nil {
		return nil, fmt.Errorf("graphserver: build request: %w", err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("graphserver: request failed: %w", err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("graphserver: read response: %w", err)
	}
	if resp.StatusCode >= http.StatusBadRequest {
		return nil, statusError(resp.StatusCode, data)
	}
	if out != nil {
		if err := json.Unmarshal(data, out); err != nil {
			return nil, fmt.Errorf("graphserver: decode response: %w", err)
		}
	}
	return data, nil
}

// statusError converts protocol status codes back into graph sentinel errors.
func statusError(status int, body []byte) error {
	var payload struct {
		Error string `json:"error"`
	}
	message := strings.TrimSpace(string(body))
	if json.Unmarshal(body, &payload) == nil && strings.TrimSpace(payload.Error) != "" {
		message = payload.Error
	}
	switch status {
	case http.StatusNotFound:
		return fmt.Errorf("%w: %s", graph.ErrNotFound, message)
	case http.StatusConflict:
		return fmt.Errorf("%w: %s", graph.ErrConflict, message)
	case http.StatusForbidden:
		return fmt.Errorf("%w: %s", graph.ErrForbidden, message)
	case http.StatusUnprocessableEntity:
		return fmt.Errorf("%w: %s", graph.ErrInvalid, message)
	default:
		return fmt.Errorf("graphserver: http %d: %s", status, message)
	}
}

func (v projectView) toProject() graph.Project {
	p := graph.Project{
		ID:               v.ID,
		Title:            v.Title,
		Origin:           v.Origin,
		Goal:             v.Goal,
		Status:           v.Status,
		BootstrapEnabled: v.BootstrapEnabled,
		CreatedAt:        v.CreatedAt,
	}
	if v.Reason != nil {
		p.Reason = &graph.ReasonLease{
			Worker:          v.Reason.Worker,
			Trigger:         v.Reason.Trigger,
			StartedAt:       v.Reason.StartedAt,
			LastHeartbeatAt: v.Reason.LastHeartbeatAt,
		}
	}
	return p
}

func (v nodeView) toNode() graph.Node {
	return graph.Node{
		ID:          v.ID,
		ProjectID:   v.ProjectID,
		Kind:        v.Kind,
		Type:        v.Type,
		Content:     v.Content,
		State:       v.State,
		From:        v.From,
		To:          v.To,
		Creator:     v.Creator,
		Worker:      v.Worker,
		LeaseUntil:  v.LeaseUntil,
		CreatedAt:   v.CreatedAt,
		ConcludedAt: v.ConcludedAt,
	}
}

func (c *httpClient) CreateProject(ctx context.Context, p graph.Project) (graph.Project, error) {
	bootstrap := p.BootstrapEnabled
	req := createProjectRequest{
		ID:               p.ID,
		Title:            p.Title,
		Origin:           p.Origin,
		Goal:             p.Goal,
		BootstrapEnabled: &bootstrap,
	}
	var out projectView
	if _, err := c.do(ctx, http.MethodPost, "/projects", req, &out); err != nil {
		return graph.Project{}, err
	}
	return out.toProject(), nil
}

func (c *httpClient) GetProject(ctx context.Context, projectID string) (graph.Project, error) {
	var out projectView
	if _, err := c.do(ctx, http.MethodGet, projectPath(projectID), nil, &out); err != nil {
		return graph.Project{}, err
	}
	return out.toProject(), nil
}

func (c *httpClient) ListProjects(ctx context.Context) ([]graph.Project, error) {
	var out []projectView
	if _, err := c.do(ctx, http.MethodGet, "/projects", nil, &out); err != nil {
		return nil, err
	}
	projects := make([]graph.Project, 0, len(out))
	for _, v := range out {
		projects = append(projects, v.toProject())
	}
	return projects, nil
}

func (c *httpClient) DeclareIntent(ctx context.Context, projectID string, n graph.Node) (graph.Node, error) {
	req := intentRequest{
		Type:    n.Type,
		Content: n.Content,
		From:    n.From,
		Creator: n.Creator,
		Worker:  n.Worker,
	}
	var out nodeView
	if _, err := c.do(ctx, http.MethodPost, projectPath(projectID)+"/intents", req, &out); err != nil {
		return graph.Node{}, err
	}
	return out.toNode(), nil
}

func (c *httpClient) ClaimIntent(ctx context.Context, projectID, intentID, worker string, lease time.Duration) (graph.Node, error) {
	// The protocol expresses a claim as a heartbeat on an open intent.
	req := workerLeaseRequest{Worker: worker, LeaseSeconds: leaseSeconds(lease)}
	var out nodeView
	if _, err := c.do(ctx, http.MethodPost, intentPath(projectID, intentID)+"/heartbeat", req, &out); err != nil {
		return graph.Node{}, err
	}
	return out.toNode(), nil
}

func (c *httpClient) HeartbeatIntent(ctx context.Context, projectID, intentID, worker string, lease time.Duration) (graph.Node, error) {
	req := workerLeaseRequest{Worker: worker, LeaseSeconds: leaseSeconds(lease)}
	var out nodeView
	if _, err := c.do(ctx, http.MethodPost, intentPath(projectID, intentID)+"/heartbeat", req, &out); err != nil {
		return graph.Node{}, err
	}
	return out.toNode(), nil
}

func (c *httpClient) ReleaseIntent(ctx context.Context, projectID, intentID, worker string) (graph.Node, error) {
	var out nodeView
	if _, err := c.do(ctx, http.MethodPost, intentPath(projectID, intentID)+"/release", releaseRequest{Worker: worker}, &out); err != nil {
		return graph.Node{}, err
	}
	return out.toNode(), nil
}

func (c *httpClient) ConcludeIntent(ctx context.Context, projectID, intentID, worker, description, factType string) (graph.Node, graph.Node, error) {
	req := concludeRequest{Worker: worker, Description: description, FactType: factType}
	var out struct {
		Fact   nodeView `json:"fact"`
		Intent nodeView `json:"intent"`
	}
	if _, err := c.do(ctx, http.MethodPost, intentPath(projectID, intentID)+"/conclude", req, &out); err != nil {
		return graph.Node{}, graph.Node{}, err
	}
	return out.Fact.toNode(), out.Intent.toNode(), nil
}

func (c *httpClient) Complete(ctx context.Context, projectID string, from []string, description, worker string) (graph.Project, error) {
	req := completeRequest{From: from, Description: description, Worker: worker}
	var out projectView
	if _, err := c.do(ctx, http.MethodPost, projectPath(projectID)+"/complete", req, &out); err != nil {
		return graph.Project{}, err
	}
	return out.toProject(), nil
}

func (c *httpClient) ClaimReason(ctx context.Context, projectID, worker, trigger string, lease time.Duration) (graph.Project, error) {
	req := workerLeaseRequest{Worker: worker, Trigger: trigger, LeaseSeconds: leaseSeconds(lease)}
	var out projectView
	if _, err := c.do(ctx, http.MethodPost, projectPath(projectID)+"/reason/claim", req, &out); err != nil {
		return graph.Project{}, err
	}
	return out.toProject(), nil
}

func (c *httpClient) HeartbeatReason(ctx context.Context, projectID, worker string, lease time.Duration) (graph.Project, error) {
	req := workerLeaseRequest{Worker: worker, LeaseSeconds: leaseSeconds(lease)}
	var out projectView
	if _, err := c.do(ctx, http.MethodPost, projectPath(projectID)+"/reason/heartbeat", req, &out); err != nil {
		return graph.Project{}, err
	}
	return out.toProject(), nil
}

func (c *httpClient) ReleaseReason(ctx context.Context, projectID, worker string) (graph.Project, error) {
	var out projectView
	if _, err := c.do(ctx, http.MethodPost, projectPath(projectID)+"/reason/release", releaseRequest{Worker: worker}, &out); err != nil {
		return graph.Project{}, err
	}
	return out.toProject(), nil
}

func (c *httpClient) AddHint(ctx context.Context, projectID, content, creator string) (graph.Node, error) {
	req := hintRequest{Content: content, Creator: creator}
	var out nodeView
	if _, err := c.do(ctx, http.MethodPost, projectPath(projectID)+"/hints", req, &out); err != nil {
		return graph.Node{}, err
	}
	return out.toNode(), nil
}

func (c *httpClient) ExportYAML(ctx context.Context, projectID string) (string, error) {
	data, err := c.do(ctx, http.MethodGet, projectPath(projectID)+"/export?format=yaml", nil, nil)
	if err != nil {
		return "", err
	}
	return string(data), nil
}
