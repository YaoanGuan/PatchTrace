package toolCalling

import (
	"AIxVuln/misc"
	"strings"
	"testing"
)

func TestBuildAgentToolsCatalogPayloadUsesRuntimeBaseURLFirst(t *testing.T) {
	oldConfig := misc.GetConfigValueDefault("misc", "AGENT_TOOLS_BASE_URL", "")
	oldRuntime := misc.GetRuntimeServerBaseURL()
	if err := misc.SetConfigValue("misc", "AGENT_TOOLS_BASE_URL", "http://10.0.0.8:9999/"); err != nil {
		t.Fatalf("SetConfigValue: %v", err)
	}
	misc.SetRuntimeServerBaseURL("http://192.168.1.23:43210")
	t.Cleanup(func() {
		_ = misc.SetConfigValue("misc", "AGENT_TOOLS_BASE_URL", oldConfig)
		misc.SetRuntimeServerBaseURL(oldRuntime)
	})

	payload, err := buildAgentToolsCatalogPayload(agentToolsCatalogQuery{})
	if err != nil {
		t.Fatalf("buildAgentToolsCatalogPayload: %v", err)
	}
	baseURL, _ := payload["base_url"].(string)
	if baseURL != "http://192.168.1.23:43210" {
		t.Fatalf("expected runtime base_url, got %q", baseURL)
	}
	items, ok := payload["items"].([]map[string]interface{})
	if !ok || len(items) == 0 {
		t.Fatalf("expected agent tool items, got %#v", payload["items"])
	}
	downloadURL, _ := items[0]["download_url"].(string)
	if !strings.Contains(downloadURL, "192.168.1.23:43210/agent_tools/") {
		t.Fatalf("expected runtime absolute download url, got %q", downloadURL)
	}
	if _, exists := items[0]["curl_example"]; exists {
		t.Fatal("curl_example should be omitted by default to reduce context size")
	}
}

func TestBuildAgentToolsCatalogPayloadFallsBackToConfiguredBaseURL(t *testing.T) {
	oldConfig := misc.GetConfigValueDefault("misc", "AGENT_TOOLS_BASE_URL", "")
	oldRuntime := misc.GetRuntimeServerBaseURL()
	if err := misc.SetConfigValue("misc", "AGENT_TOOLS_BASE_URL", "http://10.0.0.8:9999/"); err != nil {
		t.Fatalf("SetConfigValue: %v", err)
	}
	misc.SetRuntimeServerBaseURL("")
	t.Cleanup(func() {
		_ = misc.SetConfigValue("misc", "AGENT_TOOLS_BASE_URL", oldConfig)
		misc.SetRuntimeServerBaseURL(oldRuntime)
	})

	payload, err := buildAgentToolsCatalogPayload(agentToolsCatalogQuery{})
	if err != nil {
		t.Fatalf("buildAgentToolsCatalogPayload: %v", err)
	}
	baseURL, _ := payload["base_url"].(string)
	if baseURL != "http://10.0.0.8:9999" {
		t.Fatalf("expected configured base_url fallback, got %q", baseURL)
	}
}

func TestBuildAgentToolsCatalogPayloadSupportsFilters(t *testing.T) {
	payload, err := buildAgentToolsCatalogPayload(agentToolsCatalogQuery{
		ToolID:          "libgadget",
		IncludeExamples: true,
	})
	if err != nil {
		t.Fatalf("buildAgentToolsCatalogPayload: %v", err)
	}
	items, ok := payload["items"].([]map[string]interface{})
	if !ok {
		t.Fatalf("unexpected items payload: %#v", payload["items"])
	}
	if len(items) != 1 {
		t.Fatalf("expected exactly one filtered item, got %d", len(items))
	}
	if items[0]["id"] != "libgadget" {
		t.Fatalf("unexpected filtered item id: %#v", items[0]["id"])
	}
	if _, exists := items[0]["curl_example"]; !exists {
		t.Fatal("expected curl_example when includeExamples=true")
	}
}
