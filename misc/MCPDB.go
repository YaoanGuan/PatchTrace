package misc

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/google/uuid"
)

const (
	DefaultGitHubCopilotMCPID           = "github_copilot_mcp"
	DefaultGitHubCopilotMCPName         = "GitHub Copilot MCP"
	DefaultGitHubCopilotMCPURL          = "https://api.githubcopilot.com/mcp/"
	DefaultGitHubCopilotMCPToolPrefix   = "MCP_GitHubCopilot"
	DefaultIDAProMCPID                  = "ida_pro_mcp"
	DefaultIDAProMCPName                = "ida-pro-mcp"
	DefaultIDAProMCPToolPrefix          = "MCP_IDAPro"
	DefaultJADXAIMCPID                  = "jadx_ai_mcp"
	DefaultJADXAIMCPName                = "jadx-ai-mcp"
	DefaultJADXAIMCPToolPrefix          = "MCP_JADXAI"
	DefaultC2MCPID                      = "c2_mcp"
	DefaultC2MCPName                    = "c2-mcp"
	DefaultC2MCPToolPrefix              = "MCP_C2"
	BinaryReverseAgentType              = "Agent-Binary-ReverseCommonAgent"
	BinaryAndroidAgentType              = "Agent-Binary-AndroidCommonAgent"
	PentestPostExploitationAgentType    = "Agent-Pentest-PostExploitationCommonAgent"
	PentestRemoteControlAgentType       = PentestPostExploitationAgentType
	LegacyPentestRemoteControlAgentType = "Agent-Pentest-RemoteControlCommonAgent"
)

type RequiredMCPBindingSpec struct {
	ID             string
	Name           string
	ToolNamePrefix string
	AgentType      string
}

func BuiltinMCPBindings() []RequiredMCPBindingSpec {
	return []RequiredMCPBindingSpec{
		{
			ID:             DefaultIDAProMCPID,
			Name:           DefaultIDAProMCPName,
			ToolNamePrefix: DefaultIDAProMCPToolPrefix,
			AgentType:      BinaryReverseAgentType,
		},
		{
			ID:             DefaultJADXAIMCPID,
			Name:           DefaultJADXAIMCPName,
			ToolNamePrefix: DefaultJADXAIMCPToolPrefix,
			AgentType:      BinaryAndroidAgentType,
		},
		{
			ID:             DefaultC2MCPID,
			Name:           DefaultC2MCPName,
			ToolNamePrefix: DefaultC2MCPToolPrefix,
			AgentType:      PentestPostExploitationAgentType,
		},
	}
}

func RequiredBuiltinMCPBindings() []RequiredMCPBindingSpec {
	return nil
}

type MCPConfigRow struct {
	ID                string            `json:"id"`
	Name              string            `json:"name"`
	Enabled           bool              `json:"enabled"`
	Transport         string            `json:"transport"`
	Command           string            `json:"command,omitempty"`
	Args              []string          `json:"args,omitempty"`
	Env               map[string]string `json:"env,omitempty"`
	WorkDir           string            `json:"workdir,omitempty"`
	URL               string            `json:"url,omitempty"`
	Headers           map[string]string `json:"headers,omitempty"`
	ToolAllowlist     []string          `json:"tool_allowlist,omitempty"`
	ToolDenylist      []string          `json:"tool_denylist,omitempty"`
	ToolNamePrefix    string            `json:"tool_name_prefix,omitempty"`
	PreserveSchema    bool              `json:"preserve_schema,omitempty"`
	ConnectTimeoutSec int               `json:"connect_timeout_sec,omitempty"`
	CallTimeoutSec    int               `json:"call_timeout_sec,omitempty"`
	AgentTypes        []string          `json:"agent_types,omitempty"`
	AgentBindings     []MCPAgentBinding `json:"agent_bindings,omitempty"`
}

type MCPAgentBinding struct {
	AgentType     string   `json:"agent_type"`
	ToolAllowlist []string `json:"tool_allowlist,omitempty"`
	ToolDenylist  []string `json:"tool_denylist,omitempty"`
}

var mcpConfigStoreMu sync.Mutex

