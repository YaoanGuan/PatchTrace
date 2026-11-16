package llm

import (
	"AIxVuln/misc"
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"

	"github.com/openai/openai-go/v3/option"
)

// --- per-API-key rate limiting ---

// rateLimitedClient wraps a Client with a per-API-key semaphore.
type rateLimitedClient struct {
	inner     Client
	semaphore chan struct{}
}

func (r *rateLimitedClient) Chat(ctx context.Context, model string, messages []Message, tools []ToolDef) (Response, error) {
	select {
	case r.semaphore <- struct{}{}:
		defer func() { <-r.semaphore }()
	case <-ctx.Done():
		return Response{}, ctx.Err()
	}
	return r.inner.Chat(ctx, model, messages, tools)
}

// keySemaphores stores one semaphore per API key so that all clients sharing
// the same key share the same concurrency limit.
var keySemaphores = struct {
	mu sync.Mutex
	m  map[string]chan struct{}
}{m: make(map[string]chan struct{})}

// getOrCreateSemaphore returns the semaphore for the given API key,
// creating one with the specified capacity if it doesn't exist yet.
func getOrCreateSemaphore(apiKey string, capacity int) chan struct{} {
	keySemaphores.mu.Lock()
	defer keySemaphores.mu.Unlock()
	if sem, ok := keySemaphores.m[apiKey]; ok {
		return sem
	}
	sem := make(chan struct{}, capacity)
	keySemaphores.m[apiKey] = sem
	return sem
}

type llmSectionConfig struct {
	section    string
	baseURL    string
	apiKeys    []string
	userAgent  string
	apiMode    string
	backend    string
	maxRequest int
}

type llmKeyRing struct {
	mu        sync.Mutex
	keys      []string
	nextIndex int
}

func (r *llmKeyRing) next() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.keys) == 0 {
		return ""
	}
	key := r.keys[r.nextIndex]
	r.nextIndex++
	if r.nextIndex >= len(r.keys) {
		r.nextIndex = 0
	}
	return key
}

var sharedKeyRings = struct {
	mu sync.Mutex
	m  map[string]*llmKeyRing
}{m: make(map[string]*llmKeyRing)}

var cachedLLMClients = struct {
	mu sync.Mutex
	m  map[string]Client
}{m: make(map[string]Client)}

func splitConfiguredAPIKeys(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, "|-|")
	keys := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		keys = append(keys, part)
	}
	return keys
}

func normalizeLLMBaseURL(baseURL string) string {
	trimmed := strings.TrimSpace(baseURL)
	if trimmed == "" {
		return ""
	}
	return strings.TrimRight(trimmed, "/")
}

func resolveLLMSectionConfig(section string) *llmSectionConfig {
	if strings.TrimSpace(section) == "" {
		return nil
	}
	keys := splitConfiguredAPIKeys(misc.GetConfigValueDefault(section, "OPENAI_API_KEY", ""))
	if len(keys) == 0 {
		return nil
	}

	userAgent := misc.GetConfigValueDefault(section, "USER_AGENT", "")
	if userAgent == "" {
		userAgent = misc.GetConfigValueDefault("main_setting", "USER_AGENT", "AIxVuln")
	}

	rawMode := misc.GetConfigValueDefault(section, "API_MODE", "")
	if rawMode == "" {
		rawMode = misc.GetConfigValueDefault("main_setting", "API_MODE", "chat")
	}
	apiMode := strings.ToLower(strings.TrimSpace(rawMode))
	if apiMode == "" {
		apiMode = "chat"
	}

	maxReqStr := misc.GetConfigValueDefault(section, "MaxRequest",
		misc.GetConfigValueDefault("main_setting", "MaxRequest", "3"))
	maxReq, _ := strconv.Atoi(maxReqStr)
	if maxReq <= 0 {
		maxReq = 3
	}

	baseURL := normalizeLLMBaseURL(misc.GetConfigValueDefault(section, "BASE_URL", ""))
	backend := ResolveLLMBackend(apiMode, baseURL)
	misc.Debug("GetResponsesClient: section=%s, API_MODE raw=%q resolved=%q, keys=%d",
		section, rawMode, apiMode, len(keys))

	return &llmSectionConfig{
		section:    section,
		baseURL:    baseURL,
		apiKeys:    keys,
		userAgent:  userAgent,
		apiMode:    apiMode,
		backend:    backend,
		maxRequest: maxReq,
	}
}

func keyRingSignature(baseURL string, keys []string) string {
	return normalizeLLMBaseURL(baseURL) + "\x00" + strings.Join(keys, "\x00")
}

