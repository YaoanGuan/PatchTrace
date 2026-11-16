package misc

import (
	"fmt"
	"strings"
	"sync"
)

const (
	AutomationModeOff  = "off"
	AutomationModeFull = "full"

	AutomationScopeHostCommandApproval = "host_command_approval"
	AutomationScopeDecisionApproval    = "decision_approval"
	AutomationScopeAbandonIdeaDecision = "abandon_idea_decision"

	automationSourceRuntime = "runtime_flag"
	automationSourceConfig  = "config"
)

type AutomationDecision struct {
	Enabled  bool   `json:"enabled"`
	Mode     string `json:"mode,omitempty"`
	Source   string `json:"source,omitempty"`
	Scope    string `json:"scope,omitempty"`
	Reviewer string `json:"reviewer,omitempty"`
	Note     string `json:"note,omitempty"`
}

var (
	runtimeAutomationMode   string
	runtimeAutomationModeMu sync.RWMutex
)

func NormalizeAutomationMode(raw string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "", AutomationModeOff, "false", "0", "manual", "disabled":
		return AutomationModeOff, nil
	case AutomationModeFull, "true", "1", "auto", "on":
		return AutomationModeFull, nil
	default:
		return "", fmt.Errorf("unsupported automation mode %q, expected off or full", raw)
	}
}

func SetRuntimeAutomationMode(raw string) error {
	mode, err := NormalizeAutomationMode(raw)
	if err != nil {
		return err
	}
	runtimeAutomationModeMu.Lock()
	defer runtimeAutomationModeMu.Unlock()
	runtimeAutomationMode = mode
	return nil
}

func ClearRuntimeAutomationMode() {
	runtimeAutomationModeMu.Lock()
	defer runtimeAutomationModeMu.Unlock()
	runtimeAutomationMode = ""
}

func GetRuntimeAutomationMode() string {
	runtimeAutomationModeMu.RLock()
	defer runtimeAutomationModeMu.RUnlock()
	return strings.TrimSpace(runtimeAutomationMode)
}

func GetConfiguredAutomationMode() string {
	mode, _ := getConfiguredAutomationModeWithSource()
	return mode
}

func getConfiguredAutomationModeWithSource() (string, string) {
	mode, err := NormalizeAutomationMode(GetConfigValueDefault("misc", "AUTO_MODE", AutomationModeOff))
	if err != nil {
		return AutomationModeOff, automationSourceConfig
	}
	return mode, automationSourceConfig
}

func GetEffectiveAutomationMode() (string, string) {
	if runtime := GetRuntimeAutomationMode(); runtime != "" {
		return runtime, automationSourceRuntime
	}
	return getConfiguredAutomationModeWithSource()
}

func ResolveAutomationDecision(scope string) AutomationDecision {
	scope = strings.TrimSpace(scope)
	mode, source := GetEffectiveAutomationMode()
	if mode != AutomationModeFull {
		return AutomationDecision{
			Enabled: false,
			Mode:    mode,
			Source:  source,
			Scope:   scope,
		}
	}
	decision := AutomationDecision{
		Enabled:  true,
		Mode:     mode,
		Source:   source,
		Scope:    scope,
		Reviewer: "系统·全自动模式",
	}
	switch scope {
	case AutomationScopeHostCommandApproval:
		decision.Note = "全自动模式已自动批准宿主机命令执行"
	case AutomationScopeDecisionApproval:
		decision.Note = "全自动模式已自动批准该决策事项"
	case AutomationScopeAbandonIdeaDecision:
		decision.Note = "全自动模式已自动放弃该思路"
	default:
		decision.Note = "全自动模式已自动批准该操作"
	}
	if source == automationSourceRuntime {
		decision.Note += "（启动参数）"
	} else {
		decision.Note += "（设置项）"
	}
	return decision
}
