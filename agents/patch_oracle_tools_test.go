package agents

import "testing"

// TestPatchOracleToolsRegistered guards the PatchOracle main-chain tools: they
// must be exposed to both the analyze and verifier agents, and adding them must
// not disturb the index-sensitive workflow factory blocks.
func TestPatchOracleToolsRegistered(t *testing.T) {
	required := []string{
		"IngestPatchTool",
		"InvertPatchTool",
		"QueryRuleBaseTool",
		"PromoteRuleTool",
	}
	for name, factories := range map[string][]ToolFactory{
		"analyze":  AnalyzeToolFactories(),
		"verifier": VerifierToolFactories(),
	} {
		registered := make(map[string]bool, len(factories))
		for _, factory := range factories {
			handler := factory(nil)
			if handler == nil {
				t.Fatalf("%s: factory returned nil handler", name)
			}
			registered[handler.Name()] = true
		}
		for _, tool := range required {
			if !registered[tool] {
				t.Errorf("%s factory list is missing %s", name, tool)
			}
		}
	}
}
