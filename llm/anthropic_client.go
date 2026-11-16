package llm

import (
	"AIxVuln/misc"
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const anthropicVersionHeader = "2023-06-01"

// AnthropicMessagesClient implements Client using Anthropic's Messages API.
type AnthropicMessagesClient struct {
	httpClient    *http.Client
	configSection string
	baseURL       string
	apiKey        string
	userAgent     string
}

func NewAnthropicMessagesClient(configSection, baseURL, apiKey, userAgent string) *AnthropicMessagesClient {
	return &AnthropicMessagesClient{
		httpClient:    &http.Client{Timeout: 10 * time.Minute},
		configSection: configSection,
		baseURL:       normalizeLLMBaseURL(baseURL),
		apiKey:        strings.TrimSpace(apiKey),
		userAgent:     strings.TrimSpace(userAgent),
	}
}

func (c *AnthropicMessagesClient) Chat(ctx context.Context, model string, messages []Message, tools []ToolDef) (Response, error) {
	systemPrompt, anthropicMessages := messagesToAnthropicMessages(SanitizeToolCallMessages(messages))
	reqBody := anthropicMessagesRequest{
		Model:    model,
		System:   systemPrompt,
		Messages: anthropicMessages,
		Tools:    toolDefsToAnthropicTools(tools),
	}
	if maxOutput := ConfiguredMaxOutputTokens(c.configSection); maxOutput > 0 {
		reqBody.MaxTokens = maxOutput
	} else {
		reqBody.MaxTokens = defaultAnthropicMaxTokens
	}

	streamVal := misc.GetConfigValueDefault(c.configSection, "STREAM", "")
	if streamVal == "" {
		streamVal = misc.GetConfigValueDefault("main_setting", "STREAM", "false")
	}
	useStream := strings.EqualFold(streamVal, "true")
	if useStream {
		reqBody.Stream = true
		return c.chatStream(ctx, reqBody)
	}
	return c.chatSync(ctx, reqBody)
}

const defaultAnthropicMaxTokens int64 = 1024

type anthropicMessagesRequest struct {
	Model     string             `json:"model"`
	System    string             `json:"system,omitempty"`
	Messages  []anthropicMessage `json:"messages"`
	Tools     []anthropicTool    `json:"tools,omitempty"`
	MaxTokens int64              `json:"max_tokens"`
	Stream    bool               `json:"stream,omitempty"`
}

type anthropicMessage struct {
	Role    string                  `json:"role"`
	Content []anthropicContentBlock `json:"content"`
}

type anthropicContentBlock struct {
	Type      string          `json:"type"`
	Text      string          `json:"text,omitempty"`
	ID        string          `json:"id,omitempty"`
	Name      string          `json:"name,omitempty"`
	Input     json.RawMessage `json:"input,omitempty"`
	ToolUseID string          `json:"tool_use_id,omitempty"`
	Content   interface{}     `json:"content,omitempty"`
	IsError   bool            `json:"is_error,omitempty"`
}

type anthropicTool struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description,omitempty"`
	InputSchema map[string]interface{} `json:"input_schema,omitempty"`
}

type anthropicResponse struct {
	Type       string                  `json:"type"`
	ID         string                  `json:"id"`
	Role       string                  `json:"role"`
	Content    []anthropicContentBlock `json:"content"`
	StopReason string                  `json:"stop_reason"`
	Usage      anthropicResponseUsage  `json:"usage"`
	Error      *anthropicErrorEnvelope `json:"error,omitempty"`
}

type anthropicResponseUsage struct {
	InputTokens  int64 `json:"input_tokens"`
	OutputTokens int64 `json:"output_tokens"`
}

type anthropicErrorEnvelope struct {
	Type    string `json:"type"`
	Message string `json:"message"`
}

type anthropicStreamBlock struct {
	typ   string
	text  strings.Builder
	id    string
	name  string
	input strings.Builder
}

func (c *AnthropicMessagesClient) chatSync(ctx context.Context, reqBody anthropicMessagesRequest) (Response, error) {
	resp, err := c.doAnthropicRequest(ctx, reqBody)
	if err != nil {
		return Response{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return Response{}, anthropicAPIError(resp)
	}

	var parsed anthropicResponse
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return Response{}, fmt.Errorf("decode anthropic response: %w", err)
	}
	return fromAnthropicResponse(&parsed)
}

