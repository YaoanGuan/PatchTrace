package graphserver

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"AIxVuln/graph"

	"github.com/gin-gonic/gin"
)

func newTestStore(t *testing.T) *graph.Store {
	t.Helper()
	store, err := graph.Open(filepath.Join(t.TempDir(), "graph.db"))
	if err != nil {
		t.Fatalf("open graph store: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	return store
}

func newTestRouter(t *testing.T) (*gin.Engine, *graph.Store) {
	t.Helper()
	store := newTestStore(t)
	gin.SetMode(gin.TestMode)
	router := gin.New()
	New(store).Register(router)
	return router, store
}

func doRequest(t *testing.T, h http.Handler, method, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var reader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal request body: %v", err)
		}
		reader = bytes.NewReader(data)
	}
	req := httptest.NewRequest(method, path, reader)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func decodeBody(t *testing.T, rec *httptest.ResponseRecorder, out any) {
	t.Helper()
	if err := json.Unmarshal(rec.Body.Bytes(), out); err != nil {
		t.Fatalf("decode response (status %d, body %q): %v", rec.Code, rec.Body.String(), err)
	}
}

func mustStatus(t *testing.T, rec *httptest.ResponseRecorder, want int) {
	t.Helper()
	if rec.Code != want {
		t.Fatalf("status = %d, want %d; body = %s", rec.Code, want, rec.Body.String())
	}
}

func createProject(t *testing.T, h http.Handler, extra map[string]any) string {
	t.Helper()
	body := map[string]any{
		"title":  "CVE-2025-3248",
		"origin": "diff.patch",
		"goal":   "prove remote code execution",
	}
	for k, v := range extra {
		body[k] = v
	}
	rec := doRequest(t, h, http.MethodPost, "/api/graph/projects", body)
	mustStatus(t, rec, http.StatusCreated)
	var out map[string]any
	decodeBody(t, rec, &out)
	id, _ := out["id"].(string)
	if id == "" {
		t.Fatalf("created project has no id: %s", rec.Body.String())
	}
	return id
}

func declareIntent(t *testing.T, h http.Handler, projectID, description, creator string) string {
	t.Helper()
	rec := doRequest(t, h, http.MethodPost, "/api/graph/projects/"+projectID+"/intents", map[string]any{
		"type":        "gap",
		"description": description,
		"from":        []string{"origin"},
		"creator":     creator,
	})
	mustStatus(t, rec, http.StatusCreated)
	var out map[string]any
	decodeBody(t, rec, &out)
	id, _ := out["id"].(string)
	if id == "" {
		t.Fatalf("declared intent has no id: %s", rec.Body.String())
	}
	return id
}

func TestRoutesLifecycle(t *testing.T) {
	router, _ := newTestRouter(t)
	projectID := createProject(t, router, nil)
	intentID := declareIntent(t, router, projectID, "verify /api/v1/validate/code", "worker-1")

	rec := doRequest(t, router, http.MethodPost,
		"/api/graph/projects/"+projectID+"/intents/"+intentID+"/heartbeat",
		map[string]any{"worker": "worker-1"})
	mustStatus(t, rec, http.StatusOK)

	rec = doRequest(t, router, http.MethodPost,
		"/api/graph/projects/"+projectID+"/intents/"+intentID+"/conclude",
		map[string]any{"worker": "worker-1", "description": "endpoint executes attacker code"})
	mustStatus(t, rec, http.StatusOK)
	var concluded map[string]any
	decodeBody(t, rec, &concluded)
	fact, ok := concluded["fact"].(map[string]any)
	if !ok {
		t.Fatalf("conclude response missing fact: %s", rec.Body.String())
	}
	intent, ok := concluded["intent"].(map[string]any)
	if !ok {
		t.Fatalf("conclude response missing intent: %s", rec.Body.String())
	}
	factID, _ := fact["id"].(string)
	if factID == "" {
		t.Fatalf("concluded fact has no id: %s", rec.Body.String())
	}
	if got, _ := intent["to"].(string); got != factID {
		t.Fatalf("intent.to = %q, want %q", got, factID)
	}

	rec = doRequest(t, router, http.MethodPost, "/api/graph/projects/"+projectID+"/complete",
		map[string]any{"from": []string{factID}, "description": "goal demonstrated", "worker": "worker-1"})
	mustStatus(t, rec, http.StatusOK)
	var project map[string]any
	decodeBody(t, rec, &project)
	if got, _ := project["status"].(string); got != "completed" {
		t.Fatalf("project status = %q, want completed", got)
	}

	rec = doRequest(t, router, http.MethodGet, "/api/graph/projects/"+projectID+"/export?format=yaml", nil)
	mustStatus(t, rec, http.StatusOK)
	body := rec.Body.String()
	for _, want := range []string{"project:", "facts:", "intents:", factID} {
		if !strings.Contains(body, want) {
			t.Fatalf("yaml export missing %q:\n%s", want, body)
		}
	}
}

func TestStoppedProjectRejectsWrites(t *testing.T) {
	router, _ := newTestRouter(t)
	projectID := createProject(t, router, nil)

	rec := doRequest(t, router, http.MethodPut, "/api/graph/projects/"+projectID+"/status",
		map[string]any{"status": "stopped"})
	mustStatus(t, rec, http.StatusOK)

	rec = doRequest(t, router, http.MethodPost, "/api/graph/projects/"+projectID+"/intents", map[string]any{
		"type":        "gap",
		"description": "should be rejected",
		"from":        []string{"origin"},
		"creator":     "worker-1",
	})
	mustStatus(t, rec, http.StatusForbidden)
}

func TestConflictStatusCodes(t *testing.T) {
	router, _ := newTestRouter(t)
	projectID := createProject(t, router, nil)
	intentID := declareIntent(t, router, projectID, "verify endpoint", "worker-1")

	rec := doRequest(t, router, http.MethodPost,
		"/api/graph/projects/"+projectID+"/intents/"+intentID+"/heartbeat",
		map[string]any{"worker": "worker-1"})
	mustStatus(t, rec, http.StatusOK)

	rec = doRequest(t, router, http.MethodPost,
		"/api/graph/projects/"+projectID+"/intents/"+intentID+"/heartbeat",
		map[string]any{"worker": "worker-2"})
	mustStatus(t, rec, http.StatusConflict)
}

func TestInvalidBodyReturns422(t *testing.T) {
	router, _ := newTestRouter(t)
	projectID := createProject(t, router, nil)

	rec := doRequest(t, router, http.MethodPost, "/api/graph/projects/"+projectID+"/intents", map[string]any{
		"type":        "gap",
		"description": "   ",
		"from":        []string{"origin"},
		"creator":     "worker-1",
	})
	mustStatus(t, rec, http.StatusUnprocessableEntity)

	rec = doRequest(t, router, http.MethodPost, "/api/graph/projects/"+projectID+"/complete",
		map[string]any{"from": []string{"origin"}, "description": "  ", "worker": "worker-1"})
	mustStatus(t, rec, http.StatusUnprocessableEntity)
}

func TestProvenanceEndpoint(t *testing.T) {
	router, _ := newTestRouter(t)
	projectID := createProject(t, router, nil)
	intentID := declareIntent(t, router, projectID, "verify endpoint", "worker-1")

	rec := doRequest(t, router, http.MethodPost,
		"/api/graph/projects/"+projectID+"/intents/"+intentID+"/heartbeat",
		map[string]any{"worker": "worker-1"})
	mustStatus(t, rec, http.StatusOK)

	rec = doRequest(t, router, http.MethodPost,
		"/api/graph/projects/"+projectID+"/intents/"+intentID+"/conclude",
		map[string]any{"worker": "worker-1", "description": "endpoint executes attacker code"})
	mustStatus(t, rec, http.StatusOK)
	var concluded map[string]any
	decodeBody(t, rec, &concluded)
	factID := concluded["fact"].(map[string]any)["id"].(string)

	rec = doRequest(t, router, http.MethodGet,
		"/api/graph/projects/"+projectID+"/provenance?node="+factID, nil)
	mustStatus(t, rec, http.StatusOK)
	var chain []map[string]any
	decodeBody(t, rec, &chain)
	if len(chain) < 2 {
		t.Fatalf("provenance chain too short: %d (%s)", len(chain), rec.Body.String())
	}
	if got, _ := chain[0]["id"].(string); got != "origin" {
		t.Fatalf("provenance chain starts at %q, want origin", got)
	}

	rec = doRequest(t, router, http.MethodGet,
		"/api/graph/projects/"+projectID+"/provenance?node=does-not-exist", nil)
	mustStatus(t, rec, http.StatusNotFound)
}

func TestCreateProjectBootstrapDefaultsTrue(t *testing.T) {
	router, _ := newTestRouter(t)

	rec := doRequest(t, router, http.MethodPost, "/api/graph/projects", map[string]any{
		"title": "default bootstrap", "origin": "diff.patch", "goal": "prove",
	})
	mustStatus(t, rec, http.StatusCreated)
	var def map[string]any
	decodeBody(t, rec, &def)
	if got, _ := def["bootstrap_enabled"].(bool); !got {
		t.Fatalf("bootstrap_enabled = %v, want true when absent", def["bootstrap_enabled"])
	}

	rec = doRequest(t, router, http.MethodPost, "/api/graph/projects", map[string]any{
		"title": "explicit false", "origin": "diff.patch", "goal": "prove",
		"bootstrap_enabled": false,
	})
	mustStatus(t, rec, http.StatusCreated)
	var off map[string]any
	decodeBody(t, rec, &off)
	if got, ok := off["bootstrap_enabled"].(bool); !ok || got {
		t.Fatalf("bootstrap_enabled = %v, want explicit false preserved", off["bootstrap_enabled"])
	}
}

func TestHTTPClientLifecycle(t *testing.T) {
	store := newTestStore(t)
	gin.SetMode(gin.TestMode)
	router := gin.New()
	New(store).Register(router)
	srv := httptest.NewServer(router)
	defer srv.Close()

	ctx := context.Background()
	client := NewHTTPClient(srv.URL, "test-token")

	project, err := client.CreateProject(ctx, graph.Project{
		Title: "CVE-2025-3248", Origin: "diff.patch", Goal: "prove rce", BootstrapEnabled: true,
	})
	if err != nil {
		t.Fatalf("CreateProject: %v", err)
	}
	if !project.BootstrapEnabled {
		t.Fatalf("CreateProject lost bootstrap_enabled")
	}

	got, err := client.GetProject(ctx, project.ID)
	if err != nil {
		t.Fatalf("GetProject: %v", err)
	}
	if got.ID != project.ID {
		t.Fatalf("GetProject id = %q, want %q", got.ID, project.ID)
	}

	projects, err := client.ListProjects(ctx)
	if err != nil {
		t.Fatalf("ListProjects: %v", err)
	}
	if len(projects) != 1 || projects[0].ID != project.ID {
		t.Fatalf("ListProjects = %+v, want one project %s", projects, project.ID)
	}

	intent, err := client.DeclareIntent(ctx, project.ID, graph.Node{
		Type: "gap", Content: "verify endpoint", From: []string{"origin"}, Creator: "worker-1",
	})
	if err != nil {
		t.Fatalf("DeclareIntent: %v", err)
	}
	if intent.ID == "" {
		t.Fatalf("DeclareIntent returned empty id")
	}

	claimed, err := client.ClaimIntent(ctx, project.ID, intent.ID, "worker-1", time.Minute)
	if err != nil {
		t.Fatalf("ClaimIntent: %v", err)
	}
	if claimed.State != "claimed" || claimed.Worker != "worker-1" {
		t.Fatalf("ClaimIntent = %+v, want claimed by worker-1", claimed)
	}

	fact, concluded, err := client.ConcludeIntent(ctx, project.ID, intent.ID, "worker-1",
		"endpoint executes attacker code", "observation")
	if err != nil {
		t.Fatalf("ConcludeIntent: %v", err)
	}
	if fact.ID == "" || concluded.To == nil || *concluded.To != fact.ID {
		t.Fatalf("ConcludeIntent = fact %+v intent %+v", fact, concluded)
	}

	// The concluded intent is still held by worker-1, so a claim by another
	// worker is a conflict while the project is still active.
	if _, err := client.ClaimIntent(ctx, project.ID, intent.ID, "worker-2", time.Minute); !errors.Is(err, graph.ErrConflict) {
		t.Fatalf("ClaimIntent(concluded) error = %v, want ErrConflict", err)
	}

	completed, err := client.Complete(ctx, project.ID, []string{fact.ID}, "goal demonstrated", "worker-1")
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}
	if completed.Status != "completed" {
		t.Fatalf("Complete status = %q, want completed", completed.Status)
	}

	yaml, err := client.ExportYAML(ctx, project.ID)
	if err != nil {
		t.Fatalf("ExportYAML: %v", err)
	}
	if !strings.Contains(yaml, "project:") || !strings.Contains(yaml, fact.ID) {
		t.Fatalf("ExportYAML missing expected content:\n%s", yaml)
	}

	if _, err := client.GetProject(ctx, "missing-project"); !errors.Is(err, graph.ErrNotFound) {
		t.Fatalf("GetProject(missing) error = %v, want ErrNotFound", err)
	}
}

