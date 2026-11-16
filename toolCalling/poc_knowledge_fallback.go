package toolCalling

import (
	resourcepack "AIxVuln/resource"
	"fmt"
	"path"
	"strings"
)

const (
	pocKnowledgeFallbackMaxHits = 6
)

func fallbackPocKnowledgeAnswer(question string) (string, error) {
	question = strings.TrimSpace(question)
	if question == "" {
		return "", fmt.Errorf("question is required")
	}

	hits, err := collectFallbackPocKnowledgeHits(question)
	if err != nil {
		return "", err
	}
	if len(hits) == 0 {
		return strings.TrimSpace(`结论
本地 PoC 知识库里没有检索到与该问题直接相关的命中资料。

建议
- 用已经确认的产品名、组件名、版本号、接口路径或报错特征重新查询。
- 如果产品尚未确认，不要猜 CMS/厂商名；改查通用漏洞方向，如认证绕过、SQL 注入、文件上传、反序列化等。`), nil
	}

	docTopics := make([]string, 0, 4)
	seenDocs := make(map[string]struct{}, len(hits))
	for _, hit := range hits {
		topic := shortPocKnowledgeTopic(hit.RelativePath)
		if topic == "" {
			continue
		}
		if _, ok := seenDocs[topic]; ok {
			continue
		}
		seenDocs[topic] = struct{}{}
		docTopics = append(docTopics, topic)
		if len(docTopics) >= 4 {
			break
		}
	}

	var b strings.Builder
	b.WriteString("结论\n")
	b.WriteString("PoC 知识助手未在时限内完成多轮总结，以下为本地知识库直接检索命中，可先据此继续验证。\n\n")
	b.WriteString("命中线索\n")
	for i, hit := range hits {
		snippet := strings.TrimSpace(hit.Snippet)
		if snippet == "" {
			continue
		}
		b.WriteString(fmt.Sprintf("%d. [%s] %s\n", i+1, shortPocKnowledgeTopic(hit.RelativePath), snippet))
	}
	if len(docTopics) > 0 {
		b.WriteString("\n命中资料主题\n")
		for _, topic := range docTopics {
			b.WriteString("- " + topic + "\n")
		}
	}
	b.WriteString("\n注意点\n")
	b.WriteString("- 以上是直接检索结果，不等于已确认漏洞可利用。\n")
	b.WriteString("- 如果当前产品名还没有被页面标题、接口名、包名、文件路径、指纹或厂商字符串证实，不要把猜测的产品名当成事实继续扩散。\n")
	return strings.TrimSpace(b.String()), nil
}

func collectFallbackPocKnowledgeHits(question string) ([]resourcepack.PocKnowledgeDocSearchHit, error) {
	queries := buildFallbackPocKnowledgeQueries(question)
	allHits := make([]resourcepack.PocKnowledgeDocSearchHit, 0, pocKnowledgeFallbackMaxHits)
	seen := make(map[string]struct{}, pocKnowledgeFallbackMaxHits*2)
	for _, query := range queries {
		hits, err := resourcepack.SearchPocKnowledgeDocs(query, "", pocKnowledgeFallbackMaxHits)
		if err != nil {
			continue
		}
		for _, hit := range hits {
			key := hit.RelativePath + "\n" + fmt.Sprintf("%d", hit.Line) + "\n" + hit.Snippet
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			allHits = append(allHits, hit)
			if len(allHits) >= pocKnowledgeFallbackMaxHits {
				return allHits, nil
			}
		}
	}
	return allHits, nil
}

func buildFallbackPocKnowledgeQueries(question string) []string {
	question = strings.Join(strings.Fields(strings.TrimSpace(question)), " ")
	if question == "" {
		return nil
	}
	queries := []string{question}
	genericTerms := extractGenericVulnTerms(question)
	if genericTerms != "" && genericTerms != question {
		queries = append(queries, genericTerms)
	}
	productTerms := extractSpecificProductTerms(question)
	if productTerms != "" && productTerms != question && productTerms != genericTerms {
		queries = append(queries, productTerms)
	}
	return uniqueNonEmptyStrings(queries)
}

func extractGenericVulnTerms(question string) string {
	fields := strings.Fields(strings.ToLower(question))
	if len(fields) == 0 {
		return ""
	}
	keepSet := map[string]struct{}{
		"0day":            {},
		"0-day":           {},
		"auth":            {},
		"authentication":  {},
		"bypass":          {},
		"cve":             {},
		"deserialization": {},
		"deserialize":     {},
		"exploit":         {},
		"file":            {},
		"getshell":        {},
		"injection":       {},
		"lfi":             {},
		"overflow":        {},
		"path":            {},
		"poc":             {},
		"rce":             {},
		"sqli":            {},
		"sql":             {},
		"ssrf":            {},
		"traversal":       {},
		"upload":          {},
		"vuln":            {},
		"vulnerability":   {},
		"漏洞":              {},
		"利用":              {},
		"文件上传":            {},
		"认证绕过":            {},
		"未授权":             {},
		"反序列化":            {},
		"命令执行":            {},
		"注入":              {},
	}
	out := make([]string, 0, len(fields))
	seen := make(map[string]struct{}, len(fields))
	for _, field := range fields {
		field = strings.Trim(field, " ,.;:!?'\"()[]{}<>|/\\")
		if field == "" {
			continue
		}
		if _, ok := keepSet[field]; !ok {
			continue
		}
		if _, ok := seen[field]; ok {
			continue
		}
		seen[field] = struct{}{}
		out = append(out, field)
	}
	return strings.Join(out, " ")
}

func extractSpecificProductTerms(question string) string {
	fields := strings.Fields(strings.TrimSpace(question))
	if len(fields) == 0 {
		return ""
	}
	stopSet := map[string]struct{}{
		"known":           {},
		"vulnerabilities": {},
		"vulnerability":   {},
		"cve":             {},
		"cves":            {},
		"rce":             {},
		"sql":             {},
		"sqli":            {},
		"injection":       {},
		"exploit":         {},
		"poc":             {},
		"getshell":        {},
		"auth":            {},
		"authentication":  {},
		"bypass":          {},
		"file":            {},
		"upload":          {},
		"server":          {},
		"management":      {},
		"panel":           {},
		"cms":             {},
	}
	out := make([]string, 0, 4)
	seen := make(map[string]struct{}, 4)
	for _, raw := range fields {
		field := strings.Trim(strings.ToLower(raw), " ,.;:!?'\"()[]{}<>|/\\")
		if field == "" {
			continue
		}
		if _, ok := stopSet[field]; ok {
			continue
		}
		if len(field) < 3 {
			continue
		}
		if _, ok := seen[field]; ok {
			continue
		}
		seen[field] = struct{}{}
		out = append(out, field)
		if len(out) >= 4 {
			break
		}
	}
	return strings.Join(out, " ")
}

func shortPocKnowledgeTopic(relativePath string) string {
	relativePath = strings.TrimSpace(relativePath)
	if relativePath == "" {
		return "未知资料"
	}
	base := path.Base(relativePath)
	base = strings.TrimSuffix(base, path.Ext(base))
	base = strings.TrimSpace(base)
	if base != "" {
		return base
	}
	return relativePath
}

func uniqueNonEmptyStrings(items []string) []string {
	out := make([]string, 0, len(items))
	seen := make(map[string]struct{}, len(items))
	for _, item := range items {
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
	return out
}