func getOrCreateKeyRing(baseURL string, keys []string) *llmKeyRing {
	signature := keyRingSignature(baseURL, keys)
	sharedKeyRings.mu.Lock()
	defer sharedKeyRings.mu.Unlock()
	if ring, ok := sharedKeyRings.m[signature]; ok {
		return ring
	}
	copiedKeys := append([]string(nil), keys...)
	ring := &llmKeyRing{keys: copiedKeys}
	sharedKeyRings.m[signature] = ring
	return ring
}

func nextRotatedAPIKey(baseURL string, keys []string) string {
	if len(keys) == 0 {
		return ""
	}
	return getOrCreateKeyRing(baseURL, keys).next()
}

func clientCacheSignature(cfg *llmSectionConfig, apiKey string) string {
	return strings.Join([]string{
		cfg.section,
		cfg.baseURL,
		cfg.apiMode,
		cfg.backend,
		cfg.userAgent,
		apiKey,
	}, "\x00")
}

func getOrCreateCachedClient(cfg *llmSectionConfig, apiKey string) Client {
	signature := clientCacheSignature(cfg, apiKey)
	cachedLLMClients.mu.Lock()
	defer cachedLLMClients.mu.Unlock()
	if cli, ok := cachedLLMClients.m[signature]; ok {
		return cli
	}

	var inner Client
	switch cfg.backend {
	case BackendAnthropic:
		inner = NewAnthropicMessagesClient(cfg.section, cfg.baseURL, apiKey, cfg.userAgent)
	case BackendResponses:
		opts := []option.RequestOption{
			option.WithAPIKey(apiKey),
			option.WithHeader("User-Agent", cfg.userAgent),
		}
		if cfg.baseURL != "" {
			opts = append(opts, option.WithBaseURL(cfg.baseURL))
		}
		inner = NewOpenAIResponsesClient(cfg.section, opts...)
	default:
		opts := []option.RequestOption{
			option.WithAPIKey(apiKey),
			option.WithHeader("User-Agent", cfg.userAgent),
		}
		if cfg.baseURL != "" {
			opts = append(opts, option.WithBaseURL(cfg.baseURL))
		}
		inner = NewOpenAIChatClient(cfg.section, opts...)
	}

	sem := getOrCreateSemaphore(apiKey, cfg.maxRequest)
	cli := &rateLimitedClient{inner: inner, semaphore: sem}
	cachedLLMClients.m[signature] = cli
	misc.Debug("GetResponsesClient: created client for section=%s, key=...%s, maxRequest=%d",
		cfg.section, apiKey[max(0, len(apiKey)-6):], cfg.maxRequest)
	return cli
}

// GetResponsesClient returns an LLM client for the given config section(s).
// It reads BASE_URL, OPENAI_API_KEY from the section, and USER_AGENT,
// API_MODE, MaxRequest from the section first, falling back to [main_setting].
//
// API_MODE:
//   - "responses" (default) — uses the Responses API
//   - "chat" — uses the Chat Completions API
//
// Multiple API keys are separated by "|-|". Rotation is shared globally by
// the exact key list and base URL so different sections using the same key
// ring advance together instead of each starting from key #1 independently.
func GetResponsesClient(sections ...string) Client {
	for _, section := range sections {
		cfg := resolveLLMSectionConfig(section)
		if cfg == nil {
			continue
		}
		apiKey := nextRotatedAPIKey(cfg.baseURL, cfg.apiKeys)
		if apiKey == "" {
			continue
		}
		return getOrCreateCachedClient(cfg, apiKey)
	}
	return nil
}

// NewAdHocClient creates a temporary client from raw settings so the caller
// can test draft values before saving them into the config store.
func NewAdHocClient(baseURL, apiKey, apiMode, userAgent string, maxRequest int) (Client, string, error) {
	keys := splitConfiguredAPIKeys(apiKey)
	if len(keys) == 0 {
		return nil, "", fmt.Errorf("api key is empty")
	}
	if maxRequest <= 0 {
		maxRequest = 1
	}
	baseURL = normalizeLLMBaseURL(baseURL)
	userAgent = strings.TrimSpace(userAgent)
	if userAgent == "" {
		userAgent = "AIxVuln"
	}
	apiMode = strings.ToLower(strings.TrimSpace(apiMode))
	if apiMode == "" {
		apiMode = BackendChat
	}
	backend := ResolveLLMBackend(apiMode, baseURL)
	cfg := &llmSectionConfig{
		section:    "adhoc",
		baseURL:    baseURL,
		apiKeys:    append([]string(nil), keys...),
		userAgent:  userAgent,
		apiMode:    apiMode,
		backend:    backend,
		maxRequest: maxRequest,
	}
	selectedKey := nextRotatedAPIKey(baseURL, keys)
	if selectedKey == "" {
		return nil, "", fmt.Errorf("api key is empty")
	}
	return getOrCreateCachedClient(cfg, selectedKey), backend, nil
}