func TestLocalClientSupportsGapScoreAndVerdict(t *testing.T) {
	ctx := context.Background()
	store := newTestStore(t)
	client := NewLocalClient(store)

	project, err := client.CreateProject(ctx, graph.Project{
		Title: "local seams", Origin: "diff.patch", Goal: "prove", BootstrapEnabled: false,
	})
	if err != nil {
		t.Fatalf("CreateProject: %v", err)
	}
	intent, err := client.DeclareIntent(ctx, project.ID, graph.Node{
		Type: "gap", Content: "verify endpoint", From: []string{"origin"}, Creator: "worker-1",
	})
	if err != nil {
		t.Fatalf("DeclareIntent: %v", err)
	}

	scorer, ok := client.(interface {
		GapScore(context.Context, string) ([]graph.Gap, error)
	})
	if !ok {
		t.Fatal("the local client must implement GapScore for dispatcher reasoning")
	}
	gaps, err := scorer.GapScore(ctx, project.ID)
	if err != nil {
		t.Fatalf("GapScore: %v", err)
	}
	if len(gaps) != 1 || gaps[0].IntentID != intent.ID {
		t.Fatalf("GapScore = %+v, want the open intent %s", gaps, intent.ID)
	}
	if gaps[0].Description != "verify endpoint" {
		t.Fatalf("gap description = %q, want the intent content", gaps[0].Description)
	}

	observation, err := store.AddFact(ctx, project.ID, graph.Node{
		Type: "observation", Content: "sandbox: exit=0", Creator: "worker-1", From: []string{"origin"},
	})
	if err != nil {
		t.Fatalf("AddFact(observation): %v", err)
	}

	writer, ok := client.(interface {
		AddVerdict(context.Context, string, string, string, string, string, string) (graph.Node, error)
	})
	if !ok {
		t.Fatal("the local client must implement AddVerdict for explore evidence")
	}
	fact, err := writer.AddVerdict(ctx, project.ID, intent.ID, "support",
		"sandbox confirmed the endpoint", observation.ID, "file:///tmp/run.log")
	if err != nil {
		t.Fatalf("AddVerdict: %v", err)
	}
	if fact.Type != "verdict" {
		t.Fatalf("verdict fact type = %q, want verdict", fact.Type)
	}
	if !strings.Contains(fact.Content, "sandbox confirmed the endpoint") ||
		!strings.Contains(fact.Content, "file:///tmp/run.log") {
		t.Fatalf("verdict fact content = %q, want the description and evidence", fact.Content)
	}

	edges, err := store.Edges(ctx, project.ID)
	if err != nil {
		t.Fatalf("Edges: %v", err)
	}
	found := false
	for _, edge := range edges {
		if edge.From == fact.ID && edge.To == intent.ID && edge.Kind == "support" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("edges = %+v, want a support edge %s -> %s", edges, fact.ID, intent.ID)
	}
}

