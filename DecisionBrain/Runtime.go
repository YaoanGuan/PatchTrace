package DecisionBrain

import (
	"AIxVuln/agents"
	"fmt"
	"strings"
	"time"
)

type AgentRuntime struct {
	agent             agents.Agent
	agentToolName     string
	Resp              *agents.StartResp
	startedAt         time.Time
	done              bool
	activity          string
	activityUpdatedAt time.Time
}

func agentRespError(resp *agents.StartResp) string {
	if resp == nil || resp.Err == nil {
		return ""
	}
	return strings.TrimSpace(resp.Err.Error())
}

func agentRespSummary(resp *agents.StartResp) string {
	if resp == nil {
		return ""
	}
	summary := strings.TrimSpace(resp.Summary)
	if summary != "" {
		return summary
	}
	if errText := agentRespError(resp); errText != "" {
		return "失败：" + errText
	}
	return ""
}

func truncateRunes(text string, limit int) string {
	if limit <= 0 {
		return ""
	}
	runes := []rune(text)
	if len(runes) <= limit {
		return text
	}
	return string(runes[:limit]) + "..."
}

func agentRunTask(agent agents.Agent) string {
	if agent == nil {
		return ""
	}
	task := agent.GetTask()
	if task == nil {
		return ""
	}
	var taskContent string
	for i, v := range task.GetTaskList() {
		taskContent += fmt.Sprintf("task.%d: %s\n", i, v["TaskContent"])
	}
	return taskContent
}

func (r *AgentRuntime) publicActivity(state string) string {
	activity := strings.TrimSpace(r.activity)
	if activity != "" {
		return activity
	}
	switch strings.ToLower(strings.TrimSpace(state)) {
	case "done", "completed", "success":
		return "已完成"
	case "failed", "error", "cancelled":
		return "已结束"
	case "standby", "idle":
		return "待命"
	case "coordinating":
		return "组内研判中"
	default:
		return "思考中"
	}
}

func (r *AgentRuntime) addActivityInfo(rs map[string]interface{}, state string) {
	activity := r.publicActivity(state)
	rs["Activity"] = activity
	rs["activity"] = activity
	if !r.activityUpdatedAt.IsZero() {
		updated := r.activityUpdatedAt.Format("2006-01-02 15:04:05")
		rs["ActivityUpdatedAt"] = updated
		rs["activity_updated_at"] = updated
	}
}

func (db *DecisionBrain) PublicActivity() string {
	if db == nil {
		return "待命"
	}
	if db.IsWaiting() {
		if db.hasRunningAgents() {
			return "正在等待数字人完成"
		}
		return "等待新事件"
	}
	if db.hasRunningAgents() {
		return "正在协调数字人"
	}
	return "决策大脑思考中"
}

func (r *AgentRuntime) GetRunInfo() map[string]interface{} {
	rs := make(map[string]interface{})
	rs["AgentID"] = r.agent.GetId()
	state := r.agent.GetState()
	rs["RunState"] = state
	r.addActivityInfo(rs, state)
	if p := r.agent.GetProfile(); p.PersonaName != "" {
		rs["persona_name"] = p.PersonaName
	}
	rs["StartedAt"] = r.startedAt.Format("2006-01-02 15:04:05")

	stateLower := strings.ToLower(strings.TrimSpace(state))
	isDone := stateLower == "done" || stateLower == "completed" || stateLower == "success" || stateLower == "failed" || stateLower == "error" || stateLower == "cancelled"
	if errText := agentRespError(r.Resp); errText != "" {
		rs["RUNError"] = errText
	}

	if isDone {
		// For done agents: only a short one-line summary, no task content.
		rs["RUNSummary"] = truncateRunes(agentRespSummary(r.Resp), 120)
	} else {
		// For active agents: include task and a moderate summary.
		rs["RUNTask"] = agentRunTask(r.agent)
		rs["RUNSummary"] = truncateRunes(agentRespSummary(r.Resp), 300)
	}
	return rs
}

// GetRunInfoFull returns the full run info for UI display (not truncated).
func (r *AgentRuntime) GetRunInfoFull() map[string]interface{} {
	rs := make(map[string]interface{})
	rs["AgentID"] = r.agent.GetId()
	state := r.agent.GetState()
	rs["RunState"] = state
	r.addActivityInfo(rs, state)
	if p := r.agent.GetProfile(); p.PersonaName != "" || p.Gender != "" {
		rs["Profile"] = map[string]interface{}{
			"digital_human_id": p.DigitalHumanID,
			"persona_name":     p.PersonaName,
			"gender":           p.Gender,
			"avatar_file":      p.AvatarFile,
			"personality":      p.Personality,
			"age":              p.Age,
		}
	}
	rs["RUNTask"] = agentRunTask(r.agent)
	rs["StartedAt"] = r.startedAt.Format("2006-01-02 15:04:05")
	rs["RUNSummary"] = agentRespSummary(r.Resp)
	if errText := agentRespError(r.Resp); errText != "" {
		rs["RUNError"] = errText
	}
	// Expose agent context usage for UI progress bar.
	if mem := r.agent.GetMemory(); mem != nil {
		rs["context_size"] = mem.GetMsgSize(r.agent.GetId())
		rs["max_context"] = mem.GetMaxHistory()
	}
	return rs
}

func runtimeIdentityKey(r *AgentRuntime) string {
	if r == nil || r.agent == nil {
		return ""
	}
	if p := r.agent.GetProfile(); strings.TrimSpace(p.DigitalHumanID) != "" {
		return "dh:" + strings.TrimSpace(p.DigitalHumanID)
	}
	return "agent:" + strings.TrimSpace(r.agent.GetId())
}

func latestAgentRuntimesByIdentity(runs []*AgentRuntime) []*AgentRuntime {
	latest := make(map[string]*AgentRuntime)
	order := make([]string, 0, len(runs))
	for _, run := range runs {
		if run == nil || run.agent == nil {
			continue
		}
		key := runtimeIdentityKey(run)
		if key == "" {
			continue
		}
		previous, exists := latest[key]
		if !exists {
			order = append(order, key)
			latest[key] = run
			continue
		}
		if run.startedAt.After(previous.startedAt) || run.startedAt.Equal(previous.startedAt) {
			latest[key] = run
		}
	}
	out := make([]*AgentRuntime, 0, len(order))
	for _, key := range order {
		if run := latest[key]; run != nil {
			out = append(out, run)
		}
	}
	return out
}

type WebMsg struct {
	Type        string      `json:"type"`
	Data        interface{} `json:"data"`
	ProjectName string      `json:"projectName"`
}

func (db *DecisionBrain) SetAgentActivity(agentID string, activity string) {
	if db == nil {
		return
	}
	agentID = strings.TrimSpace(agentID)
	activity = strings.TrimSpace(activity)
	if agentID == "" || activity == "" {
		return
	}
	updated := false
	for _, run := range db.runAgentList {
		if run == nil || run.agent == nil || run.agent.GetId() != agentID || run.done {
			continue
		}
		run.activity = activity
		run.activityUpdatedAt = time.Now()
		updated = true
		break
	}
	if updated {
		db.AgentStateUpdate("activity")
	}
}
