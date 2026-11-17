package marketplace

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// ModelStatus is the availability of a catalog model.
type ModelStatus string

const (
	ModelOnline   ModelStatus = "ONLINE"
	ModelDegraded ModelStatus = "DEGRADED"
)

// ApiKeyStatus is the lifecycle state of an API key.
type ApiKeyStatus string

const (
	ApiKeyActive  ApiKeyStatus = "ACTIVE"
	ApiKeyRevoked ApiKeyStatus = "REVOKED"
)

// apiKeyScopes is the closed set of scopes an API key may carry.
var apiKeyScopes = map[string]struct{}{
	"tasks:read":  {},
	"tasks:write": {},
	"validate":    {},
	"compliance":  {},
}

// ModelCatalogItem is one model exposed by the API gateway.
type ModelCatalogItem struct {
	ID                 string      `json:"id"`
	Provider           string      `json:"provider"`
	Name               string      `json:"name"`
	ContextWindow      int         `json:"contextWindow"`
	InputPricePerMTok  float64     `json:"inputPricePerMTok"`
	OutputPricePerMTok float64     `json:"outputPricePerMTok"`
	LatencyMs          int         `json:"latencyMs"`
	Status             ModelStatus `json:"status"`
	Capabilities       []string    `json:"capabilities"`
}

// ApiKeyRecord is a masked API key entry.
type ApiKeyRecord struct {
	ID         string       `json:"id"`
	Label      string       `json:"label"`
	MaskedKey  string       `json:"maskedKey"`
	CreatedAt  string       `json:"createdAt"`
	LastUsedAt *string      `json:"lastUsedAt"`
	Scopes     []string     `json:"scopes"`
	Status     ApiKeyStatus `json:"status"`
}

// ApiUsage is the current billing-period usage snapshot.
type ApiUsage struct {
	Period    string  `json:"period"`
	TokensIn  int     `json:"tokensIn"`
	TokensOut int     `json:"tokensOut"`
	Requests  int     `json:"requests"`
	Spend     float64 `json:"spend"`
}

// ApiGatewayOverview is GET /api/gateway.
type ApiGatewayOverview struct {
	Models []ModelCatalogItem `json:"models"`
	Keys   []ApiKeyRecord     `json:"keys"`
	Usage  ApiUsage           `json:"usage"`
}

// CreateApiKeyInput is POST /api/gateway/keys.
type CreateApiKeyInput struct {
	Label  string   `json:"label"`
	Scopes []string `json:"scopes"`
}

// CreateApiKeyResponse returns the record plus the one-time plaintext key.
type CreateApiKeyResponse struct {
	Key          ApiKeyRecord `json:"key"`
	PlaintextKey string       `json:"plaintextKey"`
}

// GatewayState is the persisted API-gateway snapshot.
type GatewayState struct {
	Models []ModelCatalogItem `json:"models"`
	Keys   []ApiKeyRecord     `json:"keys"`
	Usage  ApiUsage           `json:"usage"`
}

func seedModels() []ModelCatalogItem {
	return []ModelCatalogItem{
		{
			ID: "model_deepseek", Provider: "DeepSeek", Name: "deepseek-v3.2",
			ContextWindow: 131072, InputPricePerMTok: 0.5, OutputPricePerMTok: 2.0,
			LatencyMs: 720, Status: ModelOnline, Capabilities: []string{"chat", "code", "reasoning"},
		},
		{
			ID: "model_qwen", Provider: "Alibaba", Name: "qwen3-max",
			ContextWindow: 262144, InputPricePerMTok: 0.8, OutputPricePerMTok: 3.2,
			LatencyMs: 680, Status: ModelOnline, Capabilities: []string{"chat", "code", "vision"},
		},
		{
			ID: "model_glm", Provider: "Zhipu", Name: "glm-4.6",
			ContextWindow: 128000, InputPricePerMTok: 0.6, OutputPricePerMTok: 2.2,
			LatencyMs: 760, Status: ModelOnline, Capabilities: []string{"chat", "code"},
		},
		{
			ID: "model_kimi", Provider: "Moonshot", Name: "kimi-k2",
			ContextWindow: 256000, InputPricePerMTok: 0.9, OutputPricePerMTok: 3.6,
			LatencyMs: 890, Status: ModelOnline, Capabilities: []string{"chat", "long-context"},
		},
		{
			ID: "model_minimax", Provider: "MiniMax", Name: "minimax-m2",
			ContextWindow: 204800, InputPricePerMTok: 0.4, OutputPricePerMTok: 1.6,
			LatencyMs: 940, Status: ModelDegraded, Capabilities: []string{"chat", "code"},
		},
		{
			ID: "model_claude", Provider: "Anthropic", Name: "claude-opus-5-5",
			ContextWindow: 200000, InputPricePerMTok: 15, OutputPricePerMTok: 75,
			LatencyMs: 1100, Status: ModelOnline, Capabilities: []string{"chat", "code", "vision", "reasoning"},
		},
		{
			ID: "model_gpt", Provider: "OpenAI", Name: "gpt-5-codex",
			ContextWindow: 400000, InputPricePerMTok: 10, OutputPricePerMTok: 40,
			LatencyMs: 980, Status: ModelOnline, Capabilities: []string{"chat", "code", "reasoning"},
		},
		{
			ID: "model_gemini", Provider: "Google", Name: "gemini-3-pro",
			ContextWindow: 1000000, InputPricePerMTok: 1.25, OutputPricePerMTok: 10,
			LatencyMs: 850, Status: ModelOnline, Capabilities: []string{"chat", "vision", "long-context"},
		},
	}
}