func TestLocalClientLifecycle(t *testing.T) {
	ctx := context.Background()
	client := NewLocalClient(newTestStore(t))

	project, err := client.CreateProject(ctx, graph.Project{
		Title: "local", Origin: "diff.patch", Goal: "prove", BootstrapEnabled: false,
	})
	if err != nil {
		t.Fatalf("CreateProject: %v", err)
	}
	if project.BootstrapEnabled {
		t.Fatalf("CreateProject did not preserve explicit BootstrapEnabled=false")
	}

	hint, err := client.AddHint(ctx, project.ID, "look at the upload handler", "analyst")
	if err != nil {
		t.Fatalf("AddHint: %v", err)
	}
	if hint.Kind != "hint" {
		t.Fatalf("AddHint kind = %q, want hint", hint.Kind)
	}

	intent, err := client.DeclareIntent(ctx, project.ID, graph.Node{
		Type: "gap", Content: "verify endpoint", From: []string{"origin"}, Creator: "worker-1",
	})
	if err != nil {
		t.Fatalf("DeclareIntent: %v", err)
	}
	if _, err := client.ClaimIntent(ctx, project.ID, intent.ID, "worker-1", time.Minute); err != nil {
		t.Fatalf("ClaimIntent: %v", err)
	}
	if _, err := client.ClaimIntent(ctx, project.ID, intent.ID, "worker-2", time.Minute); !errors.Is(err, graph.ErrConflict) {
		t.Fatalf("second ClaimIntent error = %v, want ErrConflict", err)
	}
	if _, err := client.ReleaseIntent(ctx, project.ID, intent.ID, "worker-1"); err != nil {
		t.Fatalf("ReleaseIntent: %v", err)
	}

	if _, err := client.ClaimReason(ctx, project.ID, "reason-1", "new_facts", time.Minute); err != nil {
		t.Fatalf("ClaimReason: %v", err)
	}
	if _, err := client.ClaimReason(ctx, project.ID, "reason-2", "new_facts", time.Minute); !errors.Is(err, graph.ErrConflict) {
		t.Fatalf("second ClaimReason error = %v, want ErrConflict", err)
	}
	if _, err := client.HeartbeatReason(ctx, project.ID, "reason-1", time.Minute); err != nil {
		t.Fatalf("HeartbeatReason: %v", err)
	}
	released, err := client.ReleaseReason(ctx, project.ID, "reason-1")
	if err != nil {
		t.Fatalf("ReleaseReason: %v", err)
	}
	if released.Reason != nil {
		t.Fatalf("ReleaseReason left reason lease: %+v", released.Reason)
	}
}
