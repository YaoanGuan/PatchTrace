package toolCalling

import (
	"AIxVuln/misc"
	"AIxVuln/taskManager"
	"sort"
	"strings"
	"sync"
)

type MCPProfileConfig struct {
	Name        string
	Description string
	ServerNames []string
}

type MCPToolCatalogItem struct {
	ToolName       string `json:"tool_name"`
	ServerName     string `json:"server_name"`
	RemoteToolName string `json:"remote_tool_name"`
	Description    string `json:"description,omitempty"`
	PreserveSchema bool   `json:"preserve_schema,omitempty"`
}

type mcpServerRuntime struct {
	cfg      MCPServerConfig
	shared   *mcpSharedRuntime
	once     sync.Once
	handlers []ToolHandler
	catalog  []MCPToolCatalogItem
	err      error
}

func (r *mcpServerRuntime) load() ([]ToolHandler, []MCPToolCatalogItem, error) {
	r.once.Do(func() {
		if r.shared != nil {
			server, tools, err := r.shared.load()
			if err != nil {
				r.err = err
				return
			}
			r.handlers, r.catalog = buildMCPHandlersFromTools(r.cfg, server, tools, map[string]struct{}{})
			return
		}
		r.handlers, r.catalog, r.err = connectMCPServer(r.cfg, map[string]struct{}{})
	})
	handlers := make([]ToolHandler, len(r.handlers))
	copy(handlers, r.handlers)
	catalog := make([]MCPToolCatalogItem, len(r.catalog))
	copy(catalog, r.catalog)
	return handlers, catalog, r.err
}

type mcpRegistry struct {
	mu            sync.RWMutex
	serverConfigs map[string]MCPServerConfig
	servers       map[string]*mcpServerRuntime
	sharedServers map[string]*mcpSharedRuntime
	profiles      map[string]MCPProfileConfig
	agentProfiles map[string][]string
	toolMeta      map[string]MCPToolCatalogItem
}

var (
	globalMCPRegistryMu sync.Mutex
	globalMCPRegistry   *mcpRegistry
)

func newMCPRegistry() *mcpRegistry {
	r := &mcpRegistry{
		serverConfigs: make(map[string]MCPServerConfig),
		servers:       make(map[string]*mcpServerRuntime),
		sharedServers: make(map[string]*mcpSharedRuntime),
		profiles:      make(map[string]MCPProfileConfig),
		agentProfiles: make(map[string][]string),
		toolMeta:      make(map[string]MCPToolCatalogItem),
	}
	r.loadFromStore()
	return r
}

func getGlobalMCPRegistry() *mcpRegistry {
	globalMCPRegistryMu.Lock()
	defer globalMCPRegistryMu.Unlock()
	if globalMCPRegistry == nil {
		globalMCPRegistry = newMCPRegistry()
	}
	return globalMCPRegistry
}

func ReloadMCPRegistryFromStore() {
	globalMCPRegistryMu.Lock()
	globalMCPRegistry = newMCPRegistry()
	globalMCPRegistryMu.Unlock()
}

func (r *mcpRegistry) loadFromStore() {
	for _, item := range misc.GetMCPConfigs() {
		if !item.Enabled {
			continue
		}
		for _, binding := range item.AgentBindings {
			agentType := strings.TrimSpace(binding.AgentType)
			if agentType == "" {
				continue
			}
			cfg := buildAgentScopedMCPServerConfig(item, binding)
			if !r.registerServer(cfg) {
				continue
			}
			profileName := r.profileNameForBinding(item.ID, agentType)
			r.registerProfile(MCPProfileConfig{
				Name:        profileName,
				Description: item.Name,
				ServerNames: []string{cfg.Name},
			})
			r.bindAgentTypeProfile(agentType, profileName)
		}
	}
}

func (r *mcpRegistry) profileNameForConfig(configID string) string {
	configID = sanitizeMCPToolName(strings.TrimSpace(configID))
	if configID == "" {
		configID = "config"
	}
	return "mcp_profile_" + configID
}

