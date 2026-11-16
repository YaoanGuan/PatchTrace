package toolCalling

import (
	"AIxVuln/llm"
	"AIxVuln/misc"
	"AIxVuln/prompts"
	"AIxVuln/skill"
	"AIxVuln/taskManager"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
)

const (
	skillMarkdownSummaryMethodGeneralLLM = "general_llm_config"
	skillMarkdownSummaryMethodHeuristic  = "heuristic_fallback"
	maxSkillMarkdownSummaryRunes         = 1600
)

type skillMarkdownSummary struct {
	Content string
	Method  string
}

var (
	readSkillChunkFunc     = skill.ReadFile
	readSkillDocumentFunc  = skill.ReadDocument
	summarizeSkillMarkdown = summarizeSkillMarkdownContent

	skillMarkdownSummaryCache sync.Map

	numberedListPattern = regexp.MustCompile(`^\d+\.\s+`)
)

func loadSkillChunkForAgent(task *taskManager.Task, skillName string, relativePath string, offset int, limit int) (*skill.FileChunk, error) {
	if !shouldSummarizeSkillMarkdown(relativePath) {
		return readSkillChunkFunc(skillName, relativePath, offset, limit)
	}
	doc, err := readSkillDocumentFunc(skillName, relativePath)
	if err != nil {
		return nil, err
	}
	summary := summarizeSkillMarkdown(task, doc.SkillName, doc.RelativePath, doc.Content)
	return buildSkillChunkFromText(doc.SkillName, doc.RelativePath, summary.Content, offset, limit, len([]rune(doc.Content)), summary.Method), nil
}

func shouldSummarizeSkillMarkdown(relativePath string) bool {
	rel := strings.TrimSpace(relativePath)
	if rel == "" {
		rel = "SKILL.md"
	}
	ext := strings.ToLower(filepath.Ext(rel))
	return ext == ".md" || ext == ".markdown"
}

func buildSkillChunkFromText(skillName string, relativePath string, content string, offset int, limit int, originalTotalChars int, method string) *skill.FileChunk {
	if offset < 0 {
		offset = 0
	}
	if limit <= 0 {
		limit = 12000
	}
	if limit > 30000 {
		limit = 30000
	}
	runes := []rune(content)
	if offset > len(runes) {
		offset = len(runes)
	}
	end := offset + limit
	if end > len(runes) {
		end = len(runes)
	}
	chunk := &skill.FileChunk{
		SkillName:    skillName,
		RelativePath: relativePath,
		Offset:       offset,
		Limit:        limit,
		NextOffset:   end,
		TotalChars:   len(runes),
		HasMore:      end < len(runes),
		Content:      string(runes[offset:end]),
	}
	if method != "" {
		chunk.Summarized = true
		chunk.SummaryMethod = method
		chunk.OriginalTotalChars = originalTotalChars
	}
	return chunk
}

func summarizeSkillMarkdownContent(task *taskManager.Task, skillName string, relativePath string, raw string) skillMarkdownSummary {
	raw = strings.ReplaceAll(raw, "\r\n", "\n")
	raw = strings.ReplaceAll(raw, "\r", "\n")
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return skillMarkdownSummary{}
	}
	cacheKey := buildSkillMarkdownSummaryCacheKey(skillName, relativePath, raw)
	if cached, ok := skillMarkdownSummaryCache.Load(cacheKey); ok {
		return cached.(skillMarkdownSummary)
	}
	if result, ok := summarizeSkillMarkdownWithGeneralLLM(task, skillName, relativePath, raw); ok {
		skillMarkdownSummaryCache.Store(cacheKey, result)
		return result
	}
	result := skillMarkdownSummary{
		Content: compactSkillSummaryText(heuristicSkillMarkdownSummary(raw)),
		Method:  skillMarkdownSummaryMethodHeuristic,
	}
	if result.Content == "" {
		result.Content = compactSkillSummaryText(raw)
	}
	skillMarkdownSummaryCache.Store(cacheKey, result)
	return result
}

func buildSkillMarkdownSummaryCacheKey(skillName string, relativePath string, raw string) string {
	model := strings.TrimSpace(misc.GetConfigValueDefault("general", "MODEL", misc.GetConfigValueDefault("main_setting", "MODEL", "")))
	sum := sha256.Sum256([]byte("v1\n" + model + "\n" + skillName + "\n" + relativePath + "\n" + raw))
	return hex.EncodeToString(sum[:])
}