func (c *AnthropicMessagesClient) chatStream(ctx context.Context, reqBody anthropicMessagesRequest) (Response, error) {
	resp, err := c.doAnthropicRequest(ctx, reqBody)
	if err != nil {
		return Response{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return Response{}, anthropicAPIError(resp)
	}

	return parseAnthropicStream(resp.Body)
}

func (c *AnthropicMessagesClient) doAnthropicRequest(ctx context.Context, reqBody anthropicMessagesRequest) (*http.Response, error) {
	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal anthropic request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, AnthropicMessagesURL(c.baseURL), bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create anthropic request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", c.apiKey)
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("anthropic-version", anthropicVersionHeader)
	if c.userAgent != "" {
		req.Header.Set("User-Agent", c.userAgent)
	}
	return c.httpClient.Do(req)
}

func messagesToAnthropicMessages(msgs []Message) (string, []anthropicMessage) {
	var (
		systemParts []string
		out         []anthropicMessage
	)

	for _, m := range msgs {
		switch m.Role {
		case RoleSystem:
			if strings.TrimSpace(m.Content) != "" {
				systemParts = append(systemParts, m.Content)
			}
		case RoleUser:
			out = appendAnthropicMessage(out, "user", []anthropicContentBlock{{
				Type: "text",
				Text: m.Content,
			}})
		case RoleAssistant:
			blocks := make([]anthropicContentBlock, 0, len(m.ToolCalls)+1)
			if strings.TrimSpace(m.Content) != "" {
				blocks = append(blocks, anthropicContentBlock{
					Type: "text",
					Text: m.Content,
				})
			}
			for _, tc := range m.ToolCalls {
				blocks = append(blocks, anthropicContentBlock{
					Type:  "tool_use",
					ID:    tc.ID,
					Name:  tc.Name,
					Input: normalizeAnthropicToolInput(tc.Arguments),
				})
			}
			out = appendAnthropicMessage(out, "assistant", blocks)
		case RoleTool:
			out = appendAnthropicMessage(out, "user", []anthropicContentBlock{{
				Type:      "tool_result",
				ToolUseID: m.ToolCallID,
				Content:   m.Content,
				IsError:   anthropicToolResultIsError(m.Content),
			}})
		}
	}

	return strings.Join(systemParts, "\n\n"), out
}

func appendAnthropicMessage(out []anthropicMessage, role string, blocks []anthropicContentBlock) []anthropicMessage {
	filtered := make([]anthropicContentBlock, 0, len(blocks))
	for _, block := range blocks {
		switch block.Type {
		case "text":
			if block.Text == "" {
				continue
			}
		case "tool_result":
			if strings.TrimSpace(block.ToolUseID) == "" {
				continue
			}
		case "tool_use":
			if strings.TrimSpace(block.ID) == "" || strings.TrimSpace(block.Name) == "" {
				continue
			}
		}
		filtered = append(filtered, block)
	}
	if len(filtered) == 0 {
		return out
	}
	if len(out) > 0 && out[len(out)-1].Role == role {
		out[len(out)-1].Content = append(out[len(out)-1].Content, filtered...)
		return out
	}
	return append(out, anthropicMessage{
		Role:    role,
		Content: filtered,
	})
}

func normalizeAnthropicToolInput(arguments string) json.RawMessage {
	raw := strings.TrimSpace(arguments)
	if raw == "" {
		return json.RawMessage(`{}`)
	}
	if !json.Valid([]byte(raw)) {
		escaped, _ := json.Marshal(map[string]string{"raw_arguments": raw})
		return json.RawMessage(escaped)
	}
	var compact bytes.Buffer
	if err := json.Compact(&compact, []byte(raw)); err != nil {
		return json.RawMessage(raw)
	}
	return json.RawMessage(compact.Bytes())
}

func anthropicToolResultIsError(content string) bool {
	var payload struct {
		Success *bool `json:"success"`
	}
	if err := json.Unmarshal([]byte(content), &payload); err == nil && payload.Success != nil {
		return !*payload.Success
	}
	return false
}

func toolDefsToAnthropicTools(defs []ToolDef) []anthropicTool {
	if len(defs) == 0 {
		return nil
	}
	out := make([]anthropicTool, len(defs))
	for i, d := range defs {
		out[i] = anthropicTool{
			Name:        d.Name,
			Description: d.Description,
			InputSchema: d.Parameters,
		}
	}
	return out
}

func fromAnthropicResponse(resp *anthropicResponse) (Response, error) {
	if resp == nil {
		return Response{}, fmt.Errorf("empty anthropic response")
	}
	if resp.Error != nil {
		msg := strings.TrimSpace(resp.Error.Message)
		if msg == "" {
			msg = "unknown anthropic error"
		}
		return Response{}, fmt.Errorf("anthropic error (%s): %s", resp.Error.Type, msg)
	}

	var r Response
	for _, block := range resp.Content {
		switch block.Type {
		case "text":
			r.Content += block.Text
		case "tool_use":
			r.ToolCalls = append(r.ToolCalls, ToolCall{
				ID:        block.ID,
				Name:      block.Name,
				Arguments: string(defaultJSONRaw(block.Input, "{}")),
			})
		}
	}
	if resp.Usage.InputTokens > 0 || resp.Usage.OutputTokens > 0 {
		r.Usage = Usage{
			PromptTokens:     resp.Usage.InputTokens,
			CompletionTokens: resp.Usage.OutputTokens,
			TotalTokens:      resp.Usage.InputTokens + resp.Usage.OutputTokens,
		}
	}
	if r.Content == "" && len(r.ToolCalls) == 0 {
		return Response{}, fmt.Errorf("empty anthropic response (stop_reason=%s)", resp.StopReason)
	}
	return r, nil
}

func anthropicAPIError(resp *http.Response) error {
	body, _ := io.ReadAll(resp.Body)
	var parsed anthropicResponse
	if err := json.Unmarshal(body, &parsed); err == nil && parsed.Error != nil {
		return fmt.Errorf("anthropic api error (%d %s): %s", resp.StatusCode, parsed.Error.Type, parsed.Error.Message)
	}
	trimmed := strings.TrimSpace(string(body))
	if trimmed == "" {
		trimmed = resp.Status
	}
	return fmt.Errorf("anthropic api error (%d): %s", resp.StatusCode, trimmed)
}

func parseAnthropicStream(body io.Reader) (Response, error) {
	scanner := bufio.NewScanner(body)
	scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)

	var (
		eventType string
		dataLines []string
		blocks    = make(map[int]*anthropicStreamBlock)
		resp      Response
	)

	flush := func() error {
		if eventType == "" && len(dataLines) == 0 {
			return nil
		}
		payload := strings.Join(dataLines, "\n")
		defer func() {
			eventType = ""
			dataLines = nil
		}()
		if payload == "" {
			return nil
		}
		return applyAnthropicStreamEvent(&resp, blocks, eventType, payload)
	}

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			if err := flush(); err != nil {
				return Response{}, err
			}
			continue
		}
		if strings.HasPrefix(line, ":") {
			continue
		}
		switch {
		case strings.HasPrefix(line, "event:"):
			eventType = strings.TrimSpace(strings.TrimPrefix(line, "event:"))
		case strings.HasPrefix(line, "data:"):
			dataLines = append(dataLines, strings.TrimSpace(strings.TrimPrefix(line, "data:")))
		}
	}
	if err := scanner.Err(); err != nil {
		return Response{}, err
	}
	if err := flush(); err != nil {
		return Response{}, err
	}

	if resp.Content == "" && len(resp.ToolCalls) == 0 {
		return Response{}, fmt.Errorf("empty anthropic streaming response")
	}
	return resp, nil
}

