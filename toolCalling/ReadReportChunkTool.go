package toolCalling

import (
	"AIxVuln/taskManager"
	"encoding/json"
	"fmt"
)

type ReadReportChunkTool struct {
	task *taskManager.Task
}

func NewReadReportChunkTool(task *taskManager.Task) *ReadReportChunkTool {
	return &ReadReportChunkTool{task: task}
}

func (h *ReadReportChunkTool) Name() string {
	return "ReadReportChunkTool"
}

func (h *ReadReportChunkTool) Description() string {
	return "Read a report incrementally by reportId. Use offset and limit to inspect the report in small chunks instead of dumping the whole file at once."
}

func (h *ReadReportChunkTool) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"reportId": map[string]interface{}{
				"type":        "string",
				"description": "The reportId from ReportCatalog, for example `R.1`.(required)",
			},
			"offset": map[string]interface{}{
				"type":        "integer",
				"description": "Character offset to start reading from. Default 0.",
			},
			"limit": map[string]interface{}{
				"type":        "integer",
				"description": "Maximum number of characters to return in this call. Default 1600, max 4000.",
			},
		},
		"required": []string{"reportId"},
	}
}

func (h *ReadReportChunkTool) Execute(parameters map[string]interface{}) string {
	reportIDValue := parameters["reportId"]
	if reportIDValue == nil {
		return Fail("Missing 'reportId' parameter")
	}
	reportID, ok := reportIDValue.(string)
	if !ok || reportID == "" {
		return Fail("Invalid 'reportId' parameter")
	}
	offset, err := readReportIntArg(parameters["offset"], 0)
	if err != nil {
		return Fail("Invalid 'offset' parameter: " + err.Error())
	}
	limit, err := readReportIntArg(parameters["limit"], 1600)
	if err != nil {
		return Fail("Invalid 'limit' parameter: " + err.Error())
	}
	reader := h.task.GetReportChunkReader()
	if reader == nil {
		return Fail("report reader not configured")
	}
	chunk, err := reader(reportID, offset, limit)
	if err != nil {
		return Fail(err.Error())
	}
	js, err := json.Marshal(chunk)
	if err != nil {
		return Fail(err.Error())
	}
	return Success(string(js))
}

func readReportIntArg(value interface{}, defaultValue int) (int, error) {
	if value == nil {
		return defaultValue, nil
	}
	switch v := value.(type) {
	case float64:
		return int(v), nil
	case int:
		return v, nil
	case int32:
		return int(v), nil
	case int64:
		return int(v), nil
	default:
		return 0, fmt.Errorf("expected integer")
	}
}
