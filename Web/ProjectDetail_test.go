package Web

import (
	"AIxVuln/LabManager"
	"AIxVuln/dockerManager"
	"AIxVuln/misc"
	"AIxVuln/taskManager"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"
)

// TestGetProjectReportsSandboxContainerID proves the graph-lane preflight
// signal: GET /projects/:name exposes the container id registered under the lab
// id, so an operator can confirm explore's taskManager.GetSandbox(labID)
// mapping before expecting sandbox-backed evidence.
func TestGetProjectReportsSandboxContainerID(t *testing.T) {
	gin.SetMode(gin.TestMode)

	originalDataDir := misc.GetConfigValueDefault("misc", "DATA_DIR", "./data")
	tempDataDir := t.TempDir()
	if err := misc.SetConfigValue("misc", "DATA_DIR", tempDataDir); err != nil {
		t.Fatalf("set temp data dir: %v", err)
	}
	t.Cleanup(func() {
		_ = misc.SetConfigValue("misc", "DATA_DIR", originalDataDir)
	})

	sourceDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(sourceDir, "seed.txt"), []byte("x"), 0o644); err != nil {
		t.Fatalf("write seed source file: %v", err)
	}

	server := NewServer()
	t.Cleanup(func() {
		if server.stateStore != nil {
			_ = server.stateStore.Close()
		}
	})
	pm, err := LabManager.NewLabManager(LabManager.LabConfig{
		LabName:       "graph-preflight",
		SourceCodeDir: sourceDir,
		TaskContent:   "preflight",
	})
	if err != nil {
		t.Fatalf("create lab manager: %v", err)
	}
	labID := pm.GetProjectID()
	server.pms[labID] = pm
	t.Cleanup(func() { taskManager.RemoveSandbox(labID) })

	readContainerID := func() string {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodGet, "/projects/"+labID, nil)
		c.Params = gin.Params{{Key: "name", Value: labID}}
		server.getProject(c)
		if w.Code != http.StatusOK {
			t.Fatalf("getProject status = %d, body=%s", w.Code, w.Body.String())
		}
		var body struct {
			Result struct {
				SandboxContainerID string `json:"sandboxContainerID"`
			} `json:"result"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
			t.Fatalf("decode response: %v; body=%s", err, w.Body.String())
		}
		return body.Result.SandboxContainerID
	}

	if got := readContainerID(); got != "" {
		t.Fatalf("sandboxContainerID before start = %q, want empty", got)
	}

	taskManager.SetSandbox(labID, &dockerManager.Sandbox{ContainerId: "container-abc"})
	if got := readContainerID(); got != "container-abc" {
		t.Fatalf("sandboxContainerID = %q, want container-abc", got)
	}
}
