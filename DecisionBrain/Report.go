package DecisionBrain

import (
	"AIxVuln/agents"
	"AIxVuln/llm"
	"AIxVuln/taskManager"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	defaultReportChunkLimit = 1600
	maxReportChunkLimit     = 4000
)

func (db *DecisionBrain) AddReport(report taskManager.ReportMeta) {
	report.Path = strings.TrimSpace(report.Path)
	report.Summary = strings.TrimSpace(report.Summary)
	report.TargetID = strings.TrimSpace(report.TargetID)
	if report.Path == "" || report.Summary == "" {
		return
	}
	db.mu.Lock()
	report.ReportID = fmt.Sprintf("R.%d", len(db.reportList)+1)
	if strings.TrimSpace(report.CreatedAt) == "" {
		report.CreatedAt = time.Now().Format("2006-01-02 15:04:05")
	}
	if strings.TrimSpace(report.FileName) == "" {
		report.FileName = filepath.Base(report.Path)
	}
	db.reportList = append(db.reportList, report)
	db.mu.Unlock()
	db.syncReportCatalogMemory()
	if db.memory != nil {
		targetID := report.TargetID
		if targetID == "" {
			targetID = report.FileName
		}
		db.memory.AddMessage(llm.Message{
			Role: llm.RoleUser,
			Content: fmt.Sprintf("[System Event] A new report has been submitted to ReportCatalog.\nreport_id: %s\ntarget_id: %s\nsummary: %s\nIf this report completes a milestone or the current task, send a concise update through Tool-ReportToDecisionMakers before waiting or finishing.",
				report.ReportID, targetID, report.Summary),
		})
	}
	db.signal()
	if db.webOutputChan != nil {
		payload := map[string]interface{}{
			"reportCatalog": db.buildReportCatalogItems(),
		}
		if b, err := json.Marshal(WebMsg{Type: "ReportCatalogUpdate", Data: payload, ProjectName: db.projectName}); err == nil {
			db.trySendWS(string(b))
		}
	}
	if db.reportObserver != nil {
		db.reportObserver(report)
	}
}

func (db *DecisionBrain) RestoreReportCatalog(reports []taskManager.ReportMeta) {
	db.mu.Lock()
	db.reportList = make([]taskManager.ReportMeta, 0, len(reports))
	for i, report := range reports {
		report.Path = strings.TrimSpace(report.Path)
		if report.Path == "" {
			continue
		}
		report.ReportID = strings.TrimSpace(report.ReportID)
		if report.ReportID == "" {
			report.ReportID = fmt.Sprintf("R.%d", i+1)
		}
		report.TargetID = strings.TrimSpace(report.TargetID)
		report.Summary = strings.TrimSpace(report.Summary)
		report.CreatedAt = strings.TrimSpace(report.CreatedAt)
		if report.CreatedAt == "" {
			report.CreatedAt = time.Now().Format("2006-01-02 15:04:05")
		}
		report.FileName = strings.TrimSpace(report.FileName)
		if report.FileName == "" {
			report.FileName = filepath.Base(report.Path)
		}
		db.reportList = append(db.reportList, report)
	}
	db.mu.Unlock()
	if db.memory != nil {
		if payload := db.buildReportCatalogPayload(); payload != nil {
			db.memory.AddKeyMessage("ReportCatalog", payload, false)
		}
	}
}

func (db *DecisionBrain) injectReportCatalog(a agents.Agent, agentToolName string) {
	if a == nil || a.GetMemory() == nil {
		return
	}
	payload := db.buildReportCatalogPayload()
	if payload == nil {
		return
	}
	a.GetMemory().AddKeyMessage(&llm.EnvMessageX{
		Key:       "ReportCatalog",
		Content:   db.sanitizeForDigitalHumanTool(agentToolName, payload),
		AppendEnv: false,
	})
}