func (r *mcpRegistry) profileNameForBinding(configID string, agentType string) string {
	configID = sanitizeMCPToolName(strings.TrimSpace(configID))
	agentType = sanitizeMCPToolName(strings.TrimSpace(agentType))
	if configID == "" {
		configID = "config"
	}
	if agentType == "" {
		agentType = "agent"
	}
	return "mcp_profile_" + configID + "__" + agentType
}

func buildAgentScopedMCPServerConfig(row misc.MCPConfigRow, binding misc.MCPAgentBinding) MCPServerConfig {
	agentType := strings.TrimSpace(binding.AgentType)
	prefix := strings.TrimSpace(row.ToolNamePrefix)
	if prefix == "" {
		baseID := sanitizeMCPToolName(strings.TrimSpace(row.ID))
		if baseID == "" {
			baseID = "config"
		}
		prefix = "MCP_" + baseID
	}
	cfg := MCPServerConfig{
		Name:                 scopedMCPRuntimeName(row.ID, agentType),
		SharedConnectionName: sharedMCPRuntimeName(row.ID),
		DisplayName:          row.Name,
		Type:                 row.Transport,
		Command:              row.Command,
		Args:                 append([]string(nil), row.Args...),
		WorkDir:              row.WorkDir,
		URL:                  row.URL,
		ToolNamePrefix:       prefix,
		PreserveSchema:       row.PreserveSchema,
		ConnectTimeoutSec:    row.ConnectTimeoutSec,
		CallTimeoutSec:       row.CallTimeoutSec,
	}
	if len(row.Env) > 0 {
		cfg.Env = make(map[string]interface{}, len(row.Env))
		for k, v := range row.Env {
			cfg.Env[k] = v
		}
	}
	if len(row.Headers) > 0 {
		cfg.Headers = make(map[string]interface{}, len(row.Headers))
		for k, v := range row.Headers {
			cfg.Headers[k] = v
		}
	}
	cfg.ToolAllowlist = effectiveMCPAllowlist(row.ToolAllowlist, binding.ToolAllowlist)
	cfg.ToolDenylist = mergeMCPFilters(row.ToolDenylist, binding.ToolDenylist)
	return cfg
}

func sharedMCPRuntimeName(configID string) string {
	configID = sanitizeMCPToolName(strings.TrimSpace(configID))
	if configID == "" {
		configID = "config"
	}
	return configID
}

func scopedMCPRuntimeName(configID string, agentType string) string {
	configID = sharedMCPRuntimeName(configID)
	agentType = sanitizeMCPToolName(strings.TrimSpace(agentType))
	if agentType == "" {
		agentType = "agent"
	}
	return configID + "__" + agentType
}

func effectiveMCPAllowlist(defaultAllowlist []string, agentAllowlist []string) []string {
	if len(agentAllowlist) > 0 {
		return append([]string(nil), agentAllowlist...)
	}
	return append([]string(nil), defaultAllowlist...)
}

