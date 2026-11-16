package toolCalling

import (
	"AIxVuln/misc"
	"AIxVuln/taskManager"
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type ReportVulnTool struct {
	task *taskManager.Task
}

func NewReportVulnTool(task *taskManager.Task) *ReportVulnTool {
	return &ReportVulnTool{task: task}
}

func (h *ReportVulnTool) Name() string {
	return "ReportVulnTool"
}
func (h *ReportVulnTool) Description() string {
	return "Submit the completed report. The report body must be in Chinese Markdown, and you must also provide a very short Chinese summary that states what the report covers."
}
func (h *ReportVulnTool) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"Report": map[string]interface{}{
				"type":        "string",
				"description": "Write in Markdown format and use Chinese.(required)",
			},
			"Summary": map[string]interface{}{
				"type":        "string",
				"description": "A very short Chinese summary of the report content. Keep it to one short sentence and no more than 60 characters.(required)",
			},
			"VulnType": map[string]interface{}{
				"type":        "string",
				"description": "Vulnerability type abbreviations, such as: SQLi, SSRF, UploadFile.(required)",
			},
		},
		"required": []string{"Report", "Summary", "VulnType"},
	}
}

func (h *ReportVulnTool) Execute(parameters map[string]interface{}) string {
	ReportTemp := parameters["Report"]
	if ReportTemp == nil {
		return Fail("Missing 'Report' parameter")
	}
	Report, ok := ReportTemp.(string)
	if !ok || strings.TrimSpace(Report) == "" {
		return Fail("Invalid 'Report' parameter")
	}
	if err := validateReportContent(Report); err != nil {
		return Fail(err.Error())
	}
	summaryTemp := parameters["Summary"]
	if summaryTemp == nil {
		return Fail("Missing 'Summary' parameter")
	}
	summary, ok := summaryTemp.(string)
	if !ok {
		return Fail("Invalid 'Summary' parameter")
	}
	summary = strings.TrimSpace(summary)
	if summary == "" {
		return Fail("'Summary' cannot be empty")
	}
	if len([]rune(summary)) > 60 {
		return Fail("'Summary' must be no more than 60 characters")
	}
	VulnTypeTemp := parameters["VulnType"]
	if VulnTypeTemp == nil {
		return Fail("Missing 'VulnType' parameter")
	}
	vulnType, ok := VulnTypeTemp.(string)
	if !ok || strings.TrimSpace(vulnType) == "" {
		return Fail("Invalid 'VulnType' parameter")
	}
	vulnDir := filepath.Join(h.task.GetProjectDir(), "vulns")
	currentDate := time.Now().Format("2006-01-02")
	folderName := filepath.Join(vulnDir, currentDate)
	_ = os.MkdirAll(folderName, 0755)
	var filename string
	index := 0
	for {
		filename = filepath.Join(folderName, fmt.Sprintf("%s-%d.md", vulnType, index))
		if _, err := os.Stat(filename); os.IsNotExist(err) {
			if err := os.WriteFile(filename, []byte(Report), 0644); err != nil {
				return Fail("failed to write report: " + err.Error())
			}
			handler := h.task.GetReportHandler()
			if handler == nil {
				return Fail("report handler not configured")
			}
			handler(taskManager.ReportMeta{
				TargetID:  strings.TrimSpace(h.task.GetCurrVulnId()),
				Path:      filename,
				Summary:   summary,
				CreatedAt: time.Now().Format("2006-01-02 15:04:05"),
			})
			break
		}
		index++
	}
	misc.Success("报告生成成功", fmt.Sprintf("报告输出: %s \n", filename), h.task.GetEventHandler())
	return Success("ok")
}

func validateReportContent(report string) error {
	if hasUnclosedMarkdownFence(report) {
		return fmt.Errorf("report appears truncated: fenced code block is not closed")
	}
	return nil
}

func hasUnclosedMarkdownFence(report string) bool {
	currentFence := ""
	scanner := bufio.NewScanner(strings.NewReader(report))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		fence := markdownFenceMarker(line)
		if fence == "" {
			continue
		}
		if currentFence == "" {
			currentFence = fence
			continue
		}
		if currentFence == fence {
			currentFence = ""
		}
	}
	return currentFence != ""
}

func markdownFenceMarker(line string) string {
	switch {
	case strings.HasPrefix(line, "```"):
		return "```"
	case strings.HasPrefix(line, "~~~"):
		return "~~~"
	default:
		return ""
	}
}