func seedApiKeys() []ApiKeyRecord {
	return []ApiKeyRecord{
		{
			ID: "key_001", Label: "Claude Code 生产环境", MaskedKey: "sk-pt-****-7f3a",
			CreatedAt: "2026-11-12T08:00:00Z", LastUsedAt: stringPtr("2026-11-16T02:30:00Z"),
			Scopes: []string{"tasks:read", "tasks:write"}, Status: ApiKeyActive,
		},
	}
}

func seedApiUsage() ApiUsage {
	return ApiUsage{
		Period:    time.Now().UTC().Format("2006-01"),
		TokensIn:  1284000,
		TokensOut: 496000,
		Requests:  3912,
		Spend:     86.4,
	}
}

func seedGateway() GatewayState {
	return GatewayState{
		Models: seedModels(),
		Keys:   seedApiKeys(),
		Usage:  seedApiUsage(),
	}
}

const seedNextApiKeyID = 2

// Gateway returns a copy of the API-gateway overview.
func (s *Store) Gateway() ApiGatewayOverview {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return ApiGatewayOverview{
		Models: cloneModels(s.gateway.Models),
		Keys:   cloneApiKeys(s.gateway.Keys),
		Usage:  s.gateway.Usage,
	}
}

// CreateApiKey validates the request, generates a one-time plaintext key and
// stores the masked record.
func (s *Store) CreateApiKey(input CreateApiKeyInput) (CreateApiKeyResponse, error) {
	label := strings.TrimSpace(input.Label)
	if label == "" {
		return CreateApiKeyResponse{}, newError(http.StatusBadRequest, "label is required")
	}
	if len(input.Scopes) == 0 {
		return CreateApiKeyResponse{}, newError(http.StatusBadRequest, "scopes are required")
	}
	for _, scope := range input.Scopes {
		if _, ok := apiKeyScopes[scope]; !ok {
			return CreateApiKeyResponse{}, newError(http.StatusBadRequest, "invalid scope %s", scope)
		}
	}

	raw := make([]byte, 4)
	if _, err := rand.Read(raw); err != nil {
		return CreateApiKeyResponse{}, newError(http.StatusInternalServerError, "generate api key: %v", err)
	}
	suffix := hex.EncodeToString(raw)
	plaintext := "sk-pt-" + suffix

	s.mu.Lock()
	defer s.mu.Unlock()

	record := ApiKeyRecord{
		ID:         fmt.Sprintf("key_%03d", s.nextApiKeyID),
		Label:      label,
		MaskedKey:  "sk-pt-****-" + suffix[len(suffix)-4:],
		CreatedAt:  nowString(),
		LastUsedAt: nil,
		Scopes:     cloneStrings(input.Scopes),
		Status:     ApiKeyActive,
	}
	s.nextApiKeyID++
	s.gateway.Keys = append([]ApiKeyRecord{record}, s.gateway.Keys...)

	if err := s.persistLocked(); err != nil {
		return CreateApiKeyResponse{}, newError(http.StatusInternalServerError, "persist api key: %v", err)
	}

	return CreateApiKeyResponse{Key: cloneApiKey(record), PlaintextKey: plaintext}, nil
}

// RevokeApiKey marks a key REVOKED. Unknown keys yield a 404 domain error.
func (s *Store) RevokeApiKey(id string) (ApiKeyRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for index, record := range s.gateway.Keys {
		if record.ID != id {
			continue
		}
		s.gateway.Keys[index].Status = ApiKeyRevoked
		if err := s.persistLocked(); err != nil {
			return ApiKeyRecord{}, newError(http.StatusInternalServerError, "persist api key revocation: %v", err)
		}
		return cloneApiKey(s.gateway.Keys[index]), nil
	}
	return ApiKeyRecord{}, newError(http.StatusNotFound, "api key not found")
}

func cloneModels(input []ModelCatalogItem) []ModelCatalogItem {
	output := make([]ModelCatalogItem, len(input))
	for index, model := range input {
		output[index] = model
		output[index].Capabilities = cloneStrings(model.Capabilities)
	}
	return output
}

func cloneApiKeys(input []ApiKeyRecord) []ApiKeyRecord {
	output := make([]ApiKeyRecord, len(input))
	for index, record := range input {
		output[index] = cloneApiKey(record)
	}
	return output
}

func cloneApiKey(input ApiKeyRecord) ApiKeyRecord {
	output := input
	output.Scopes = cloneStrings(input.Scopes)
	if input.LastUsedAt != nil {
		output.LastUsedAt = stringPtr(*input.LastUsedAt)
	}
	return output
}