func applyAnthropicStreamEvent(resp *Response, blocks map[int]*anthropicStreamBlock, eventType, payload string) error {
	switch eventType {
	case "ping":
		return nil
	case "error":
		var evt struct {
			Error anthropicErrorEnvelope `json:"error"`
		}
		if err := json.Unmarshal([]byte(payload), &evt); err != nil {
			return fmt.Errorf("decode anthropic stream error: %w", err)
		}
		msg := strings.TrimSpace(evt.Error.Message)
		if msg == "" {
			msg = "unknown anthropic streaming error"
		}
		return fmt.Errorf("anthropic stream error (%s): %s", evt.Error.Type, msg)
	case "message_start":
		var evt struct {
			Message struct {
				Usage anthropicResponseUsage `json:"usage"`
			} `json:"message"`
		}
		if err := json.Unmarshal([]byte(payload), &evt); err != nil {
			return fmt.Errorf("decode anthropic message_start: %w", err)
		}
		resp.Usage.PromptTokens = evt.Message.Usage.InputTokens
		resp.Usage.TotalTokens = evt.Message.Usage.InputTokens + resp.Usage.CompletionTokens
		return nil
	case "content_block_start":
		var evt struct {
			Index        int                   `json:"index"`
			ContentBlock anthropicContentBlock `json:"content_block"`
		}
		if err := json.Unmarshal([]byte(payload), &evt); err != nil {
			return fmt.Errorf("decode anthropic content_block_start: %w", err)
		}
		block := &anthropicStreamBlock{
			typ:  evt.ContentBlock.Type,
			id:   evt.ContentBlock.ID,
			name: evt.ContentBlock.Name,
		}
		if evt.ContentBlock.Type == "text" && evt.ContentBlock.Text != "" {
			block.text.WriteString(evt.ContentBlock.Text)
		}
		if evt.ContentBlock.Type == "tool_use" && len(evt.ContentBlock.Input) > 0 {
			block.input.Write(defaultJSONRaw(evt.ContentBlock.Input, "{}"))
		}
		blocks[evt.Index] = block
		return nil
	case "content_block_delta":
		var evt struct {
			Index int `json:"index"`
			Delta struct {
				Type        string `json:"type"`
				Text        string `json:"text"`
				PartialJSON string `json:"partial_json"`
			} `json:"delta"`
		}
		if err := json.Unmarshal([]byte(payload), &evt); err != nil {
			return fmt.Errorf("decode anthropic content_block_delta: %w", err)
		}
		block, ok := blocks[evt.Index]
		if !ok {
			return nil
		}
		switch evt.Delta.Type {
		case "text_delta":
			block.text.WriteString(evt.Delta.Text)
		case "input_json_delta":
			block.input.WriteString(evt.Delta.PartialJSON)
		}
		return nil
	case "content_block_stop":
		var evt struct {
			Index int `json:"index"`
		}
		if err := json.Unmarshal([]byte(payload), &evt); err != nil {
			return fmt.Errorf("decode anthropic content_block_stop: %w", err)
		}
		block, ok := blocks[evt.Index]
		if !ok {
			return nil
		}
		switch block.typ {
		case "text":
			resp.Content += block.text.String()
		case "tool_use":
			resp.ToolCalls = append(resp.ToolCalls, ToolCall{
				ID:        block.id,
				Name:      block.name,
				Arguments: defaultJSONString(block.input.String(), "{}"),
			})
		}
		delete(blocks, evt.Index)
		return nil
	case "message_delta":
		var evt struct {
			Usage anthropicResponseUsage `json:"usage"`
		}
		if err := json.Unmarshal([]byte(payload), &evt); err != nil {
			return fmt.Errorf("decode anthropic message_delta: %w", err)
		}
		if evt.Usage.OutputTokens > 0 {
			resp.Usage.CompletionTokens = evt.Usage.OutputTokens
			resp.Usage.TotalTokens = resp.Usage.PromptTokens + resp.Usage.CompletionTokens
		}
		return nil
	case "message_stop":
		return nil
	default:
		return nil
	}
}

func defaultJSONString(raw, fallback string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return fallback
	}
	if !json.Valid([]byte(raw)) {
		return fallback
	}
	var compact bytes.Buffer
	if err := json.Compact(&compact, []byte(raw)); err != nil {
		return fallback
	}
	return compact.String()
}

func defaultJSONRaw(raw json.RawMessage, fallback string) []byte {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || !json.Valid(trimmed) {
		return []byte(fallback)
	}
	var compact bytes.Buffer
	if err := json.Compact(&compact, trimmed); err != nil {
		return []byte(fallback)
	}
	return compact.Bytes()
}