func (db *DecisionBrain) syncReportCatalogMemory() {
	payload := db.buildReportCatalogPayload()
	if payload == nil {
		return
	}
	db.memory.AddKeyMessage("ReportCatalog", payload, false)
	db.broadcastReportCatalogToAgents(payload)
	db.signal()
}

func (db *DecisionBrain) broadcastReportCatalogToAgents(payload map[string]interface{}) {
	type runningAgentRef struct {
		agent    agents.Agent
		toolName string
	}
	db.mu.Lock()
	runningAgents := make([]runningAgentRef, 0, len(db.runAgentList))
	for _, ar := range db.runAgentList {
		if ar != nil && ar.agent != nil && !ar.done {
			runningAgents = append(runningAgents, runningAgentRef{agent: ar.agent, toolName: ar.agentToolName})
		}
	}
	db.mu.Unlock()
	for _, ref := range runningAgents {
		a := ref.agent
		a.GetMemory().AddKeyMessage(&llm.EnvMessageX{
			Key:       "ReportCatalog",
			Content:   db.sanitizeForDigitalHumanTool(ref.toolName, payload),
			AppendEnv: false,
		})
	}
}

func (db *DecisionBrain) buildReportCatalogPayload() map[string]interface{} {
	items := db.buildReportCatalogItems()
	if len(items) == 0 {
		return nil
	}
	return map[string]interface{}{
		"items":  items,
		"notice": "Only short summaries are shown here. Use ReadReportChunkTool or Tool-ReadReportChunkTool with reportId, offset, and limit to read a report incrementally.",
	}
}

func (db *DecisionBrain) buildReportCatalogItems() []map[string]interface{} {
	reports := db.GetReportList()
	if len(reports) == 0 {
		return nil
	}
	items := make([]map[string]interface{}, 0, len(reports))
	for _, report := range reports {
		items = append(items, map[string]interface{}{
			"reportId":  report.ReportID,
			"targetId":  report.TargetID,
			"summary":   report.Summary,
			"createdAt": report.CreatedAt,
			"fileName":  report.FileName,
		})
	}
	return items
}

func (db *DecisionBrain) ReadReportChunk(reportID string, offset int, limit int) (*taskManager.ReportChunk, error) {
	report, err := db.getReportByID(reportID)
	if err != nil {
		return nil, err
	}
	content, err := os.ReadFile(report.Path)
	if err != nil {
		return nil, fmt.Errorf("read report failed: %w", err)
	}
	limit = normalizeReportChunkLimit(limit)
	runes := []rune(string(content))
	if offset < 0 {
		offset = 0
	}
	if offset > len(runes) {
		offset = len(runes)
	}
	end := offset + limit
	if end > len(runes) {
		end = len(runes)
	}
	returned := end - offset
	nextOffset := end
	return &taskManager.ReportChunk{
		ReportID:   report.ReportID,
		TargetID:   report.TargetID,
		Summary:    report.Summary,
		CreatedAt:  report.CreatedAt,
		FileName:   report.FileName,
		Offset:     offset,
		Limit:      limit,
		Returned:   returned,
		Total:      len(runes),
		NextOffset: nextOffset,
		Done:       nextOffset >= len(runes),
		Content:    string(runes[offset:end]),
	}, nil
}

func (db *DecisionBrain) getReportByID(reportID string) (taskManager.ReportMeta, error) {
	reportID = strings.TrimSpace(reportID)
	if reportID == "" {
		return taskManager.ReportMeta{}, fmt.Errorf("reportId is required")
	}
	db.mu.Lock()
	defer db.mu.Unlock()
	for _, report := range db.reportList {
		if strings.EqualFold(strings.TrimSpace(report.ReportID), reportID) {
			return report, nil
		}
	}
	return taskManager.ReportMeta{}, fmt.Errorf("report %s not found", reportID)
}

func normalizeReportChunkLimit(limit int) int {
	if limit <= 0 {
		return defaultReportChunkLimit
	}
	if limit > maxReportChunkLimit {
		return maxReportChunkLimit
	}
	return limit
}
