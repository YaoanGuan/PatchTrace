package llm

import (
	"AIxVuln/misc"
	"strconv"
	"strings"
)

const (
	defaultMaxOutputTokensGeneral = 1024 * 10
	defaultMaxOutputTokensReport  = 0
	maxConfiguredOutputTokens     = 1024 * 10
)

func ConfiguredMaxOutputTokens(section string) int64 {
	if strings.EqualFold(strings.TrimSpace(section), "report") {
		return 0
	}
	raw := strings.TrimSpace(misc.GetConfigValueDefault(section, "MaxOutputTokens",
		misc.GetConfigValueDefault("main_setting", "MaxOutputTokens", "")))
	if raw != "" {
		if n, err := strconv.Atoi(raw); err == nil && n > 0 {
			if n > maxConfiguredOutputTokens {
				n = maxConfiguredOutputTokens
			}
			return int64(n)
		}
	}
	return int64(defaultMaxOutputTokensForSection(section))
}

func defaultMaxOutputTokensForSection(section string) int {
	switch strings.ToLower(strings.TrimSpace(section)) {
	case "report":
		return defaultMaxOutputTokensReport
	default:
		return defaultMaxOutputTokensGeneral
	}
}