func summarizeSkillMarkdownWithGeneralLLM(task *taskManager.Task, skillName string, relativePath string, raw string) (skillMarkdownSummary, bool) {
	model := strings.TrimSpace(misc.GetConfigValueDefault("general", "MODEL", misc.GetConfigValueDefault("main_setting", "MODEL", "")))
	if model == "" {
		return skillMarkdownSummary{}, false
	}
	cli := llm.GetResponsesClient("general", "main_setting")
	if cli == nil {
		return skillMarkdownSummary{}, false
	}

	systemPrompt := prompts.SkillMarkdownSummarySystemPrompt()
	userPrompt := fmt.Sprintf("Skill: %s\nFile: %s\n\nMarkdown:\n%s", skillName, relativePath, raw)
	msgs := []llm.Message{
		{Role: llm.RoleSystem, Content: systemPrompt},
		{Role: llm.RoleUser, Content: userPrompt},
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	projectName := ""
	if task != nil {
		projectName = task.GetProjectName()
	}
	resp, err := llm.RequestLLMWithOpts(cli, ctx, model, msgs, nil, llm.RequestLLMOpts{
		ProjectName: projectName,
		AgentLabel:  "skills-markdown-summary",
	})
	if err != nil {
		misc.Debug("ReadSkillTool markdown summary failed, fallback to heuristic: %v", err)
		return skillMarkdownSummary{}, false
	}
	content := compactSkillSummaryText(resp.Content)
	if content == "" {
		return skillMarkdownSummary{}, false
	}
	return skillMarkdownSummary{
		Content: content,
		Method:  skillMarkdownSummaryMethodGeneralLLM,
	}, true
}

func compactSkillSummaryText(raw string) string {
	raw = strings.ReplaceAll(raw, "\r\n", "\n")
	raw = strings.ReplaceAll(raw, "\r", "\n")
	lines := strings.Split(raw, "\n")
	out := make([]string, 0, len(lines))
	lastBlank := true
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			if lastBlank {
				continue
			}
			out = append(out, "")
			lastBlank = true
			continue
		}
		line = strings.TrimPrefix(line, "#")
		line = strings.TrimSpace(line)
		line = strings.ReplaceAll(line, "```", "")
		if line == "" {
			continue
		}
		out = append(out, line)
		lastBlank = false
	}
	return truncateRunes(strings.TrimSpace(strings.Join(out, "\n")), maxSkillMarkdownSummaryRunes)
}

func heuristicSkillMarkdownSummary(raw string) string {
	lines := strings.Split(raw, "\n")
	out := make([]string, 0, len(lines))
	inFence := false
	lastBlank := true
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "```") {
			inFence = !inFence
			continue
		}
		if trimmed == "" {
			if lastBlank {
				continue
			}
			out = append(out, "")
			lastBlank = true
			continue
		}
		if inFence {
			if !looksLikeImportantSkillCommand(trimmed) {
				continue
			}
		} else if !looksLikeImportantSkillLine(trimmed) {
			continue
		}
		out = append(out, trimmed)
		lastBlank = false
	}
	return strings.Join(out, "\n")
}

func looksLikeImportantSkillLine(line string) bool {
	if strings.HasPrefix(line, "#") {
		return true
	}
	if strings.HasPrefix(line, "- ") || strings.HasPrefix(line, "* ") || numberedListPattern.MatchString(line) {
		return true
	}
	lower := strings.ToLower(line)
	keywords := []string{
		"适用场景", "使用流程", "步骤", "前提", "命令", "文件", "脚本", "读取", "下载",
		"scripts/", "references/", "assets/", "skill.md", ".sh", ".py", ".js", ".so",
		"adb ", "frida", "gdb", "curl ", "http://", "https://", "/data/", "/sourcecodedir",
	}
	for _, kw := range keywords {
		if strings.Contains(lower, kw) {
			return true
		}
	}
	return len([]rune(line)) <= 90 && utf8.RuneCountInString(line) > 0 && strings.Contains(line, "`")
}

func looksLikeImportantSkillCommand(line string) bool {
	lower := strings.ToLower(line)
	prefixes := []string{
		"adb ", "am ", "pm ", "su ", "curl ", "wget ", "frida", "gdb", "find ",
		"ps ", "ls ", "chmod ", "chown ", "python ", "python3 ", "sh ", "/",
		"call ", "set ",
	}
	for _, prefix := range prefixes {
		if strings.HasPrefix(lower, prefix) {
			return true
		}
	}
	return strings.Contains(lower, "/data/") || strings.Contains(lower, "http://") || strings.Contains(lower, "https://")
}

func truncateRunes(raw string, maxRunes int) string {
	if maxRunes <= 0 {
		return ""
	}
	runes := []rune(raw)
	if len(runes) <= maxRunes {
		return raw
	}
	return strings.TrimSpace(string(runes[:maxRunes])) + "\n[已截断]"
}
