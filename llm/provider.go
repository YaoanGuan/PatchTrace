package llm

import (
	"net/url"
	"strings"
)

const (
	BackendChat      = "chat"
	BackendResponses = "responses"
	BackendAnthropic = "anthropic"
	BackendClaude    = "claude"
	BackendMessages  = "messages"
)

func ResolveLLMBackend(apiMode, baseURL string) string {
	mode := strings.ToLower(strings.TrimSpace(apiMode))
	switch mode {
	case BackendResponses:
		return BackendResponses
	case BackendAnthropic, BackendClaude, BackendMessages:
		return BackendAnthropic
	}
	if IsAnthropicBaseURL(baseURL) {
		return BackendAnthropic
	}
	return BackendChat
}

func IsAnthropicBaseURL(baseURL string) bool {
	trimmed := normalizeLLMBaseURL(baseURL)
	if trimmed == "" {
		return false
	}
	u, err := url.Parse(trimmed)
	if err != nil {
		return strings.Contains(strings.ToLower(trimmed), "anthropic")
	}
	host := strings.ToLower(strings.TrimSpace(u.Hostname()))
	if host == "" {
		return strings.Contains(strings.ToLower(trimmed), "anthropic")
	}
	return strings.Contains(host, "anthropic.com")
}

func AnthropicMessagesURL(baseURL string) string {
	return anthropicEndpointURL(baseURL, "/messages")
}

func AnthropicModelsURL(baseURL string) string {
	return anthropicEndpointURL(baseURL, "/models")
}

func anthropicEndpointURL(baseURL, endpoint string) string {
	trimmed := normalizeLLMBaseURL(baseURL)
	if trimmed == "" {
		return ""
	}
	u, err := url.Parse(trimmed)
	if err != nil {
		if strings.HasSuffix(trimmed, "/v1"+endpoint) {
			return trimmed
		}
		if strings.HasSuffix(trimmed, "/v1") {
			return trimmed + endpoint
		}
		return trimmed + "/v1" + endpoint
	}

	path := strings.TrimRight(u.Path, "/")
	switch {
	case strings.HasSuffix(path, "/v1"+endpoint):
		// already points at the exact endpoint
	case strings.HasSuffix(path, "/v1"):
		path += endpoint
	default:
		path += "/v1" + endpoint
	}
	u.Path = path
	u.RawPath = ""
	return u.String()
}
