package LabManager

import (
	"AIxVuln/llm"
	"AIxVuln/misc"
	"AIxVuln/prompts"
	"AIxVuln/taskManager"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	labSummaryRequestTimeout     = 120 * time.Second
	labSummaryInputBudgetLarge   = 22000
	labSummaryInputBudgetMedium  = 14000
	labSummaryInputBudgetSmall   = 9000
	labSummaryOverviewTokenLimit = 1200
	labSummaryExcerptTokenCap    = 1800
	labSummaryExcerptTokenFloor  = 280
	labSummaryOmittedMarker      = "\n\n[... 中间内容已省略 ...]\n\n"
	labSummaryAgentLabel         = "实验室总结"
)

type GeneratedSharedReport struct {
	ReportID   string `json:"reportId"`
	DownloadID string `json:"downloadId"`
	TargetID   string `json:"targetId,omitempty"`
	Summary    string `json:"summary"`
	CreatedAt  string `json:"createdAt"`
	FileName   string `json:"fileName,omitempty"`
	GroupID    string `json:"groupId"`
	GroupName  string `json:"groupName"`
	Content    string `json:"content"`
}

type summarySourceReport struct {
	GroupID   string
	GroupName string
	Meta      taskManager.ReportMeta
	Content   string
}

func (lm *LabManager) getLabReportCatalogSnapshot() []taskManager.ReportMeta {
	lm.labReportMu.RLock()
	defer lm.labReportMu.RUnlock()
	if len(lm.labReportCatalog) == 0 {
		return nil
	}
	out := make([]taskManager.ReportMeta, 0, len(lm.labReportCatalog))
	for _, report := range lm.labReportCatalog {
		path := strings.TrimSpace(report.Path)
		if path == "" {
			continue
		}
		item := report
		item.ReportID = strings.TrimSpace(item.ReportID)
		item.TargetID = strings.TrimSpace(item.TargetID)
		item.Path = path
		item.Summary = strings.TrimSpace(item.Summary)
		item.CreatedAt = strings.TrimSpace(item.CreatedAt)
		item.FileName = strings.TrimSpace(item.FileName)
		if item.FileName == "" {
			item.FileName = filepath.Base(item.Path)
		}
		out = append(out, item)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func (lm *LabManager) restoreLabReportCatalog(reports []taskManager.ReportMeta) {
	lm.labReportMu.Lock()
	defer lm.labReportMu.Unlock()
	if len(reports) == 0 {
		lm.labReportCatalog = nil
		return
	}
	out := make([]taskManager.ReportMeta, 0, len(reports))
	for i, report := range reports {
		path := strings.TrimSpace(report.Path)
		if path == "" {
			continue
		}
		item := report
		item.ReportID = strings.TrimSpace(item.ReportID)
		if item.ReportID == "" {
			item.ReportID = fmt.Sprintf("L.%d", i+1)
		}
		item.TargetID = strings.TrimSpace(item.TargetID)
		item.Path = path
		item.Summary = strings.TrimSpace(item.Summary)
		item.CreatedAt = strings.TrimSpace(item.CreatedAt)
		item.FileName = strings.TrimSpace(item.FileName)
		if item.FileName == "" {
			item.FileName = filepath.Base(item.Path)
		}
		out = append(out, item)
	}
	lm.labReportCatalog = out
}

func legacyLabReportCatalog(info taskManager.ProjectInfo) []taskManager.ReportMeta {
	appendReport := func(out []taskManager.ReportMeta, reportID string, path string) []taskManager.ReportMeta {
		path = strings.TrimSpace(path)
		if path == "" {
			return out
		}
		reportID = strings.TrimSpace(reportID)
		if reportID == "" {
			reportID = filepath.Base(path)
		}
		for _, existing := range out {
			if strings.TrimSpace(existing.ReportID) == reportID && strings.TrimSpace(existing.Path) == path {
				return out
			}
		}
		return append(out, taskManager.ReportMeta{
			ReportID: reportID,
			Path:     path,
			FileName: filepath.Base(path),
		})
	}
	out := make([]taskManager.ReportMeta, 0)
	for key, path := range info.ReportList {
		key = strings.TrimSpace(key)
		path = strings.TrimSpace(path)
		if key == "" || path == "" {
			continue
		}
		parts := strings.SplitN(key, ":", 2)
		if len(parts) == 2 && parts[0] == labControlGroupID {
			out = appendReport(out, parts[1], path)
		}
	}
	return out
}

func (lm *LabManager) GenerateSharedReportSummary() (GeneratedSharedReport, error) {
	sources := lm.collectSummarySourceReports()
	if len(sources) == 0 {
		return GeneratedSharedReport{}, fmt.Errorf("no shared reports available to summarize")
	}
	systemPrompt := prompts.SharedReportSummarySystemPrompt()
	cli := llm.GetResponsesClient("decision", "main_setting")
	model := misc.GetConfigValueDefault("decision", "MODEL", "")
	if strings.TrimSpace(model) == "" {
		model = misc.GetConfigValueDefault("main_setting", "MODEL", "")
	}
	budgets := []int{labSummaryInputBudgetLarge, labSummaryInputBudgetMedium, labSummaryInputBudgetSmall}
	var lastErr error
	for _, budget := range budgets {
		userPrompt := lm.buildSummaryPrompt(sources, budget)
		ctx, cancel := context.WithTimeout(context.Background(), labSummaryRequestTimeout)
		resp, err := llm.RequestLLMWithOpts(cli, ctx, model, []llm.Message{
			{Role: llm.RoleSystem, Content: systemPrompt},
			{Role: llm.RoleUser, Content: userPrompt},
		}, nil, llm.RequestLLMOpts{
			ProjectName: lm.labID,
			AgentLabel:  labSummaryAgentLabel,
		})
		cancel()
		if err != nil {
			lastErr = err
			if llm.IsContextLimitError(err) {
				continue
			}
			return GeneratedSharedReport{}, err
		}
		content := strings.TrimSpace(resp.Content)
		if content == "" {
			lastErr = fmt.Errorf("empty summary content")
			continue
		}
		if !strings.HasPrefix(content, "#") {
			content = "# 实验室总结\n\n" + content
		}
		report, err := lm.storeGeneratedSummaryReport(content, len(sources))
		if err != nil {
			return GeneratedSharedReport{}, err
		}
		lm.notifyGroupsUpdated()
		return GeneratedSharedReport{
			ReportID:   report.ReportID,
			DownloadID: sharedReportDownloadID(labControlGroupID, report.ReportID),
			TargetID:   report.TargetID,
			Summary:    report.Summary,
			CreatedAt:  report.CreatedAt,
			FileName:   report.FileName,
			GroupID:    labControlGroupID,
			GroupName:  labControlGroupName,
			Content:    content,
		}, nil
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("failed to generate summary report")
	}
	return GeneratedSharedReport{}, lastErr
}

func (lm *LabManager) collectSummarySourceReports() []summarySourceReport {
	out := make([]summarySourceReport, 0)
	for _, groupID := range lm.groupOrder {
		group, ok := lm.groups[groupID]
		if !ok || group == nil {
			continue
		}
		for _, report := range group.SharedReportCatalog() {
			path := strings.TrimSpace(report.Path)
			if path == "" {
				continue
			}
			body, err := os.ReadFile(path)
			if err != nil {
				continue
			}
			content := normalizeSummarySourceContent(string(body))
			if content == "" {
				continue
			}
			report.FileName = strings.TrimSpace(report.FileName)
			if report.FileName == "" {
				report.FileName = filepath.Base(path)
			}
			out = append(out, summarySourceReport{
				GroupID:   group.ID(),
				GroupName: group.Name(),
				Meta:      report,
				Content:   content,
			})
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		left := strings.TrimSpace(out[i].Meta.CreatedAt)
		right := strings.TrimSpace(out[j].Meta.CreatedAt)
		if left == right {
			if out[i].GroupName == out[j].GroupName {
				return strings.TrimSpace(out[i].Meta.ReportID) < strings.TrimSpace(out[j].Meta.ReportID)
			}
			return out[i].GroupName < out[j].GroupName
		}
		return left < right
	})
	return out
}

func normalizeSummarySourceContent(content string) string {
	content = strings.ReplaceAll(content, "\r\n", "\n")
	content = strings.ReplaceAll(content, "\r", "\n")
	return strings.TrimSpace(content)
}

func (lm *LabManager) buildSummaryPrompt(sources []summarySourceReport, inputBudget int) string {
	overview := truncatePrefixByTokens(normalizeSummarySourceContent(lm.ensureProjectOverview()), labSummaryOverviewTokenLimit)
	header := strings.TrimSpace(fmt.Sprintf(`项目名称: %s
用户任务: %s
项目速览:
%s

	请基于以下共享报告生成一份实验室层面的总结。要求：
1. 只依据已提供的报告内容，不要臆造不存在的结论。
2. 相同发现只保留一次，但可以指出哪些组互相印证了该发现。
3. 明确写出已确认、待确认、阻塞和下一步建议。
4. 输出仅限 Markdown 正文。`, lm.labName, strings.TrimSpace(lm.taskContent), fallbackText(overview, "暂无项目速览")))
	var builder strings.Builder
	builder.WriteString(header)
	builder.WriteString("\n\n共享报告节选如下：\n")
	currentTokens := llm.CountTokens(builder.String())
	perReportBudget := labSummaryExcerptTokenCap
	if len(sources) > 0 {
		if candidate := inputBudget / len(sources); candidate < perReportBudget {
			perReportBudget = candidate
		}
	}
	if perReportBudget < labSummaryExcerptTokenFloor {
		perReportBudget = labSummaryExcerptTokenFloor
	}
	for i, report := range sources {
		block := formatSummarySourceBlock(i+1, report, buildSummaryExcerpt(report.Content, perReportBudget))
		blockTokens := llm.CountTokens(block)
		if currentTokens+blockTokens > inputBudget {
			includedCurrent := false
			if currentTokens < inputBudget {
				minBlock := formatSummarySourceBlock(i+1, report, buildSummaryExcerpt(report.Content, labSummaryExcerptTokenFloor))
				minBlockTokens := llm.CountTokens(minBlock)
				if currentTokens+minBlockTokens <= inputBudget {
					builder.WriteString(minBlock)
					currentTokens += minBlockTokens
					includedCurrent = true
				}
			}
			omitted := len(sources) - i
			if includedCurrent {
				omitted--
			}
			if omitted > 0 {
				builder.WriteString(fmt.Sprintf("\n其余 %d 份报告因上下文预算限制未附节选，请在总结中仅基于已给出的材料下结论。\n", omitted))
			}
			break
		}
		builder.WriteString(block)
		currentTokens += blockTokens
	}
	return strings.TrimSpace(builder.String())
}

func formatSummarySourceBlock(index int, report summarySourceReport, excerpt string) string {
	fileName := strings.TrimSpace(report.Meta.FileName)
	if fileName == "" {
		fileName = filepath.Base(strings.TrimSpace(report.Meta.Path))
	}
	return strings.TrimSpace(fmt.Sprintf(`
## 报告 %d
来源研究组: %s
report_id: %s
file_name: %s
created_at: %s
summary: %s

正文节选:
%s
`, index,
		fallbackText(strings.TrimSpace(report.GroupName), report.GroupID),
		fallbackText(strings.TrimSpace(report.Meta.ReportID), "unknown"),
		fallbackText(fileName, "unknown.md"),
		fallbackText(strings.TrimSpace(report.Meta.CreatedAt), "unknown"),
		fallbackText(strings.TrimSpace(report.Meta.Summary), "无摘要"),
		fallbackText(strings.TrimSpace(excerpt), "无正文"),
	)) + "\n\n"
}

func buildSummaryExcerpt(content string, tokenBudget int) string {
	content = normalizeSummarySourceContent(content)
	if content == "" {
		return ""
	}
	if tokenBudget <= 0 || llm.CountTokens(content) <= tokenBudget {
		return content
	}
	if tokenBudget <= llm.CountTokens(labSummaryOmittedMarker)+32 {
		return truncatePrefixByTokens(content, tokenBudget)
	}
	headBudget := tokenBudget * 2 / 3
	tailBudget := tokenBudget - headBudget - llm.CountTokens(labSummaryOmittedMarker)
	if tailBudget < 32 {
		tailBudget = 32
		headBudget = tokenBudget - tailBudget - llm.CountTokens(labSummaryOmittedMarker)
	}
	head := strings.TrimSpace(truncatePrefixByTokens(content, headBudget))
	tail := strings.TrimSpace(truncateSuffixByTokens(content, tailBudget))
	if head == "" {
		return truncatePrefixByTokens(content, tokenBudget)
	}
	if tail == "" || tail == head {
		return head
	}
	return head + labSummaryOmittedMarker + tail
}

func truncatePrefixByTokens(text string, limit int) string {
	text = strings.TrimSpace(text)
	if text == "" || limit <= 0 || llm.CountTokens(text) <= limit {
		return text
	}
	runes := []rune(text)
	lo, hi := 0, len(runes)
	for lo < hi {
		mid := (lo + hi + 1) / 2
		if llm.CountTokens(string(runes[:mid])) <= limit {
			lo = mid
		} else {
			hi = mid - 1
		}
	}
	return strings.TrimSpace(string(runes[:lo]))
}

func truncateSuffixByTokens(text string, limit int) string {
	text = strings.TrimSpace(text)
	if text == "" || limit <= 0 || llm.CountTokens(text) <= limit {
		return text
	}
	runes := []rune(text)
	lo, hi := 0, len(runes)
	for lo < hi {
		mid := (lo + hi + 1) / 2
		if llm.CountTokens(string(runes[len(runes)-mid:])) <= limit {
			lo = mid
		} else {
			hi = mid - 1
		}
	}
	return strings.TrimSpace(string(runes[len(runes)-lo:]))
}

func fallbackText(value string, fallback string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback
	}
	return value
}

func (lm *LabManager) storeGeneratedSummaryReport(content string, sourceCount int) (taskManager.ReportMeta, error) {
	reportsDir := filepath.Join(lm.GetProjectDir(), "lab", "reports")
	if err := os.MkdirAll(reportsDir, 0755); err != nil {
		return taskManager.ReportMeta{}, err
	}
	createdAt := time.Now().Format("2006-01-02 15:04:05")
	baseName := "lab-summary-" + time.Now().Format("20060102-150405")
	fileName := baseName + ".md"
	path := filepath.Join(reportsDir, fileName)
	for i := 2; ; i++ {
		if _, err := os.Stat(path); os.IsNotExist(err) {
			break
		}
		fileName = fmt.Sprintf("%s-%d.md", baseName, i)
		path = filepath.Join(reportsDir, fileName)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return taskManager.ReportMeta{}, err
	}
	summary := fmt.Sprintf("基于%d份共享报告生成的实验室总结", sourceCount)
	meta := taskManager.ReportMeta{
		ReportID:  lm.nextLabReportID(),
		Path:      path,
		Summary:   summary,
		CreatedAt: createdAt,
		FileName:  fileName,
	}
	lm.labReportMu.Lock()
	lm.labReportCatalog = append(lm.labReportCatalog, meta)
	lm.labReportMu.Unlock()
	return meta, nil
}

func (lm *LabManager) nextLabReportID() string {
	lm.labReportMu.RLock()
	defer lm.labReportMu.RUnlock()
	maxSeq := 0
	for _, report := range lm.labReportCatalog {
		reportID := strings.TrimSpace(report.ReportID)
		if !strings.HasPrefix(reportID, "L.") {
			continue
		}
		n, err := strconv.Atoi(strings.TrimSpace(strings.TrimPrefix(reportID, "L.")))
		if err != nil {
			continue
		}
		if n > maxSeq {
			maxSeq = n
		}
	}
	return fmt.Sprintf("L.%d", maxSeq+1)
}
