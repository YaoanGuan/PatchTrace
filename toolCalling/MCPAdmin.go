package toolCalling

import (
	"AIxVuln/misc"
	"fmt"
	"sort"
	"strings"
)

type MCPTestResult struct {
	OK          bool     `json:"ok"`
	Message     string   `json:"message"`
	ToolCount   int      `json:"tool_count"`
	ToolNames   []string `json:"tool_names,omitempty"`
	ServerName  string   `json:"server_name"`
	Transport   string   `json:"transport"`
	DisplayName string   `json:"display_name,omitempty"`
}

func TestMCPConfig(row misc.MCPConfigRow) MCPTestResult {
	row = misc.NormalizeMCPConfigForRuntime(row)
	cfg := MCPServerConfig{
		Name:              row.ID,
		DisplayName:       row.Name,
		Type:              row.Transport,
		Command:           row.Command,
		Args:              append([]string(nil), row.Args...),
		WorkDir:           row.WorkDir,
		URL:               row.URL,
		ToolNamePrefix:    row.ToolNamePrefix,
		PreserveSchema:    row.PreserveSchema,
		ConnectTimeoutSec: row.ConnectTimeoutSec,
		CallTimeoutSec:    row.CallTimeoutSec,
	}
	if len(row.Env) > 0 {
		cfg.Env = make(map[string]interface{}, len(row.Env))
		for k, v := range row.Env {
			cfg.Env[k] = v
		}
	}
	if len(row.Headers) > 0 {
		cfg.Headers = make(map[string]interface{}, len(row.Headers))
		for k, v := range row.Headers {
			cfg.Headers[k] = v
		}
	}
	cfg.ToolAllowlist = append([]string(nil), row.ToolAllowlist...)
	cfg.ToolDenylist = append([]string(nil), row.ToolDenylist...)

	handlers, _, err := connectMCPServer(cfg, map[string]struct{}{})
	result := MCPTestResult{
		ServerName:  cfg.Name,
		Transport:   cfg.Type,
		DisplayName: row.Name,
	}
	if err != nil {
		result.Message = err.Error()
		return result
	}
	if len(handlers) > 0 {
		if h, ok := handlers[0].(*mcpToolHandler); ok && h.server != nil && h.server.session != nil {
			defer h.server.session.Close()
		}
	}
	if len(handlers) == 0 {
		result.OK = true
		result.Message = "Connected, but no tools were exposed by the current filters."
		return result
	}
	result.OK = true
	result.ToolCount = len(handlers)
	result.ToolNames = make([]string, 0, len(handlers))
	for _, h := range handlers {
		if h == nil {
			continue
		}
		result.ToolNames = append(result.ToolNames, h.Name())
	}
	sort.Strings(result.ToolNames)
	result.Message = fmt.Sprintf("Connected successfully and discovered %d tool(s).", len(result.ToolNames))
	return result
}

func TestMCPConfigForAgent(row misc.MCPConfigRow, agentType string) MCPTestResult {
	row = misc.NormalizeMCPConfigForRuntime(row)
	agentType = strings.TrimSpace(agentType)
	result := MCPTestResult{
		ServerName:  row.ID,
		Transport:   row.Transport,
		DisplayName: row.Name,
	}
	if agentType == "" {
		result.Message = "agent type is required"
		return result
	}
	var binding *misc.MCPAgentBinding
	for i := range row.AgentBindings {
		if strings.TrimSpace(row.AgentBindings[i].AgentType) == agentType {
			binding = &row.AgentBindings[i]
			break
		}
	}
	if binding == nil {
		result.Message = fmt.Sprintf("agent binding not found for %s", agentType)
		return result
	}
	cfg := buildAgentScopedMCPServerConfig(row, *binding)
	handlers, _, err := connectMCPServer(cfg, map[string]struct{}{})
	if err != nil {
		result.Message = err.Error()
		return result
	}
	if len(handlers) > 0 {
		if h, ok := handlers[0].(*mcpToolHandler); ok && h.server != nil && h.server.session != nil {
			defer h.server.session.Close()
		}
	}
	if len(handlers) == 0 {
		result.OK = true
		result.Message = "Connected, but no tools were exposed by the current agent binding filters."
		return result
	}
	result.OK = true
	result.ToolCount = len(handlers)
	result.ToolNames = make([]string, 0, len(handlers))
	for _, h := range handlers {
		if h == nil {
			continue
		}
		result.ToolNames = append(result.ToolNames, h.Name())
	}
	sort.Strings(result.ToolNames)
	result.Message = fmt.Sprintf("Connected successfully and discovered %d tool(s) for %s.", len(result.ToolNames), agentType)
	return result
}

