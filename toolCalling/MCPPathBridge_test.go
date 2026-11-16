package toolCalling

import (
	"strings"
	"testing"
)

func TestRewriteMCPArgsMapsSandboxPathToHostPath(t *testing.T) {
	sourceRoot := "/tmp/aix-lab/源代码"
	args := map[string]interface{}{
		"apk_path": "aix-mcp://samples/甲壳虫ADB助手.apk",
		"workspace": map[string]interface{}{
			"project_dir": "aix-mcp://decoded",
		},
		"command": `frida -i "aix-mcp://samples/甲壳虫ADB助手.apk"`,
	}

	rewritten := rewriteMCPArgs(args, sourceRoot)

	if got := rewritten["apk_path"]; got != "/tmp/aix-lab/源代码/samples/甲壳虫ADB助手.apk" {
		t.Fatalf("apk_path rewrite mismatch: got %v", got)
	}
	workspace, _ := rewritten["workspace"].(map[string]interface{})
	if got := workspace["project_dir"]; got != "/tmp/aix-lab/源代码/decoded" {
		t.Fatalf("project_dir rewrite mismatch: got %v", got)
	}
	if got := rewritten["command"]; got != `frida -i "/tmp/aix-lab/源代码/samples/甲壳虫ADB助手.apk"` {
		t.Fatalf("command rewrite mismatch: got %v", got)
	}
}

func TestRewriteMCPToolResponseMapsHostPathBackToSandboxPath(t *testing.T) {
	sourceRoot := "/tmp/aix-lab/源代码"
	raw := Success(map[string]interface{}{
		"text":    "workspace ready at /tmp/aix-lab/源代码/samples/甲壳虫ADB助手.apk",
		"content": []interface{}{map[string]interface{}{"path": "/tmp/aix-lab/源代码/decoded/AndroidManifest.xml"}},
	})

	rewritten := rewriteMCPToolResponse(raw, sourceRoot)

	if strings.Contains(rewritten, `/tmp/aix-lab/源代码`) {
		t.Fatalf("host path should not remain in rewritten response: %s", rewritten)
	}
	if !strings.Contains(rewritten, "aix-mcp://samples/甲壳虫ADB助手.apk") {
		t.Fatalf("aix-mcp apk path missing from rewritten response: %s", rewritten)
	}
	if !strings.Contains(rewritten, "aix-mcp://decoded/AndroidManifest.xml") {
		t.Fatalf("aix-mcp manifest path missing from rewritten response: %s", rewritten)
	}
}
