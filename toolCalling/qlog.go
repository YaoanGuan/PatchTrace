package toolCalling

import (
	"AIxVuln/misc"
	"AIxVuln/taskManager"
	"encoding/json"
	"strings"
)

var qLogPath = "q.log"

type qLogEntry struct {
	Tool        string `json:"tool"`
	ProjectName string `json:"project_name,omitempty"`
	Question    string `json:"question"`
	Answer      string `json:"answer,omitempty"`
	Error       string `json:"error,omitempty"`
}

func appendQuestionAnswerLog(task *taskManager.Task, toolName string, question string, answer string, err error) {
	entry := qLogEntry{
		Tool:     strings.TrimSpace(toolName),
		Question: strings.TrimSpace(question),
		Answer:   strings.TrimSpace(answer),
	}
	if task != nil {
		entry.ProjectName = strings.TrimSpace(task.GetProjectName())
	}
	if err != nil {
		entry.Error = err.Error()
	}
	payload, marshalErr := json.Marshal(entry)
	if marshalErr != nil {
		misc.Debug("appendQuestionAnswerLog marshal failed for %s: %v", toolName, marshalErr)
		return
	}
	if writeErr := misc.AppendLogBasic(qLogPath, string(payload)); writeErr != nil {
		misc.Debug("appendQuestionAnswerLog write failed for %s: %v", toolName, writeErr)
	}
}