func GetMCPConfigs() []MCPConfigRow {
	mcpConfigStoreMu.Lock()
	defer mcpConfigStoreMu.Unlock()

	path := ResolveMCPConfigStorePath()
	rows, err := readMCPConfigRowsFromFile(path)
	if err == nil {
		return finalizeMCPConfigRows(rows)
	}
	if !os.IsNotExist(err) {
		Warn("MCP", "读取 MCP 配置文件失败，回退到内置默认配置: "+err.Error(), nil)
		return finalizeMCPConfigRows(nil)
	}

	rows = bootstrapMCPConfigRows(path)
	if writeErr := writeMCPConfigRowsToFile(path, rows); writeErr != nil {
		Warn("MCP", "初始化 MCP 配置文件失败: "+writeErr.Error(), nil)
	}
	return rows
}

func SaveAllMCPConfigs(list []MCPConfigRow) error {
	mcpConfigStoreMu.Lock()
	defer mcpConfigStoreMu.Unlock()

	rows := finalizeMCPConfigRows(list)
	return writeMCPConfigRowsToFile(ResolveMCPConfigStorePath(), rows)
}

func bootstrapMCPConfigRows(storePath string) []MCPConfigRow {
	if shouldAttemptLegacyMCPMigration(storePath) {
		if rows, ok := loadLegacyMCPConfigsFromFiles(); ok {
			return finalizeMCPConfigRows(rows)
		}
		if rows, ok := loadLegacyMCPConfigsFromSQLite(); ok {
			return finalizeMCPConfigRows(rows)
		}
	}
	return finalizeMCPConfigRows(seedDefaultMCPConfigs())
}

func shouldAttemptLegacyMCPMigration(storePath string) bool {
	if GetRuntimeMCPConfigStorePath() != "" {
		return false
	}
	return normalizeMCPConfigStorePath(storePath) == DefaultMCPConfigStorePath()
}

func loadLegacyMCPConfigsFromFiles() ([]MCPConfigRow, bool) {
	for _, path := range uniqueMCPConfigPaths([]string{
		filepathJoinSafe(GetDataDir(), "mcp-config.json"),
		filepathJoinSafe(GetDataDir(), "mcp-configs.json"),
		"./mcp-config.json",
		"./mcp-configs.json",
	}) {
		rows, err := readMCPConfigRowsFromFile(path)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			Warn("MCP", "读取旧版 MCP 配置文件失败，已跳过: "+err.Error(), nil)
			continue
		}
		Info("MCP", "已从旧版 MCP 配置文件迁移: "+path, nil)
		return rows, true
	}
	return nil, false
}

func loadLegacyMCPConfigsFromSQLite() ([]MCPConfigRow, bool) {
	initConfigDB()
	var tableName string
	if err := configDB.QueryRow(`SELECT name FROM sqlite_master WHERE type = 'table' AND name = 'mcp_config'`).Scan(&tableName); err != nil {
		return nil, false
	}

	rows, err := configDB.Query(`SELECT id, name, enabled, transport, command, args_json, env_json, workdir, url, headers_json, tool_allowlist_json, tool_denylist_json, tool_name_prefix, preserve_schema, connect_timeout_sec, call_timeout_sec, agent_types_json, agent_bindings_json FROM mcp_config ORDER BY name, id`)
	if err != nil {
		Warn("MCP", "读取旧版 SQLite MCP 配置失败，已跳过: "+err.Error(), nil)
		return nil, false
	}
	defer rows.Close()

	var result []MCPConfigRow
	for rows.Next() {
		var (
			row               MCPConfigRow
			enabledInt        int
			preserveSchemaInt int
			argsJSON          string
			envJSON           string
			headersJSON       string
			toolAllowlistJSON string
			toolDenylistJSON  string
			agentTypesJSON    string
			agentBindingsJSON string
		)
		if err := rows.Scan(&row.ID, &row.Name, &enabledInt, &row.Transport, &row.Command, &argsJSON, &envJSON, &row.WorkDir, &row.URL, &headersJSON, &toolAllowlistJSON, &toolDenylistJSON, &row.ToolNamePrefix, &preserveSchemaInt, &row.ConnectTimeoutSec, &row.CallTimeoutSec, &agentTypesJSON, &agentBindingsJSON); err != nil {
			continue
		}
		row.Enabled = enabledInt != 0
		row.PreserveSchema = preserveSchemaInt != 0
		_ = json.Unmarshal([]byte(argsJSON), &row.Args)
		_ = json.Unmarshal([]byte(envJSON), &row.Env)
		_ = json.Unmarshal([]byte(headersJSON), &row.Headers)
		_ = json.Unmarshal([]byte(toolAllowlistJSON), &row.ToolAllowlist)
		_ = json.Unmarshal([]byte(toolDenylistJSON), &row.ToolDenylist)
		_ = json.Unmarshal([]byte(agentTypesJSON), &row.AgentTypes)
		_ = json.Unmarshal([]byte(agentBindingsJSON), &row.AgentBindings)
		result = append(result, row)
	}
	if len(result) == 0 {
		return nil, false
	}
	Info("MCP", "已从旧版 SQLite MCP 配置迁移", nil)
	return result, true
}

