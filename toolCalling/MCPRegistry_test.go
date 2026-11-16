package toolCalling

import (
	"AIxVuln/misc"
	"AIxVuln/taskManager"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestBuildAgentScopedMCPServerConfigUsesPerAgentFilters(t *testing.T) {
	row := misc.NormalizeMCPConfigForRuntime(misc.MCPConfigRow{
		ID:             "demo_mcp",
		Name:           "demo-mcp",
		Transport:      "stdio",
		Command:        "/bin/demo",
		ToolNamePrefix: "",
		ToolAllowlist:  []string{"common_tool", "shared_tool"},
		ToolDenylist:   []string{"global_deny"},
		AgentBindings: []misc.MCPAgentBinding{
			{
				AgentType:     "Agent-A",
				ToolAllowlist: []string{"agent_only_tool"},
				ToolDenylist:  []string{"agent_deny"},
			},
		},
	})

	cfg := buildAgentScopedMCPServerConfig(row, row.AgentBindings[0])

	if cfg.Name != "demo_mcp__Agent_A" {
		t.Fatalf("unexpected scoped runtime name: %q", cfg.Name)
	}
	if cfg.SharedConnectionName != "demo_mcp" {
		t.Fatalf("unexpected shared connection name: %q", cfg.SharedConnectionName)
	}
	if cfg.DisplayName != "demo-mcp" {
		t.Fatalf("unexpected display name: %q", cfg.DisplayName)
	}
	if cfg.ToolNamePrefix != "MCP_demo_mcp" {
		t.Fatalf("unexpected default tool prefix: %q", cfg.ToolNamePrefix)
	}
	if len(cfg.ToolAllowlist) != 1 || cfg.ToolAllowlist[0] != "agent_only_tool" {
		t.Fatalf("unexpected scoped allowlist: %+v", cfg.ToolAllowlist)
	}
	if len(cfg.ToolDenylist) != 2 || cfg.ToolDenylist[0] != "global_deny" || cfg.ToolDenylist[1] != "agent_deny" {
		t.Fatalf("unexpected scoped denylist: %+v", cfg.ToolDenylist)
	}
}

func TestMCPRegistryRuntimeForServerUsesProjectScope(t *testing.T) {
	r := &mcpRegistry{
		serverConfigs: make(map[string]MCPServerConfig),
		servers:       make(map[string]*mcpServerRuntime),
		sharedServers: make(map[string]*mcpSharedRuntime),
		profiles:      make(map[string]MCPProfileConfig),
		agentProfiles: make(map[string][]string),
		toolMeta:      make(map[string]MCPToolCatalogItem),
	}
	if !r.registerServer(MCPServerConfig{
		Name:           "demo_mcp__Agent_A",
		DisplayName:    "demo",
		Type:           "http",
		URL:            "http://127.0.0.1:9999/mcp",
		ToolNamePrefix: "MCP_demo_mcp",
	}) {
		t.Fatal("registerServer returned false")
	}

	projectA1 := r.runtimeForServer("demo_mcp__Agent_A", "project-a")
	projectA2 := r.runtimeForServer("demo_mcp__Agent_A", "project-a")
	projectB := r.runtimeForServer("demo_mcp__Agent_A", "project-b")
	global1 := r.runtimeForServer("demo_mcp__Agent_A", "")
	global2 := r.runtimeForServer("demo_mcp__Agent_A", "")

	if projectA1 == nil || projectA2 == nil || projectB == nil || global1 == nil || global2 == nil {
		t.Fatal("runtimeForServer returned nil runtime")
	}
	if projectA1 != projectA2 {
		t.Fatal("same project should reuse the same MCP runtime")
	}
	if projectA1 == projectB {
		t.Fatal("different projects should not share the same MCP runtime")
	}
	if global1 != global2 {
		t.Fatal("empty project scope should still reuse the same MCP runtime")
	}
	if global1 == projectA1 {
		t.Fatal("global runtime should not be reused for a project-scoped MCP runtime")
	}
	if projectA1.cfg.Name != "demo_mcp__Agent_A" || projectB.cfg.Name != "demo_mcp__Agent_A" {
		t.Fatal("project scoping should not change the advertised MCP server name")
	}
}

func TestMCPRegistryRuntimeForServerSharesConnectionAcrossAgentTypesWithinProject(t *testing.T) {
	r := &mcpRegistry{
		serverConfigs: make(map[string]MCPServerConfig),
		servers:       make(map[string]*mcpServerRuntime),
		sharedServers: make(map[string]*mcpSharedRuntime),
		profiles:      make(map[string]MCPProfileConfig),
		agentProfiles: make(map[string][]string),
		toolMeta:      make(map[string]MCPToolCatalogItem),
	}
	if !r.registerServer(MCPServerConfig{
		Name:                 "demo_mcp__DecisionBrain",
		SharedConnectionName: "demo_mcp",
		DisplayName:          "demo",
		Type:                 "http",
		URL:                  "http://127.0.0.1:9999/mcp",
		ToolNamePrefix:       "MCP_demo_mcp",
	}) {
		t.Fatal("registerServer for decision brain returned false")
	}
	if !r.registerServer(MCPServerConfig{
		Name:                 "demo_mcp__Agent_A",
		SharedConnectionName: "demo_mcp",
		DisplayName:          "demo",
		Type:                 "http",
		URL:                  "http://127.0.0.1:9999/mcp",
		ToolNamePrefix:       "MCP_demo_mcp",
	}) {
		t.Fatal("registerServer for agent returned false")
	}

	decisionRuntime := r.runtimeForServer("demo_mcp__DecisionBrain", "project-a")
	agentRuntime := r.runtimeForServer("demo_mcp__Agent_A", "project-a")
	otherProjectRuntime := r.runtimeForServer("demo_mcp__Agent_A", "project-b")

	if decisionRuntime == nil || agentRuntime == nil || otherProjectRuntime == nil {
		t.Fatal("expected runtimes for all lookups")
	}
	if decisionRuntime == agentRuntime {
		t.Fatal("different scoped runtimes should still be distinct runtime wrappers")
	}
	if decisionRuntime.shared == nil || agentRuntime.shared == nil || otherProjectRuntime.shared == nil {
		t.Fatal("expected all runtimes to resolve a shared MCP connection runtime")
	}
	if decisionRuntime.shared != agentRuntime.shared {
		t.Fatal("same project should share one MCP connection across agent types")
	}
	if agentRuntime.shared == otherProjectRuntime.shared {
		t.Fatal("different projects should not share the same MCP connection")
	}
}

func TestMCPRegistryRuntimeForServerUsesResolvedRuntimeProjectScope(t *testing.T) {
	taskManager.SetRuntimeAlias("lab_pentest_security", "lab")
	taskManager.SetRuntimeAlias("lab_binary_security", "lab")
	defer taskManager.SetRuntimeAlias("lab_pentest_security", "")
	defer taskManager.SetRuntimeAlias("lab_binary_security", "")

	r := &mcpRegistry{
		serverConfigs: make(map[string]MCPServerConfig),
		servers:       make(map[string]*mcpServerRuntime),
		sharedServers: make(map[string]*mcpSharedRuntime),
		profiles:      make(map[string]MCPProfileConfig),
		agentProfiles: make(map[string][]string),
		toolMeta:      make(map[string]MCPToolCatalogItem),
	}
	if !r.registerServer(MCPServerConfig{
		Name:                 "demo_mcp__DecisionBrain",
		SharedConnectionName: "demo_mcp",
		DisplayName:          "demo",
		Type:                 "http",
		URL:                  "http://127.0.0.1:9999/mcp",
		ToolNamePrefix:       "MCP_demo_mcp",
	}) {
		t.Fatal("registerServer returned false")
	}

	pentestRuntime := r.runtimeForServer("demo_mcp__DecisionBrain", "lab_pentest_security")
	binaryRuntime := r.runtimeForServer("demo_mcp__DecisionBrain", "lab_binary_security")
	labRuntime := r.runtimeForServer("demo_mcp__DecisionBrain", "lab")

	if pentestRuntime == nil || binaryRuntime == nil || labRuntime == nil {
		t.Fatal("expected runtimes for aliased project names")
	}
	if pentestRuntime != binaryRuntime || pentestRuntime != labRuntime {
		t.Fatal("project aliases resolving to the same runtime project should reuse the same MCP runtime")
	}
	if pentestRuntime.shared != labRuntime.shared {
		t.Fatal("resolved runtime project aliases should also share the same MCP connection")
	}
}

func TestBuildMCPHTTPClientKeepsPerClientCookieState(t *testing.T) {
	clientA := buildMCPHTTPClient(nil)
	clientB := buildMCPHTTPClient(map[string]interface{}{"Authorization": "Bearer demo"})

	if clientA == nil || clientA.Jar == nil {
		t.Fatal("expected MCP HTTP client without headers to still provide a cookie jar")
	}
	if clientB == nil || clientB.Jar == nil {
		t.Fatal("expected MCP HTTP client with headers to provide a cookie jar")
	}
	if clientB.Transport == nil {
		t.Fatal("expected MCP HTTP client with headers to inject request headers")
	}
	if clientA.Jar == clientB.Jar {
		t.Fatal("expected each MCP HTTP client to keep an isolated cookie jar")
	}
}

func TestBuildMCPTransportWithHTTPClientReusesExistingCookieJar(t *testing.T) {
	existing := buildMCPHTTPClient(map[string]interface{}{"Authorization": "Bearer demo"})
	if existing == nil || existing.Jar == nil {
		t.Fatal("expected existing client to provide a cookie jar")
	}

	transport, reused, err := buildMCPTransportWithHTTPClient(MCPServerConfig{
		Type: "http",
		URL:  "http://127.0.0.1:9999/mcp",
	}, existing)
	if err != nil {
		t.Fatalf("buildMCPTransportWithHTTPClient returned error: %v", err)
	}
	if reused != existing {
		t.Fatal("expected transport builder to reuse the provided HTTP client")
	}
	streamable, ok := transport.(*mcp.StreamableClientTransport)
	if !ok {
		t.Fatalf("expected streamable transport, got %T", transport)
	}
	if streamable.HTTPClient != existing {
		t.Fatal("expected streamable transport to keep the provided HTTP client")
	}
	if streamable.HTTPClient.Jar != existing.Jar {
		t.Fatal("expected cookie jar to remain unchanged across transport rebuilds")
	}
}
