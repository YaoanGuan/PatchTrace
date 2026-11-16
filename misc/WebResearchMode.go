package misc

import (
	"fmt"
	"strings"
	"sync"
)

const (
	WebResearchModeAudit         = "audit"
	WebResearchModeComprehensive = "comprehensive"
)

var (
	runtimeWebResearchMode   string
	runtimeWebResearchModeMu sync.RWMutex
)

func NormalizeWebResearchMode(raw string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "", WebResearchModeAudit, "whitebox", "full", "审计", "白盒":
		return WebResearchModeAudit, nil
	case WebResearchModeComprehensive, "combined", "hybrid", "综合":
		return WebResearchModeComprehensive, nil
	default:
		return "", fmt.Errorf("unsupported web research mode %q, expected audit or comprehensive", raw)
	}
}

func SetRuntimeWebResearchMode(raw string) error {
	mode, err := NormalizeWebResearchMode(raw)
	if err != nil {
		return err
	}
	runtimeWebResearchModeMu.Lock()
	defer runtimeWebResearchModeMu.Unlock()
	runtimeWebResearchMode = mode
	return nil
}

func ClearRuntimeWebResearchMode() {
	runtimeWebResearchModeMu.Lock()
	defer runtimeWebResearchModeMu.Unlock()
	runtimeWebResearchMode = ""
}

func GetRuntimeWebResearchMode() string {
	runtimeWebResearchModeMu.RLock()
	defer runtimeWebResearchModeMu.RUnlock()
	return strings.TrimSpace(runtimeWebResearchMode)
}

func GetEffectiveWebResearchMode() string {
	if runtime := GetRuntimeWebResearchMode(); runtime != "" {
		return runtime
	}
	return WebResearchModeAudit
}
