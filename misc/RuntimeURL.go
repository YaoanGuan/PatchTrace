package misc

import (
	"net"
	"net/url"
	"strings"
	"sync"
)

var (
	runtimeServerBaseURL   string
	runtimeServerBaseURLMu sync.RWMutex
)

func SetRuntimeServerBaseURL(raw string) {
	runtimeServerBaseURLMu.Lock()
	defer runtimeServerBaseURLMu.Unlock()
	runtimeServerBaseURL = normalizeRuntimeBaseURL(raw)
}

func GetRuntimeServerBaseURL() string {
	runtimeServerBaseURLMu.RLock()
	defer runtimeServerBaseURLMu.RUnlock()
	return runtimeServerBaseURL
}

func GetAgentToolsBaseURL() string {
	if runtime := GetRuntimeServerBaseURL(); runtime != "" {
		return runtime
	}
	return normalizeRuntimeBaseURL(GetConfigValueDefault("misc", "AGENT_TOOLS_BASE_URL", ""))
}

func GetRuntimeServerHost() string {
	baseURL := GetRuntimeServerBaseURL()
	if baseURL == "" {
		baseURL = GetAgentToolsBaseURL()
	}
	if baseURL == "" {
		return ""
	}
	parsed, err := url.Parse(baseURL)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(parsed.Hostname())
}

func ComposeRuntimeBaseURL(host string, port string) string {
	host = strings.TrimSpace(host)
	port = strings.TrimSpace(port)
	if host == "" || port == "" {
		return ""
	}
	return "http://" + net.JoinHostPort(host, port)
}

func normalizeRuntimeBaseURL(raw string) string {
	raw = strings.TrimSpace(raw)
	return strings.TrimRight(raw, "/")
}