func seedDefaultMCPConfigs() []MCPConfigRow {
	result := make([]MCPConfigRow, 0, 1)
	auth := strings.TrimSpace(dbGet("misc", "GITHUB_COPILOT_MCP_AUTHORIZATION"))
	if auth != "" {
		result = append(result, MCPConfigRow{
			ID:        DefaultGitHubCopilotMCPID,
			Name:      DefaultGitHubCopilotMCPName,
			Enabled:   true,
			Transport: "streamable",
			URL:       DefaultGitHubCopilotMCPURL,
			Headers: map[string]string{
				"Authorization": auth,
			},
			ToolNamePrefix: DefaultGitHubCopilotMCPToolPrefix,
			AgentTypes:     []string{"Agent-General-GeneralCommonAgent"},
		})
		_ = dbDelete("misc", "GITHUB_COPILOT_MCP_AUTHORIZATION")
	}
	return result
}

func finalizeMCPConfigRows(list []MCPConfigRow) []MCPConfigRow {
	merged := make(map[string]MCPConfigRow, len(list)+len(BuiltinMCPBindings()))
	for _, raw := range list {
		row := normalizeMCPConfigRow(raw)
		merged[row.ID] = row
	}
	for _, spec := range BuiltinMCPBindings() {
		if _, ok := merged[spec.ID]; ok {
			continue
		}
		merged[spec.ID] = normalizeMCPConfigRow(MCPConfigRow{
			ID:             spec.ID,
			Name:           spec.Name,
			Enabled:        false,
			Transport:      "stdio",
			ToolNamePrefix: spec.ToolNamePrefix,
			AgentTypes:     []string{spec.AgentType},
		})
	}
	out := make([]MCPConfigRow, 0, len(merged))
	for _, row := range merged {
		out = append(out, normalizeMCPConfigRow(row))
	}
	sort.Slice(out, func(i, j int) bool {
		leftName := strings.ToLower(strings.TrimSpace(out[i].Name))
		rightName := strings.ToLower(strings.TrimSpace(out[j].Name))
		if leftName == rightName {
			return out[i].ID < out[j].ID
		}
		return leftName < rightName
	})
	return out
}

func filepathJoinSafe(elem ...string) string {
	if len(elem) == 0 {
		return ""
	}
	return strings.TrimSpace(filepath.Join(elem...))
}

func NormalizeMCPConfigForRuntime(row MCPConfigRow) MCPConfigRow {
	return normalizeMCPConfigRow(row)
}

func GetRequiredIDAProMCPConfig() (*MCPConfigRow, bool) {
	return GetBuiltinMCPConfig(DefaultIDAProMCPID, DefaultIDAProMCPName)
}

func GetRequiredJADXAIMCPConfig() (*MCPConfigRow, bool) {
	return GetBuiltinMCPConfig(DefaultJADXAIMCPID, DefaultJADXAIMCPName)
}

func GetRequiredC2MCPConfig() (*MCPConfigRow, bool) {
	return GetBuiltinMCPConfig(DefaultC2MCPID, DefaultC2MCPName)
}

func GetBuiltinMCPConfig(id string, name string) (*MCPConfigRow, bool) {
	rows := GetMCPConfigs()
	for _, row := range rows {
		if row.ID == id {
			copyRow := row
			return &copyRow, true
		}
	}
	for _, row := range rows {
		if strings.EqualFold(strings.TrimSpace(row.Name), strings.TrimSpace(name)) {
			copyRow := row
			return &copyRow, true
		}
	}
	return nil, false
}

func GetRequiredBuiltinMCPConfig(id string, name string) (*MCPConfigRow, bool) {
	return GetBuiltinMCPConfig(id, name)
}

