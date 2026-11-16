package toolCalling

import (
	"context"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	maxMCPToolNameLength = 64
)

type MCPServerConfig struct {
	Name                 string                 `json:"name"`
	SharedConnectionName string                 `json:"shared_connection_name,omitempty"`
	DisplayName          string                 `json:"display_name,omitempty"`
	Type                 string                 `json:"type,omitempty"`
	Enabled              *bool                  `json:"enabled,omitempty"`
	Command              string                 `json:"command,omitempty"`
	Args                 []string               `json:"args,omitempty"`
	Env                  map[string]interface{} `json:"env,omitempty"`
	WorkDir              string                 `json:"workdir,omitempty"`
	URL                  string                 `json:"url,omitempty"`
	Headers              map[string]interface{} `json:"headers,omitempty"`
	ToolAllowlist        []string               `json:"tool_allowlist,omitempty"`
	ToolDenylist         []string               `json:"tool_denylist,omitempty"`
	ToolNamePrefix       string                 `json:"tool_name_prefix,omitempty"`
	PreserveSchema       bool                   `json:"preserve_schema,omitempty"`
	ConnectTimeoutSec    int                    `json:"connect_timeout_sec,omitempty"`
	CallTimeoutSec       int                    `json:"call_timeout_sec,omitempty"`
}

type mcpConnectedServer struct {
	cfg         MCPServerConfig
	session     *mcp.ClientSession
	callTimeout time.Duration
	httpClient  *http.Client
	mu          sync.Mutex
}

type mcpSharedRuntime struct {
	cfg    MCPServerConfig
	once   sync.Once
	server *mcpConnectedServer
	tools  []*mcp.Tool
	err    error
}

func (r *mcpSharedRuntime) load() (*mcpConnectedServer, []*mcp.Tool, error) {
	r.once.Do(func() {
		r.server, r.tools, r.err = connectSharedMCPServer(r.cfg)
	})
	tools := make([]*mcp.Tool, len(r.tools))
	copy(tools, r.tools)
	return r.server, tools, r.err
}

func (cfg MCPServerConfig) displayName() string {
	if name := strings.TrimSpace(cfg.DisplayName); name != "" {
		return name
	}
	return strings.TrimSpace(cfg.Name)
}

func (cfg MCPServerConfig) sharedConnectionNameOrDefault() string {
	if name := strings.TrimSpace(cfg.SharedConnectionName); name != "" {
		return name
	}
	return strings.TrimSpace(cfg.Name)
}

func (cfg MCPServerConfig) sharedConnectionConfig() MCPServerConfig {
	shared := cfg
	shared.Name = cfg.sharedConnectionNameOrDefault()
	shared.SharedConnectionName = shared.Name
	shared.ToolAllowlist = nil
	shared.ToolDenylist = nil
	shared.ToolNamePrefix = ""
	return shared
}

type mcpToolHandler struct {
	server         *mcpConnectedServer
	name           string
	remoteToolName string
	description    string
	parameters     map[string]interface{}
}

func (s *mcpConnectedServer) reconnectLocked() error {
	if s == nil {
		return fmt.Errorf("MCP server is unavailable")
	}
	if s.session != nil {
		_ = s.session.Close()
		s.session = nil
	}
	transport, httpClient, err := buildMCPTransportWithHTTPClient(s.cfg, s.httpClient)
	if err != nil {
		return err
	}
	if httpClient != nil {
		s.httpClient = httpClient
	}
	connectCtx, cancel := context.WithTimeout(context.Background(), s.cfg.connectTimeout())
	defer cancel()
	client := mcp.NewClient(&mcp.Implementation{Name: "AIxVuln", Version: "v1.0.0"}, nil)
	session, err := client.Connect(connectCtx, transport, nil)
	if err != nil {
		return err
	}
	s.session = session
	return nil
}

func (s *mcpConnectedServer) callTool(remoteToolName string, callArgs map[string]interface{}) (*mcp.CallToolResult, error) {
	if s == nil {
		return nil, fmt.Errorf("MCP server is unavailable")
	}
	ctx, cancel := context.WithTimeout(context.Background(), s.callTimeout)
	defer cancel()

	s.mu.Lock()
	defer s.mu.Unlock()
	if s.session == nil {
		if err := s.reconnectLocked(); err != nil {
			return nil, err
		}
	}
	res, err := s.session.CallTool(ctx, &mcp.CallToolParams{
		Name:      remoteToolName,
		Arguments: callArgs,
	})
	if err == nil {
		return res, nil
	}
	if !looksLikeReconnectableMCPError(err) {
		return nil, err
	}
	if reconnectErr := s.reconnectLocked(); reconnectErr != nil {
		return nil, fmt.Errorf("%v (reconnect failed: %w)", err, reconnectErr)
	}
	return s.session.CallTool(ctx, &mcp.CallToolParams{
		Name:      remoteToolName,
		Arguments: callArgs,
	})
}