func mergeMCPFilters(left []string, right []string) []string {
	seen := make(map[string]struct{}, len(left)+len(right))
	out := make([]string, 0, len(left)+len(right))
	for _, item := range append(append([]string(nil), left...), right...) {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		if _, ok := seen[item]; ok {
			continue
		}
		seen[item] = struct{}{}
		out = append(out, item)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func (r *mcpRegistry) registerServer(cfg MCPServerConfig) bool {
	cfg.Name = strings.TrimSpace(cfg.Name)
	if cfg.Enabled != nil && !*cfg.Enabled {
		return false
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if cfg.Name == "" {
		cfg.Name = inferMCPServerName(cfg, len(r.serverConfigs))
	}
	if cfg.SharedConnectionName == "" {
		cfg.SharedConnectionName = cfg.Name
	}
	if cfg.ToolNamePrefix == "" {
		cfg.ToolNamePrefix = "MCP_" + cfg.Name
	}
	r.serverConfigs[cfg.Name] = cfg
	for key := range r.servers {
		if runtimeKeyBelongsToMCPServer(key, cfg.Name) {
			delete(r.servers, key)
		}
	}
	for key := range r.sharedServers {
		if runtimeKeyBelongsToMCPServer(key, cfg.sharedConnectionNameOrDefault()) {
			delete(r.sharedServers, key)
		}
	}
	return true
}

func (r *mcpRegistry) registerProfile(cfg MCPProfileConfig) {
	name := strings.TrimSpace(cfg.Name)
	if name == "" {
		return
	}
	serverNames := make([]string, 0, len(cfg.ServerNames))
	seen := make(map[string]struct{}, len(cfg.ServerNames))
	for _, serverName := range cfg.ServerNames {
		serverName = strings.TrimSpace(serverName)
		if serverName == "" {
			continue
		}
		if _, ok := seen[serverName]; ok {
			continue
		}
		seen[serverName] = struct{}{}
		serverNames = append(serverNames, serverName)
	}
	cfg.Name = name
	cfg.ServerNames = serverNames
	r.mu.Lock()
	r.profiles[name] = cfg
	r.mu.Unlock()
}

func (r *mcpRegistry) bindAgentTypeProfile(agentType string, profileName string) {
	agentType = strings.TrimSpace(agentType)
	profileName = strings.TrimSpace(profileName)
	if agentType == "" || profileName == "" {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	existing := r.agentProfiles[agentType]
	for _, item := range existing {
		if item == profileName {
			return
		}
	}
	r.agentProfiles[agentType] = append(existing, profileName)
	sort.Strings(r.agentProfiles[agentType])
}

func RegisterMCPServer(cfg MCPServerConfig) {
	getGlobalMCPRegistry().registerServer(cfg)
}

func RegisterMCPProfile(cfg MCPProfileConfig) {
	getGlobalMCPRegistry().registerProfile(cfg)
}

func GetMCPToolHandlersForProfiles(profileNames ...string) []ToolHandler {
	return getGlobalMCPRegistry().handlersForProfiles(profileNames...)
}

func GetMCPToolHandlersForAgentType(agentType string) []ToolHandler {
	return getGlobalMCPRegistry().handlersForAgentType(agentType)
}

func GetMCPToolHandlersForAgentTypeWithProject(agentType string, projectName string) []ToolHandler {
	return getGlobalMCPRegistry().handlersForAgentTypeWithProject(agentType, projectName)
}

func RegisterConfiguredMCPProfilesForAgentType(client *ToolManager, agentType string) {
	if client == nil {
		return
	}
	client.RegisterHandlers(GetMCPToolHandlersForAgentType(agentType)...)
}

func ListMCPProfiles() []MCPProfileConfig {
	r := getGlobalMCPRegistry()
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]MCPProfileConfig, 0, len(r.profiles))
	for _, cfg := range r.profiles {
		item := MCPProfileConfig{
			Name:        cfg.Name,
			Description: cfg.Description,
			ServerNames: append([]string(nil), cfg.ServerNames...),
		}
		out = append(out, item)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Name < out[j].Name
	})
	return out
}

func ListMCPToolsForProfiles(profileNames ...string) []MCPToolCatalogItem {
	r := getGlobalMCPRegistry()
	_ = r.handlersForProfiles(profileNames...)
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]MCPToolCatalogItem, 0, len(r.toolMeta))
	for _, item := range r.toolMeta {
		out = append(out, item)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].ServerName != out[j].ServerName {
			return out[i].ServerName < out[j].ServerName
		}
		return out[i].ToolName < out[j].ToolName
	})
	return out
}

func ListMCPToolsForAgentType(agentType string) []MCPToolCatalogItem {
	return ListMCPToolsForProfiles(getGlobalMCPRegistry().profilesForAgentType(agentType)...)
}

func ShouldCompactMCPTool(name string) bool {
	if !strings.HasPrefix(name, "MCP_") {
		return false
	}
	r := getGlobalMCPRegistry()
	r.mu.RLock()
	defer r.mu.RUnlock()
	meta, ok := r.toolMeta[name]
	if !ok {
		return true
	}
	return !meta.PreserveSchema
}

func (r *mcpRegistry) handlersForAgentType(agentType string) []ToolHandler {
	return r.handlersForProfiles(r.profilesForAgentType(agentType)...)
}

