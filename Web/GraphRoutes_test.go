package Web

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"AIxVuln/graph"
	"AIxVuln/graphserver"
	"AIxVuln/misc"

	"github.com/gin-gonic/gin"
)

// TestRegisterAPIRoutesMountsGraphAPI proves the optional graph protocol family
// can be mounted on the real Web gin engine without opening a port.
func TestRegisterAPIRoutesMountsGraphAPI(t *testing.T) {
	gin.SetMode(gin.TestMode)

	originalDataDir := misc.GetConfigValueDefault("misc", "DATA_DIR", "./data")
	tempDataDir := t.TempDir()
	if err := misc.SetConfigValue("misc", "DATA_DIR", tempDataDir); err != nil {
		t.Fatalf("set temp data dir: %v", err)
	}
	t.Cleanup(func() {
		_ = misc.SetConfigValue("misc", "DATA_DIR", originalDataDir)
	})

	store, err := graph.Open(filepath.Join(tempDataDir, "graph.db"))
	if err != nil {
		t.Fatalf("open graph store: %v", err)
	}
	defer store.Close()

	server := NewServer()
	t.Cleanup(func() {
		if server.stateStore != nil {
			_ = server.stateStore.Close()
		}
	})
	server.RegisterAPIRoutes(graphserver.New(store).Register)

	engine, ok := server.Handler(nil).(*gin.Engine)
	if !ok {
		t.Fatal("Handler did not return a *gin.Engine")
	}
	routes := make(map[string]struct{}, len(engine.Routes()))
	for _, route := range engine.Routes() {
		routes[route.Method+" "+route.Path] = struct{}{}
	}
	for _, want := range []string{
		"POST /api/graph/projects",
		"GET /api/graph/projects",
		"GET /api/graph/projects/:id",
		"POST /api/graph/projects/:id/hints",
		"POST /api/graph/projects/:id/intents",
		"POST /api/graph/projects/:id/complete",
		"GET /api/graph/projects/:id/export",
		"GET /api/graph/projects/:id/provenance",
	} {
		if _, found := routes[want]; !found {
			t.Fatalf("route %q is not mounted; got %v", want, routes)
		}
	}
}

// TestGraphAPIRoutesRequireBearerToken proves the optional graph family is
// mounted inside the same authenticated group as /projects and /config: an
// unauthenticated create/hint call must be rejected by tokenAuthMiddleware,
// while a token from the existing /login flow must pass the middleware.
func TestGraphAPIRoutesRequireBearerToken(t *testing.T) {
	gin.SetMode(gin.TestMode)

	originalDataDir := misc.GetConfigValueDefault("misc", "DATA_DIR", "./data")
	tempDataDir := t.TempDir()
	if err := misc.SetConfigValue("misc", "DATA_DIR", tempDataDir); err != nil {
		t.Fatalf("set temp data dir: %v", err)
	}
	t.Cleanup(func() {
		_ = misc.SetConfigValue("misc", "DATA_DIR", originalDataDir)
	})

	store, err := graph.Open(filepath.Join(tempDataDir, "graph.db"))
	if err != nil {
		t.Fatalf("open graph store: %v", err)
	}
	defer store.Close()

	server := NewServer()
	t.Cleanup(func() {
		if server.stateStore != nil {
			_ = server.stateStore.Close()
		}
	})
	server.RegisterAPIRoutes(graphserver.New(store).Register)
	handler := server.Handler(nil)

	token := generateToken("graph-api-test")
	cases := []struct {
		method string
		path   string
		body   string
	}{
		{http.MethodPost, "/api/graph/projects", `{"id":"auth-probe","title":"probe","goal":"probe"}`},
		{http.MethodGet, "/api/graph/projects", ""},
		{http.MethodPost, "/api/graph/projects/auth-probe/hints", `{"content":"hint","creator":"test"}`},
	}
	for _, tc := range cases {
		req := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
		if tc.body != "" {
			req.Header.Set("Content-Type", "application/json")
		}
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("%s %s without a token = %d, want 401; body=%s", tc.method, tc.path, rec.Code, rec.Body.String())
		}

		req = httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
		if tc.body != "" {
			req.Header.Set("Content-Type", "application/json")
		}
		req.Header.Set("Authorization", "Bearer "+token)
		rec = httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code == http.StatusUnauthorized {
			t.Fatalf("%s %s with a valid bearer token was rejected; body=%s", tc.method, tc.path, rec.Body.String())
		}
	}
}
