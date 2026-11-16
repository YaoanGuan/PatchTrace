package patchintel

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

// FormatRules renders the top-k rules injected into an inversion prompt.
// It is shared by the inversion pipeline and the dispatch bootstrap task so
// both render the rule context identically.
func FormatRules(rules []Rule) string {
	if len(rules) == 0 {
		return "（规则库无命中，使用场景模板冷启动兜底；本次验证成功的新模式请通过 PromoteRuleTool 沉淀）"
	}
	var b strings.Builder
	for i, rule := range rules {
		fmt.Fprintf(&b, "[规则 %d] id=%d version=%d confidence=%.2f\n", i+1, rule.ID, rule.Version, rule.Confidence)
		b.WriteString("模式: " + rule.Pattern + "\n")
		if rule.TriggerFeatures != "" {
			b.WriteString("触发特征: " + rule.TriggerFeatures + "\n")
		}
		if rule.VerificationNotes != "" {
			b.WriteString("验证要点: " + rule.VerificationNotes + "\n")
		}
		if rule.CVEID != "" {
			b.WriteString("来源: " + rule.CVEID + "\n")
		}
		b.WriteString("\n")
	}
	return strings.TrimSpace(b.String())
}

// TruncateRunes caps raw at maxRunes runes and appends the diff budget marker.
// A non-positive maxRunes means "no limit".
func TruncateRunes(raw string, maxRunes int) string {
	if maxRunes <= 0 || utf8.RuneCountInString(raw) <= maxRunes {
		return raw
	}
	runes := []rune(raw)
	return string(runes[:maxRunes]) + "\n[补丁 diff 已按 token 预算截断]"
}

// MetadataOrUnknown renders an optional patch metadata field, using the same
// placeholder everywhere a profile is shown to a model or a human.
func MetadataOrUnknown(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "未知"
	}
	return value
}
