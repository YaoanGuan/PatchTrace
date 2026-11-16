package misc

import "testing"

func TestBuiltinMCPBindingsIncludeIDAJADXC2(t *testing.T) {
	specs := BuiltinMCPBindings()
	want := map[string]struct {
		name      string
		prefix    string
		agentType string
	}{
		DefaultIDAProMCPID: {
			name:      DefaultIDAProMCPName,
			prefix:    DefaultIDAProMCPToolPrefix,
			agentType: BinaryReverseAgentType,
		},
		DefaultJADXAIMCPID: {
			name:      DefaultJADXAIMCPName,
			prefix:    DefaultJADXAIMCPToolPrefix,
			agentType: BinaryAndroidAgentType,
		},
		DefaultC2MCPID: {
			name:      DefaultC2MCPName,
			prefix:    DefaultC2MCPToolPrefix,
			agentType: PentestPostExploitationAgentType,
		},
	}
	for _, spec := range specs {
		expected, ok := want[spec.ID]
		if !ok {
			continue
		}
		if spec.Name != expected.name {
			t.Fatalf("unexpected builtin mcp name for %s: %q", spec.ID, spec.Name)
		}
		if spec.ToolNamePrefix != expected.prefix {
			t.Fatalf("unexpected builtin mcp tool prefix for %s: %q", spec.ID, spec.ToolNamePrefix)
		}
		if spec.AgentType != expected.agentType {
			t.Fatalf("unexpected builtin mcp agent type for %s: %q", spec.ID, spec.AgentType)
		}
		delete(want, spec.ID)
	}
	if len(want) != 0 {
		t.Fatalf("builtin mcp bindings missing entries: %+v", want)
	}
}

func TestRequiredBuiltinMCPBindingsEmpty(t *testing.T) {
	specs := RequiredBuiltinMCPBindings()
	if len(specs) != 0 {
		t.Fatalf("required builtin mcp binding count mismatch: %d", len(specs))
	}
}

func TestNormalizeMCPConfigRowBuildsAgentBindingsFromLegacyAgentTypes(t *testing.T) {
	row := normalizeMCPConfigRow(MCPConfigRow{
		ID:         "demo",
		Name:       "demo",
		AgentTypes: []string{"Agent-B", "Agent-A", "Agent-A"},
	})
	if len(row.AgentBindings) != 2 {
		t.Fatalf("unexpected agent binding count: %d", len(row.AgentBindings))
	}
	if row.AgentBindings[0].AgentType != "Agent-A" || row.AgentBindings[1].AgentType != "Agent-B" {
		t.Fatalf("unexpected normalized bindings: %+v", row.AgentBindings)
	}
	if len(row.AgentTypes) != 2 || row.AgentTypes[0] != "Agent-A" || row.AgentTypes[1] != "Agent-B" {
		t.Fatalf("unexpected normalized agent types: %+v", row.AgentTypes)
	}
}

func TestNormalizeMCPConfigRowMergesDuplicateAgentBindings(t *testing.T) {
	row := normalizeMCPConfigRow(MCPConfigRow{
		ID:   "demo",
		Name: "demo",
		AgentBindings: []MCPAgentBinding{
			{AgentType: "Agent-A", ToolAllowlist: []string{"tool_a"}},
			{AgentType: "Agent-A", ToolAllowlist: []string{"tool_b"}, ToolDenylist: []string{"deny_a"}},
		},
	})
	if len(row.AgentBindings) != 1 {
		t.Fatalf("unexpected binding count: %d", len(row.AgentBindings))
	}
	binding := row.AgentBindings[0]
	if len(binding.ToolAllowlist) != 2 || binding.ToolAllowlist[0] != "tool_a" || binding.ToolAllowlist[1] != "tool_b" {
		t.Fatalf("unexpected merged allowlist: %+v", binding.ToolAllowlist)
	}
	if len(binding.ToolDenylist) != 1 || binding.ToolDenylist[0] != "deny_a" {
		t.Fatalf("unexpected merged denylist: %+v", binding.ToolDenylist)
	}
}

func TestNormalizeMCPConfigRowUpgradesLegacyPentestRemoteControlBinding(t *testing.T) {
	row := normalizeMCPConfigRow(MCPConfigRow{
		ID:   "demo",
		Name: "demo",
		AgentBindings: []MCPAgentBinding{
			{AgentType: LegacyPentestRemoteControlAgentType},
			{AgentType: PentestPostExploitationAgentType},
		},
	})
	if len(row.AgentBindings) != 1 {
		t.Fatalf("unexpected binding count after legacy upgrade: %d", len(row.AgentBindings))
	}
	if row.AgentBindings[0].AgentType != PentestPostExploitationAgentType {
		t.Fatalf("unexpected upgraded binding agent type: %q", row.AgentBindings[0].AgentType)
	}
	if len(row.AgentTypes) != 1 || row.AgentTypes[0] != PentestPostExploitationAgentType {
		t.Fatalf("unexpected upgraded agent types: %+v", row.AgentTypes)
	}
}
