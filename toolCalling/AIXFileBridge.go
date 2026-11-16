package toolCalling

import (
	"AIxVuln/misc"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"
)

const (
	aixFileTagOpen  = "<aixfile>"
	aixFileTagClose = "</aixfile>"
)

func rewriteToolArgsWithAIXFiles(args map[string]interface{}, sourceRoot string) (map[string]interface{}, error) {
	if len(args) == 0 || strings.TrimSpace(sourceRoot) == "" {
		return args, nil
	}
	out := make(map[string]interface{}, len(args))
	for k, v := range args {
		rewritten, err := rewriteToolValueWithAIXFiles(v, sourceRoot)
		if err != nil {
			return nil, err
		}
		out[k] = rewritten
	}
	return out, nil
}

func rewriteToolValueWithAIXFiles(value interface{}, sourceRoot string) (interface{}, error) {
	switch v := value.(type) {
	case string:
		return rewriteToolStringWithAIXFiles(v, sourceRoot)
	case []interface{}:
		out := make([]interface{}, len(v))
		for i, item := range v {
			rewritten, err := rewriteToolValueWithAIXFiles(item, sourceRoot)
			if err != nil {
				return nil, err
			}
			out[i] = rewritten
		}
		return out, nil
	case map[string]interface{}:
		out := make(map[string]interface{}, len(v))
		for k, item := range v {
			rewritten, err := rewriteToolValueWithAIXFiles(item, sourceRoot)
			if err != nil {
				return nil, err
			}
			out[k] = rewritten
		}
		return out, nil
	default:
		return value, nil
	}
}

func rewriteToolStringWithAIXFiles(raw string, sourceRoot string) (string, error) {
	if strings.TrimSpace(raw) == "" || strings.TrimSpace(sourceRoot) == "" || !strings.Contains(raw, aixFileTagOpen) {
		return raw, nil
	}
	var out strings.Builder
	cursor := 0
	for cursor < len(raw) {
		openIdx := strings.Index(raw[cursor:], aixFileTagOpen)
		if openIdx < 0 {
			out.WriteString(raw[cursor:])
			break
		}
		openIdx += cursor
		out.WriteString(raw[cursor:openIdx])
		contentStart := openIdx + len(aixFileTagOpen)
		closeRel := strings.Index(raw[contentStart:], aixFileTagClose)
		if closeRel < 0 {
			return "", fmt.Errorf("unclosed <aixfile> tag")
		}
		closeIdx := contentStart + closeRel
		ref := raw[contentStart:closeIdx]
		fileContent, err := loadAIXFileContent(ref, sourceRoot)
		if err != nil {
			return "", err
		}
		out.WriteString(fileContent)
		cursor = closeIdx + len(aixFileTagClose)
	}
	return out.String(), nil
}

func loadAIXFileContent(ref string, sourceRoot string) (string, error) {
	targetPath, err := resolveAIXFileReference(ref, sourceRoot)
	if err != nil {
		return "", err
	}
	data, err := os.ReadFile(targetPath)
	if err != nil {
		return "", fmt.Errorf("read <aixfile> %q failed: %w", strings.TrimSpace(ref), err)
	}
	if !utf8.Valid(data) {
		return "", fmt.Errorf("<aixfile>%s</aixfile> is not valid UTF-8 text; use aix-mcp://... when the tool expects a file path instead of inline file content", strings.TrimSpace(ref))
	}
	return string(data), nil
}

func resolveAIXFileReference(ref string, sourceRoot string) (string, error) {
	root := filepath.Clean(strings.TrimSpace(sourceRoot))
	if root == "" {
		return "", fmt.Errorf("source root is empty")
	}
	value := strings.TrimSpace(ref)
	if value == "" {
		return "", fmt.Errorf("empty <aixfile> reference")
	}
	value = misc.NormalizeVisibleSourcePath(value)
	candidate := filepath.Clean(filepath.Join(root, filepath.FromSlash(value)))
	if !pathWithinRoot(candidate, root) {
		return "", fmt.Errorf("<aixfile>%s</aixfile> escapes source root", strings.TrimSpace(ref))
	}
	return candidate, nil
}