func (h *mcpToolHandler) Name() string {
	return h.name
}

func (h *mcpToolHandler) Description() string {
	return h.description
}

func (h *mcpToolHandler) Parameters() map[string]interface{} {
	return cloneMap(h.parameters)
}

func (h *mcpToolHandler) Execute(args map[string]interface{}) string {
	if h.server == nil {
		return Fail("MCP session is not available")
	}
	callArgs := map[string]interface{}{}
	for k, v := range args {
		callArgs[k] = v
	}
	res, err := h.server.callTool(h.remoteToolName, callArgs)
	if err != nil {
		return Fail(fmt.Sprintf("MCP tool call failed (%s/%s): %v", h.server.cfg.displayName(), h.remoteToolName, err))
	}

	result := map[string]interface{}{
		"server":  h.server.cfg.displayName(),
		"tool":    h.remoteToolName,
		"isError": res.IsError,
	}
	if content := mcpContentAsAny(res.Content); content != nil {
		result["content"] = content
	}
	if res.StructuredContent != nil {
		result["structuredContent"] = res.StructuredContent
	}
	if txt := strings.TrimSpace(extractMCPText(res.Content)); txt != "" {
		result["text"] = txt
	}
	if res.IsError {
		return Fail(result)
	}
	return Success(result)
}

func looksLikeReconnectableMCPError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(strings.TrimSpace(err.Error()))
	if msg == "" {
		return false
	}
	for _, token := range []string{
		"connection closed",
		"client is closing",
		"failed to reconnect",
		"broken pipe",
		"eof",
		"transport is closing",
		"use of closed network connection",
	} {
		if strings.Contains(msg, token) {
			return true
		}
	}
	return false
}

func GetMCPToolHandlers() []ToolHandler {
	return GetMCPToolHandlersForProfiles()
}

func GetGitHubCopilotMCPToolHandlers() []ToolHandler {
	return GetMCPToolHandlersForAgentType("Agent-General-GeneralCommonAgent")
}

func connectMCPServer(cfg MCPServerConfig, usedNames map[string]struct{}) ([]ToolHandler, []MCPToolCatalogItem, error) {
	server, tools, err := connectSharedMCPServer(cfg)
	if err != nil {
		return nil, nil, err
	}
	handlers, catalog := buildMCPHandlersFromTools(cfg, server, tools, usedNames)
	if len(handlers) == 0 {
		if server != nil && server.session != nil {
			_ = server.session.Close()
		}
		return nil, nil, nil
	}
	return handlers, catalog, nil
}

func connectSharedMCPServer(cfg MCPServerConfig) (*mcpConnectedServer, []*mcp.Tool, error) {
	transport, httpClient, err := buildMCPTransportWithHTTPClient(cfg, nil)
	if err != nil {
		return nil, nil, err
	}

	connectCtx, cancel := context.WithTimeout(context.Background(), cfg.connectTimeout())
	defer cancel()

	client := mcp.NewClient(&mcp.Implementation{Name: "AIxVuln", Version: "v1.0.0"}, nil)
	session, err := client.Connect(connectCtx, transport, nil)
	if err != nil {
		return nil, nil, err
	}

	listCtx, listCancel := context.WithTimeout(context.Background(), cfg.connectTimeout())
	tools, err := listMCPTools(listCtx, session)
	listCancel()
	if err != nil {
		_ = session.Close()
		return nil, nil, err
	}
	server := &mcpConnectedServer{
		cfg:         cfg,
		session:     session,
		callTimeout: cfg.callTimeout(),
		httpClient:  httpClient,
	}
	return server, tools, nil
}