func (r *mcpRegistry) handlersForAgentTypeWithProject(agentType string, projectName string) []ToolHandler {
	return r.handlersForProfilesWithProject(projectName, r.profilesForAgentType(agentType)...)
}

func (r *mcpRegistry) profilesForAgentType(agentType string) []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	items := append([]string(nil), r.agentProfiles[strings.TrimSpace(agentType)]...)
	sort.Strings(items)
	return items
}

func (r *mcpRegistry) handlersForProfiles(profileNames ...string) []ToolHandler {
	return r.handlersForProfilesWithProject("", profileNames...)
}

func (r *mcpRegistry) handlersForProfilesWithProject(projectName string, profileNames ...string) []ToolHandler {
	serverNames := r.resolveServerNames(profileNames...)
	if len(serverNames) == 0 {
		return nil
	}
	out := make([]ToolHandler, 0)
	for _, serverName := range serverNames {
		runtime := r.runtimeForServer(serverName, projectName)
		if runtime == nil {
			continue
		}
		handlers, catalog, err := runtime.load()
		if err != nil {
			misc.Debug("MCP server %s init failed: %v", serverName, err)
			continue
		}
		r.rememberToolCatalog(catalog)
		out = append(out, handlers...)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Name() < out[j].Name()
	})
	return out
}

func (r *mcpRegistry) runtimeForServer(serverName string, projectName string) *mcpServerRuntime {
	serverName = strings.TrimSpace(serverName)
	if serverName == "" {
		return nil
	}
	cacheKey := mcpRuntimeCacheKey(serverName, projectName)
	r.mu.Lock()
	defer r.mu.Unlock()
	if runtime := r.servers[cacheKey]; runtime != nil {
		return runtime
	}
	cfg, ok := r.serverConfigs[serverName]
	if !ok {
		return nil
	}
	sharedKey := mcpRuntimeCacheKey(cfg.sharedConnectionNameOrDefault(), projectName)
	shared := r.sharedServers[sharedKey]
	if shared == nil {
		shared = &mcpSharedRuntime{cfg: cfg.sharedConnectionConfig()}
		r.sharedServers[sharedKey] = shared
	}
	runtime := &mcpServerRuntime{cfg: cfg, shared: shared}
	r.servers[cacheKey] = runtime
	return runtime
}

func mcpRuntimeCacheKey(serverName string, projectName string) string {
	serverName = strings.TrimSpace(serverName)
	projectName = normalizeMCPRuntimeProject(projectName)
	if projectName == "" {
		return serverName
	}
	return serverName + "\x00" + projectName
}

func normalizeMCPRuntimeProject(projectName string) string {
	projectName = strings.TrimSpace(projectName)
	if projectName == "" {
		return ""
	}
	resolved := taskManager.ResolveRuntimeProjectName(projectName)
	if strings.TrimSpace(resolved) == "" {
		return projectName
	}
	return strings.TrimSpace(resolved)
}

func runtimeKeyBelongsToMCPServer(cacheKey string, serverName string) bool {
	serverName = strings.TrimSpace(serverName)
	if serverName == "" {
		return false
	}
	return cacheKey == serverName || strings.HasPrefix(cacheKey, serverName+"\x00")
}

func (r *mcpRegistry) resolveServerNames(profileNames ...string) []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	seen := make(map[string]struct{})
	out := make([]string, 0)
	for _, profileName := range profileNames {
		cfg, ok := r.profiles[strings.TrimSpace(profileName)]
		if !ok {
			continue
		}
		for _, serverName := range cfg.ServerNames {
			serverName = strings.TrimSpace(serverName)
			if serverName == "" {
				continue
			}
			if _, ok := r.serverConfigs[serverName]; !ok {
				continue
			}
			if _, ok := seen[serverName]; ok {
				continue
			}
			seen[serverName] = struct{}{}
			out = append(out, serverName)
		}
	}
	return out
}

func (r *mcpRegistry) rememberToolCatalog(items []MCPToolCatalogItem) {
	if len(items) == 0 {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, item := range items {
		if item.ToolName == "" {
			continue
		}
		r.toolMeta[item.ToolName] = item
	}
}