func AgentTypesBoundToAnyMCP() []string {
	rows := misc.GetMCPConfigs()
	seen := make(map[string]struct{})
	out := make([]string, 0)
	for _, row := range rows {
		for _, agentType := range row.AgentTypes {
			agentType = strings.TrimSpace(agentType)
			if agentType == "" {
				continue
			}
			if _, ok := seen[agentType]; ok {
				continue
			}
			seen[agentType] = struct{}{}
			out = append(out, agentType)
		}
	}
	sort.Strings(out)
	return out
}

func validateRequiredMCPRequirement(spec misc.RequiredMCPBindingSpec) error {
	row, ok := misc.GetRequiredBuiltinMCPConfig(spec.ID, spec.Name)
	if !ok || row == nil {
		return fmt.Errorf("请先在 MCP 设置中填写必需的 %s 配置，并测试通过后再启动实验室", spec.Name)
	}
	if !row.Enabled {
		return fmt.Errorf("必需的 %s 当前未启用，请启用并测试通过后再启动实验室", spec.Name)
	}
	switch row.Transport {
	case "stdio":
		if strings.TrimSpace(row.Command) == "" {
			return fmt.Errorf("必需的 %s 缺少 Command，请先填写并测试通过", spec.Name)
		}
	case "streamable", "sse":
		if strings.TrimSpace(row.URL) == "" {
			return fmt.Errorf("必需的 %s 缺少 URL，请先填写并测试通过", spec.Name)
		}
	default:
		return fmt.Errorf("必需的 %s transport 无效: %s", spec.Name, row.Transport)
	}
	bound := false
	for _, binding := range row.AgentBindings {
		if strings.TrimSpace(binding.AgentType) == spec.AgentType {
			bound = true
			break
		}
	}
	if !bound {
		return fmt.Errorf("必需的 %s 必须绑定到 %s", spec.Name, spec.AgentType)
	}
	result := TestMCPConfigForAgent(*row, spec.AgentType)
	if !result.OK {
		msg := strings.TrimSpace(result.Message)
		if msg == "" {
			msg = "未知错误"
		}
		return fmt.Errorf("%s 连接测试未通过: %s", spec.Name, msg)
	}
	if result.ToolCount <= 0 {
		return fmt.Errorf("%s 已连接但未暴露任何工具，请检查服务端或过滤配置", spec.Name)
	}
	return nil
}

func ValidateRequiredMCPRequirements() error {
	for _, spec := range misc.RequiredBuiltinMCPBindings() {
		if err := validateRequiredMCPRequirement(spec); err != nil {
			return err
		}
	}
	return nil
}

func ValidateIDAProMCPRequirement() error {
	for _, spec := range misc.BuiltinMCPBindings() {
		if spec.ID == misc.DefaultIDAProMCPID {
			return validateRequiredMCPRequirement(spec)
		}
	}
	return nil
}

func ValidateJADXAIMCPRequirement() error {
	for _, spec := range misc.BuiltinMCPBindings() {
		if spec.ID == misc.DefaultJADXAIMCPID {
			return validateRequiredMCPRequirement(spec)
		}
	}
	return nil
}

func ValidateC2MCPRequirement() error {
	for _, spec := range misc.BuiltinMCPBindings() {
		if spec.ID == misc.DefaultC2MCPID {
			return validateRequiredMCPRequirement(spec)
		}
	}
	return nil
}