func normalizeMCPConfigRow(row MCPConfigRow) MCPConfigRow {
	row.ID = strings.TrimSpace(row.ID)
	if row.ID == "" {
		row.ID = uuid.New().String()
	}
	row.Name = strings.TrimSpace(row.Name)
	if row.Name == "" {
		row.Name = row.ID
	}
	row.Transport = strings.ToLower(strings.TrimSpace(row.Transport))
	if row.Transport == "http" {
		row.Transport = "streamable"
	}
	switch row.Transport {
	case "streamable", "stdio", "sse":
	default:
		row.Transport = "stdio"
	}
	row.Command = strings.TrimSpace(row.Command)
	row.WorkDir = strings.TrimSpace(row.WorkDir)
	row.URL = strings.TrimSpace(row.URL)
	row.ToolNamePrefix = strings.TrimSpace(row.ToolNamePrefix)
	row.Args = normalizeOrderedStringList(row.Args)
	row.ToolAllowlist = normalizeStringList(row.ToolAllowlist)
	row.ToolDenylist = normalizeStringList(row.ToolDenylist)
	row.AgentBindings = normalizeMCPAgentBindings(row.AgentBindings, row.AgentTypes)
	row.AgentTypes = agentTypesFromBindings(row.AgentBindings)
	row.Env = normalizeStringMap(row.Env)
	row.Headers = normalizeStringMap(row.Headers)
	if row.ConnectTimeoutSec <= 0 {
		row.ConnectTimeoutSec = 20
	}
	if row.CallTimeoutSec <= 0 {
		row.CallTimeoutSec = 120
	}
	return row
}

func normalizeMCPAgentType(agentType string) string {
	switch strings.TrimSpace(agentType) {
	case LegacyPentestRemoteControlAgentType, PentestPostExploitationAgentType:
		return PentestPostExploitationAgentType
	default:
		return strings.TrimSpace(agentType)
	}
}

func normalizeMCPAgentBindings(bindings []MCPAgentBinding, legacyAgentTypes []string) []MCPAgentBinding {
	merged := make(map[string]MCPAgentBinding)
	for _, binding := range bindings {
		agentType := normalizeMCPAgentType(binding.AgentType)
		if agentType == "" {
			continue
		}
		current := merged[agentType]
		current.AgentType = agentType
		current.ToolAllowlist = mergeStringLists(current.ToolAllowlist, binding.ToolAllowlist)
		current.ToolDenylist = mergeStringLists(current.ToolDenylist, binding.ToolDenylist)
		merged[agentType] = current
	}
	for _, rawAgentType := range normalizeStringList(legacyAgentTypes) {
		agentType := normalizeMCPAgentType(rawAgentType)
		if _, ok := merged[agentType]; ok {
			continue
		}
		merged[agentType] = MCPAgentBinding{AgentType: agentType}
	}
	if len(merged) == 0 {
		return nil
	}
	keys := make([]string, 0, len(merged))
	for agentType := range merged {
		keys = append(keys, agentType)
	}
	sort.Strings(keys)
	out := make([]MCPAgentBinding, 0, len(keys))
	for _, agentType := range keys {
		item := merged[agentType]
		item.ToolAllowlist = normalizeStringList(item.ToolAllowlist)
		item.ToolDenylist = normalizeStringList(item.ToolDenylist)
		out = append(out, item)
	}
	return out
}

func agentTypesFromBindings(bindings []MCPAgentBinding) []string {
	if len(bindings) == 0 {
		return nil
	}
	out := make([]string, 0, len(bindings))
	for _, binding := range bindings {
		agentType := strings.TrimSpace(binding.AgentType)
		if agentType == "" {
			continue
		}
		out = append(out, agentType)
	}
	return normalizeStringList(out)
}

func mergeStringLists(left []string, right []string) []string {
	combined := make([]string, 0, len(left)+len(right))
	combined = append(combined, left...)
	combined = append(combined, right...)
	return normalizeStringList(combined)
}

func normalizeStringList(items []string) []string {
	if len(items) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(items))
	out := make([]string, 0, len(items))
	for _, item := range items {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		if _, ok := seen[item]; ok {
			continue
		}
		seen[item] = struct{}{}
		out = append(out, item)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func normalizeOrderedStringList(items []string) []string {
	if len(items) == 0 {
		return nil
	}
	out := make([]string, 0, len(items))
	for _, item := range items {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		out = append(out, item)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func normalizeStringMap(in map[string]string) map[string]string {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]string, len(in))
	for k, v := range in {
		key := strings.TrimSpace(k)
		if key == "" {
			continue
		}
		out[key] = strings.TrimSpace(v)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}
