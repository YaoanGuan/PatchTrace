package toolCalling

import (
	"AIxVuln/taskManager"
)

type GuidanceTool struct {
	task *taskManager.Task
}

func NewGuidanceTool(task *taskManager.Task) *GuidanceTool {
	return &GuidanceTool{task: task}
}

func (h *GuidanceTool) Name() string {
	return "GuidanceTool"
}
func (h *GuidanceTool) Description() string {
	return "Last-resort expert escalation channel. Call this only after the relevant AskPocKnowledgeTool and/or AskToolUsageGuideTool queries plus your own direct attempts still cannot unblock the task."
}
func (h *GuidanceTool) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"source": map[string]interface{}{
				"type":        "string",
				"description": "Your identity — who you are and what type of digital human you are. For example: '林辰宇(代码分析数字人)' or '江亦琛(漏洞验证数字人)'. This helps the expert give you a more targeted answer.(required)",
			},
			"description": map[string]interface{}{
				"type":        "string",
				"description": "Describe the blocker, what target/product/technique is involved, which AskPocKnowledgeTool / AskToolUsageGuideTool queries you already made, what commands or attempts you tried, and why they failed.(required)",
			},
		},
	}
}

func (h *GuidanceTool) Execute(parameters map[string]interface{}) string {
	descTemp := parameters["description"]
	if descTemp == nil {
		return Fail("Missing 'description' parameter")
	}
	desc, ok := descTemp.(string)
	if !ok {
		return Fail("'description' parameter should be string")
	}
	source := ""
	if s, ok := parameters["source"].(string); ok {
		source = s
	}
	handler := h.task.GetGuidanceHandler()
	if handler == nil {
		return Fail("guidance handler is not set")
	}
	issue := desc + "\n\n"
	r := handler(source, issue)
	return Success(r)
}
