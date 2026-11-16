package Web

import (
	"path/filepath"
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
