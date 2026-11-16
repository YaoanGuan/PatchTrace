package Web

import (
	"AIxVuln/LabManager"
	"AIxVuln/misc"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestDelProjectRemovesManagedProjectDirectory(t *testing.T) {
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
	if err := os.WriteFile(filepath.Join(sourceDir, "seed.txt"), []byte("x"), 0644); err != nil {
		t.Fatalf("write seed source file: %v", err)
	}

	server := NewServer()
	t.Cleanup(func() {
		if server.stateStore != nil {
			_ = server.stateStore.Close()
		}
	})

	pm, err := LabManager.NewLabManager(LabManager.LabConfig{
		LabName:       "delete-me",
		SourceCodeDir: sourceDir,
		TaskContent:   "test task",
	})
	if err != nil {
		t.Fatalf("create lab manager: %v", err)
	}
	server.pms[pm.GetProjectID()] = pm

	projectDir := filepath.Join(tempDataDir, "projects", pm.GetProjectID())
	if _, err := os.Stat(projectDir); err != nil {
		t.Fatalf("project dir should exist before deletion: %v", err)
	}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/projects/"+pm.GetProjectID()+"/del", nil)
	c.Params = gin.Params{{Key: "name", Value: pm.GetProjectID()}}

	server.delProject(c)

	if w.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d body=%s", w.Code, w.Body.String())
	}
	if _, exists := server.pms[pm.GetProjectID()]; exists {
		t.Fatal("project should be removed from runtime map")
	}
	if _, err := os.Stat(projectDir); !os.IsNotExist(err) {
		t.Fatalf("project dir should be removed, stat err=%v", err)
	}
	if server.stateStore != nil {
		projects, err := server.stateStore.LoadAll()
		if err != nil {
			t.Fatalf("load persisted projects: %v", err)
		}
		if len(projects) != 0 {
			t.Fatalf("expected no persisted projects after deletion, got %d", len(projects))
		}
	}
}
