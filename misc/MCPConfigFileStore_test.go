package misc

import (
	"os"
	"path/filepath"
	"testing"
)

func useTempMCPStorePath(t *testing.T) string {
	t.Helper()
	oldPath := GetRuntimeMCPConfigStorePath()
	storePath := filepath.Join(t.TempDir(), "mcp.json")
	SetRuntimeMCPConfigStorePath(storePath)
	t.Cleanup(func() {
		if oldPath != "" {
			SetRuntimeMCPConfigStorePath(oldPath)
			return
		}
		ClearRuntimeMCPConfigStorePath()
	})
	return storePath
}

func TestSaveAllMCPConfigsWritesResolvedStoreFile(t *testing.T) {
	storePath := useTempMCPStorePath(t)

	if err := SaveAllMCPConfigs([]MCPConfigRow{
		{
			ID:        "custom_stdio",
			Name:      "custom-stdio",
			Enabled:   true,
			Transport: "stdio",
			Command:   "/usr/bin/env",
			Args:      []string{"python3", "-V"},
			AgentTypes: []string{
				"Agent-General-GeneralCommonAgent",
			},
		},
	}); err != nil {
		t.Fatalf("SaveAllMCPConfigs: %v", err)
	}

	if _, err := os.Stat(storePath); err != nil {
		t.Fatalf("expected store file to exist: %v", err)
	}

	rows, err := readMCPConfigRowsFromFile(storePath)
	if err != nil {
		t.Fatalf("readMCPConfigRowsFromFile: %v", err)
	}
	ids := make(map[string]struct{}, len(rows))
	for _, row := range rows {
		ids[row.ID] = struct{}{}
	}
	for _, wantID := range []string{"custom_stdio", DefaultIDAProMCPID, DefaultJADXAIMCPID, DefaultC2MCPID} {
		if _, ok := ids[wantID]; !ok {
			t.Fatalf("expected %s to be persisted, got ids=%v", wantID, ids)
		}
	}
}

func TestGetMCPConfigsBootstrapsDefaultStoreFile(t *testing.T) {
	storePath := useTempMCPStorePath(t)

	rows := GetMCPConfigs()
	if len(rows) < 3 {
		t.Fatalf("expected builtin MCP configs, got %d", len(rows))
	}
	if _, err := os.Stat(storePath); err != nil {
		t.Fatalf("expected bootstrapped store file to exist: %v", err)
	}

	ids := make(map[string]struct{}, len(rows))
	for _, row := range rows {
		ids[row.ID] = struct{}{}
	}
	for _, wantID := range []string{DefaultIDAProMCPID, DefaultJADXAIMCPID, DefaultC2MCPID} {
		if _, ok := ids[wantID]; !ok {
			t.Fatalf("expected builtin MCP %s, got ids=%v", wantID, ids)
		}
	}
}