func buildMCPHandlersFromTools(cfg MCPServerConfig, server *mcpConnectedServer, tools []*mcp.Tool, usedNames map[string]struct{}) ([]ToolHandler, []MCPToolCatalogItem) {
	if server == nil || len(tools) == 0 {
		return nil, nil
	}
	allowSet := stringSet(cfg.ToolAllowlist)
	denySet := stringSet(cfg.ToolDenylist)
	handlers := make([]ToolHandler, 0, len(tools))
	catalog := make([]MCPToolCatalogItem, 0, len(tools))
	for _, tool := range tools {
		if tool == nil || strings.TrimSpace(tool.Name) == "" {
			continue
		}
		if len(allowSet) > 0 {
			if _, ok := allowSet[tool.Name]; !ok {
				continue
			}
		}
		if _, blocked := denySet[tool.Name]; blocked {
			continue
		}

		alias := buildMCPToolAlias(cfg, tool.Name, usedNames)
		desc := strings.TrimSpace(tool.Description)
		if desc == "" {
			desc = fmt.Sprintf("MCP tool %s from server %s", tool.Name, cfg.displayName())
		} else {
			desc = fmt.Sprintf("[MCP %s/%s] %s", cfg.displayName(), tool.Name, desc)
		}
		parameters := normalizeMCPInputSchema(tool.InputSchema)
		handlers = append(handlers, &mcpToolHandler{
			server:         server,
			name:           alias,
			remoteToolName: tool.Name,
			description:    desc,
			parameters:     parameters,
		})
		catalog = append(catalog, MCPToolCatalogItem{
			ToolName:       alias,
			ServerName:     cfg.displayName(),
			RemoteToolName: tool.Name,
			Description:    desc,
			PreserveSchema: cfg.PreserveSchema,
		})
	}
	return handlers, catalog
}

func buildMCPTransport(cfg MCPServerConfig) (mcp.Transport, error) {
	transport, _, err := buildMCPTransportWithHTTPClient(cfg, nil)
	return transport, err
}

func buildMCPTransportWithHTTPClient(cfg MCPServerConfig, existing *http.Client) (mcp.Transport, *http.Client, error) {
	switch cfg.Type {
	case "", "stdio":
		cmd := exec.Command(cfg.Command, cfg.Args...)
		if cfg.WorkDir != "" {
			cmd.Dir = cfg.WorkDir
		}
		if len(cfg.Env) > 0 {
			env := os.Environ()
			for k, v := range normalizeStringMap(cfg.Env) {
				env = append(env, k+"="+v)
			}
			cmd.Env = env
		}
		return &mcp.CommandTransport{Command: cmd}, nil, nil
	case "streamable", "http":
		transport := &mcp.StreamableClientTransport{Endpoint: cfg.URL}
		client := existing
		if client == nil {
			client = buildMCPHTTPClient(cfg.Headers)
		}
		transport.HTTPClient = client
		return transport, client, nil
	case "sse":
		transport := &mcp.SSEClientTransport{Endpoint: cfg.URL}
		client := existing
		if client == nil {
			client = buildMCPHTTPClient(cfg.Headers)
		}
		transport.HTTPClient = client
		return transport, client, nil
	default:
		return nil, nil, fmt.Errorf("unsupported MCP transport type %q", cfg.Type)
	}
}

func listMCPTools(ctx context.Context, session *mcp.ClientSession) ([]*mcp.Tool, error) {
	var tools []*mcp.Tool
	for tool, err := range session.Tools(ctx, nil) {
		if err != nil {
			return nil, err
		}
		if tool != nil {
			tools = append(tools, tool)
		}
	}
	return tools, nil
}

func readMCPServerConfigs() ([]MCPServerConfig, error) {
	// Dynamic MCP server settings are loaded from SQLite by MCPRegistry.
	return nil, nil
}

func readGitHubCopilotMCPServerConfigs() []MCPServerConfig {
	return nil
}

func inferMCPServerName(cfg MCPServerConfig, idx int) string {
	if cfg.Command != "" {
		return sanitizeMCPToolName(filepath.Base(cfg.Command))
	}
	if cfg.URL != "" {
		if u, err := url.Parse(cfg.URL); err == nil {
			if host := strings.TrimSpace(u.Hostname()); host != "" {
				return sanitizeMCPToolName(host)
			}
		}
	}
	return fmt.Sprintf("mcp_server_%d", idx+1)
}

func buildMCPToolAlias(cfg MCPServerConfig, remoteToolName string, usedNames map[string]struct{}) string {
	prefix := cfg.ToolNamePrefix
	if prefix == "" {
		prefix = "MCP_" + cfg.Name
	}
	base := sanitizeMCPToolName(prefix + "_" + remoteToolName)
	base = shortenMCPToolName(base)
	name := base
	for i := 2; ; i++ {
		if _, exists := usedNames[name]; !exists {
			usedNames[name] = struct{}{}
			return name
		}
		name = shortenMCPToolName(fmt.Sprintf("%s_%d", base, i))
	}
}

