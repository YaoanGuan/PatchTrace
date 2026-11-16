package llm

import "strings"

func IsContextLimitError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(strings.TrimSpace(err.Error()))
	if msg == "" {
		return false
	}
	return strings.Contains(msg, "maximum context length") ||
		strings.Contains(msg, "maximum context") ||
		strings.Contains(msg, "context length exceeded") ||
		strings.Contains(msg, "context_length_exceeded") ||
		strings.Contains(msg, "context window") ||
		strings.Contains(msg, "too many tokens") ||
		strings.Contains(msg, "input tokens") ||
		strings.Contains(msg, "prompt is too long") ||
		strings.Contains(msg, "request too large") ||
		strings.Contains(msg, "reduce the length of the messages")
}
