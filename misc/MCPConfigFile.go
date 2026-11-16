package misc

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

type mcpConfigFilePayload struct {
	Configs []MCPConfigRow `json:"configs"`
}

var (
	runtimeMCPConfigStorePath   string
	runtimeMCPConfigStorePathMu sync.RWMutex
)

func DefaultMCPConfigStorePath() string {
	return normalizeMCPConfigStorePath("./mcp.json")
}

func DefaultMCPConfigFileCandidates() []string {
	dataDir := GetDataDir()
	return uniqueMCPConfigPaths([]string{
		"./mcp.json",
		filepath.Join(dataDir, "mcp.json"),
		filepath.Join(dataDir, "mcp-config.json"),
		filepath.Join(dataDir, "mcp-configs.json"),
		"./mcp-config.json",
		"./mcp-configs.json",
	})
}

func SetRuntimeMCPConfigStorePath(path string) {
	runtimeMCPConfigStorePathMu.Lock()
	defer runtimeMCPConfigStorePathMu.Unlock()
	runtimeMCPConfigStorePath = normalizeMCPConfigStorePath(path)
}

func ClearRuntimeMCPConfigStorePath() {
	runtimeMCPConfigStorePathMu.Lock()
	defer runtimeMCPConfigStorePathMu.Unlock()
	runtimeMCPConfigStorePath = ""
}

func GetRuntimeMCPConfigStorePath() string {
	runtimeMCPConfigStorePathMu.RLock()
	defer runtimeMCPConfigStorePathMu.RUnlock()
	return runtimeMCPConfigStorePath
}

func ResolveMCPConfigStorePath() string {
	if runtime := GetRuntimeMCPConfigStorePath(); runtime != "" {
		return runtime
	}
	return DefaultMCPConfigStorePath()
}

func LoadMCPConfigsFromFile(path string) error {
	path = normalizeMCPConfigStorePath(path)
	if path == "" {
		return fmt.Errorf("mcp config path is empty")
	}
	rows, err := readMCPConfigRowsFromFile(path)
	if err != nil {
		return err
	}
	SetRuntimeMCPConfigStorePath(path)
	return SaveAllMCPConfigs(rows)
}

func LoadMCPConfigsFromCandidateFiles(paths []string) (string, bool, error) {
	for _, rawPath := range uniqueMCPConfigPaths(paths) {
		if rawPath == "" {
			continue
		}
		if _, err := os.Stat(rawPath); err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return "", false, err
		}
		if err := LoadMCPConfigsFromFile(rawPath); err != nil {
			return rawPath, true, err
		}
		return rawPath, true, nil
	}
	return "", false, nil
}

func parseMCPConfigRows(data []byte) ([]MCPConfigRow, error) {
	var payload mcpConfigFilePayload
	if err := json.Unmarshal(data, &payload); err == nil && payload.Configs != nil {
		return payload.Configs, nil
	}
	var rows []MCPConfigRow
	if err := json.Unmarshal(data, &rows); err == nil && rows != nil {
		return rows, nil
	}
	return nil, fmt.Errorf("invalid MCP config file format: expected {\"configs\": [...]} or [...]")
}

func readMCPConfigRowsFromFile(path string) ([]MCPConfigRow, error) {
	path = normalizeMCPConfigStorePath(path)
	if path == "" {
		return nil, fmt.Errorf("mcp config path is empty")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return parseMCPConfigRows(data)
}

func writeMCPConfigRowsToFile(path string, rows []MCPConfigRow) error {
	path = normalizeMCPConfigStorePath(path)
	if path == "" {
		return fmt.Errorf("mcp config path is empty")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	payload := mcpConfigFilePayload{Configs: rows}
	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(path, data, 0644)
}

func normalizeMCPConfigStorePath(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	if abs, err := filepath.Abs(raw); err == nil {
		return filepath.Clean(abs)
	}
	return filepath.Clean(raw)
}

func uniqueMCPConfigPaths(paths []string) []string {
	if len(paths) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(paths))
	out := make([]string, 0, len(paths))
	for _, raw := range paths {
		path := normalizeMCPConfigStorePath(raw)
		if path == "" {
			continue
		}
		if _, ok := seen[path]; ok {
			continue
		}
		seen[path] = struct{}{}
		out = append(out, path)
	}
	return out
}
