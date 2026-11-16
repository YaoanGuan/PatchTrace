package misc

import (
	"encoding/json"
	"fmt"
	"strings"
)

// RepairJSONStringLiterals escapes raw control characters that accidentally
// appear inside JSON string literals, which commonly happens when an LLM emits
// multiline tool arguments without escaping newlines.
func RepairJSONStringLiterals(raw string) (string, bool) {
	if raw == "" {
		return raw, false
	}
	var b strings.Builder
	b.Grow(len(raw) + 16)
	inString := false
	escaped := false
	changed := false
	for i := 0; i < len(raw); i++ {
		c := raw[i]
		if inString {
			if escaped {
				b.WriteByte(c)
				escaped = false
				continue
			}
			switch c {
			case '\\':
				b.WriteByte(c)
				escaped = true
			case '"':
				b.WriteByte(c)
				inString = false
			case '\n':
				b.WriteString(`\n`)
				changed = true
			case '\r':
				b.WriteString(`\r`)
				changed = true
			case '\t':
				b.WriteString(`\t`)
				changed = true
			case '\b':
				b.WriteString(`\b`)
				changed = true
			case '\f':
				b.WriteString(`\f`)
				changed = true
			default:
				if c < 0x20 {
					fmt.Fprintf(&b, "\\u%04x", c)
					changed = true
				} else {
					b.WriteByte(c)
				}
			}
			continue
		}
		b.WriteByte(c)
		if c == '"' {
			inString = true
		}
	}
	if !changed {
		return raw, false
	}
	return b.String(), true
}

// UnmarshalJSONLenient first attempts normal JSON parsing, then retries after
// escaping raw control characters inside string literals.
func UnmarshalJSONLenient(raw string, dst interface{}) error {
	if strings.TrimSpace(raw) == "" {
		return json.Unmarshal([]byte(`{}`), dst)
	}
	if err := json.Unmarshal([]byte(raw), dst); err == nil {
		return nil
	}
	repaired, changed := RepairJSONStringLiterals(raw)
	if !changed {
		return json.Unmarshal([]byte(raw), dst)
	}
	return json.Unmarshal([]byte(repaired), dst)
}
