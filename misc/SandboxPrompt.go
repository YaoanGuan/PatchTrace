package misc

import (
	"sort"
	"strconv"
	"strings"
)

const SandboxListenerFallbackInstruction = "- When you need to run listening services (e.g., nc to receive a reverse shell) in the sandbox, use only the runtime-assigned mapped host callback IP and the explicitly assigned listener ports. Never use the sandbox/container private IP as a reverse-shell callback target. When operating the C2, do not use these listener ports for forwarding or proxy ports."

func NormalizeSandboxListenerPorts(ports []string) []string {
	normalized := make([]string, 0, len(ports))
	seen := make(map[string]struct{}, len(ports))
	for _, port := range ports {
		port = strings.TrimSpace(port)
		if port == "" {
			continue
		}
		if _, exists := seen[port]; exists {
			continue
		}
		seen[port] = struct{}{}
		normalized = append(normalized, port)
	}
	sort.Slice(normalized, func(i, j int) bool {
		pi, errI := strconv.Atoi(normalized[i])
		pj, errJ := strconv.Atoi(normalized[j])
		if errI == nil && errJ == nil {
			return pi < pj
		}
		return normalized[i] < normalized[j]
	})
	return normalized
}

func RenderSandboxPromptTemplate(raw string, host string, ports []string) string {
	raw = strings.TrimSpace(raw)
	host = strings.TrimSpace(host)
	ports = NormalizeSandboxListenerPorts(ports)
	if raw == "" {
		return ""
	}
	out := raw
	if host != "" {
		out = strings.ReplaceAll(out, "<serverip>", host)
	}
	for i := 0; i < len(ports); i++ {
		out = strings.Replace(out, "xxxxx", ports[i], 1)
	}
	return out
}

func BuildSandboxListenerInstruction(raw string, host string, ports []string) string {
	host = strings.TrimSpace(host)
	ports = NormalizeSandboxListenerPorts(ports)
	if host == "" || len(ports) < 3 {
		return SandboxListenerFallbackInstruction
	}
	return RenderSandboxPromptTemplate(raw, host, ports)
}