func sanitizeMCPToolName(name string) string {
	if name == "" {
		return "MCP_Tool"
	}
	var b strings.Builder
	b.Grow(len(name))
	prevUnderscore := false
	for _, r := range name {
		isLetter := (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z')
		isDigit := r >= '0' && r <= '9'
		if isLetter || isDigit {
			b.WriteRune(r)
			prevUnderscore = false
			continue
		}
		if !prevUnderscore {
			b.WriteByte('_')
			prevUnderscore = true
		}
	}
	out := strings.Trim(b.String(), "_")
	if out == "" {
		out = "MCP_Tool"
	}
	first := out[0]
	if first >= '0' && first <= '9' {
		out = "MCP_" + out
	}
	return out
}

func shortenMCPToolName(name string) string {
	if len(name) <= maxMCPToolNameLength {
		return name
	}
	h := fnv.New32a()
	_, _ = h.Write([]byte(name))
	suffix := fmt.Sprintf("%x", h.Sum32())
	keep := maxMCPToolNameLength - len(suffix) - 1
	if keep < 1 {
		keep = 1
	}
	return name[:keep] + "_" + suffix
}

func normalizeMCPInputSchema(raw any) map[string]interface{} {
	defaultSchema := map[string]interface{}{
		"type":       "object",
		"properties": map[string]interface{}{},
	}
	if raw == nil {
		return defaultSchema
	}

	var schema map[string]interface{}
	switch v := raw.(type) {
	case map[string]interface{}:
		schema = cloneMap(v)
	default:
		b, err := json.Marshal(v)
		if err != nil {
			return defaultSchema
		}
		if err := json.Unmarshal(b, &schema); err != nil {
			return defaultSchema
		}
	}

	if schema == nil {
		return defaultSchema
	}
	if t, ok := schema["type"].(string); !ok || strings.TrimSpace(t) == "" {
		schema["type"] = "object"
	}
	if _, ok := schema["properties"]; !ok {
		schema["properties"] = map[string]interface{}{}
	}
	return schema
}

func extractMCPText(contents []mcp.Content) string {
	lines := make([]string, 0, len(contents))
	for _, c := range contents {
		switch v := c.(type) {
		case *mcp.TextContent:
			if txt := strings.TrimSpace(v.Text); txt != "" {
				lines = append(lines, txt)
			}
		case *mcp.ResourceLink:
			if v.URI != "" {
				lines = append(lines, fmt.Sprintf("[resource] %s", v.URI))
			}
		case *mcp.EmbeddedResource:
			if v.Resource != nil {
				if txt := strings.TrimSpace(v.Resource.Text); txt != "" {
					lines = append(lines, txt)
				}
			}
		}
	}
	return strings.Join(lines, "\n")
}

func mcpContentAsAny(contents []mcp.Content) any {
	if len(contents) == 0 {
		return []interface{}{}
	}
	b, err := json.Marshal(contents)
	if err != nil {
		return nil
	}
	var out any
	if err := json.Unmarshal(b, &out); err != nil {
		return nil
	}
	return out
}

func stringSet(items []string) map[string]struct{} {
	if len(items) == 0 {
		return nil
	}
	out := make(map[string]struct{}, len(items))
	for _, item := range items {
		s := strings.TrimSpace(item)
		if s == "" {
			continue
		}
		out[s] = struct{}{}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func normalizeStringMap(in map[string]interface{}) map[string]string {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]string, len(in))
	for k, v := range in {
		key := strings.TrimSpace(k)
		if key == "" {
			continue
		}
		out[key] = fmt.Sprint(v)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func cloneMap(in map[string]interface{}) map[string]interface{} {
	if in == nil {
		return map[string]interface{}{}
	}
	out := make(map[string]interface{}, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func (cfg MCPServerConfig) connectTimeout() time.Duration {
	sec := cfg.ConnectTimeoutSec
	if sec <= 0 {
		sec = 20
	}
	return time.Duration(sec) * time.Second
}

func (cfg MCPServerConfig) callTimeout() time.Duration {
	sec := cfg.CallTimeoutSec
	if sec <= 0 {
		sec = 120
	}
	return time.Duration(sec) * time.Second
}

type headerInjectRoundTripper struct {
	base    http.RoundTripper
	headers map[string]string
}

func buildMCPHTTPClient(raw map[string]interface{}) *http.Client {
	headers := normalizeStringMap(raw)
	jar, _ := cookiejar.New(nil)
	client := &http.Client{
		Jar: jar,
	}
	if len(headers) == 0 {
		return client
	}
	client.Transport = &headerInjectRoundTripper{
		base:    http.DefaultTransport,
		headers: headers,
	}
	return client
}

func (rt *headerInjectRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	base := rt.base
	if base == nil {
		base = http.DefaultTransport
	}
	cloned := req.Clone(req.Context())
	for k, v := range rt.headers {
		cloned.Header.Set(k, v)
	}
	return base.RoundTrip(cloned)
}
