package DecisionBrain

import (
	"AIxVuln/llm"
	"AIxVuln/taskManager"
	"AIxVuln/toolCalling"
	"encoding/json"
	"strings"
	"time"
)

func (db *DecisionBrain) GetTools() []llm.ToolDef {
	var definitions []llm.ToolDef

	if db.enableExploitFlow {
		definitions = append(definitions, llm.ToolDef{
			Name:        "Tool-SynthesizeChainTool",
			Description: "Used to combine one or more exploitable `exploitIdeaId`s into a single `exploitChain`. You need to provide the set of exploitIdea IDs and the rationale for their combined exploitation. After submission is complete, you will see relevant status information in the next status panel.",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"exploitIdeaIdSet": map[string]interface{}{
						"type":        "array",
						"description": "Set of exploitIdea IDs to be combined into an exploitChain, e.g., [\"E.1\", \"E.9\"] (required)",
						"items": map[string]interface{}{
							"type": "string",
						},
					},
					"idea": map[string]interface{}{
						"type":        "string",
						"description": "Exploitation rationale for the exploitChain.(required)",
					},
				},
			},
		})
	}

	// Tool: Wait — brain enters waiting state until new events arrive.
	definitions = append(definitions, llm.ToolDef{
		Name:        "Tool-Wait",
		Description: "Enter a waiting state. Use this when there is nothing to do right now and you need to wait for digital humans to finish their work or for new events. The system will wake you when something changes.",
		Parameters:  map[string]interface{}{"type": "object", "properties": map[string]interface{}{}},
	})

	// Tool: FinishTask — publish a completion summary to the decision-makers channel.
	definitions = append(definitions, llm.ToolDef{
		Name:        "Tool-FinishTask",
		Description: "Publish a completion summary for the current research round to the decision-makers channel. This tool does NOT end the project. Use it only when all currently scheduled digital humans have completed and you can provide a concise Chinese summary of what has been accomplished, plus optional details and next steps.",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"summary": map[string]interface{}{
					"type":        "string",
					"description": "A concise Chinese summary of the completed work for the current round. Required.",
				},
				"details": map[string]interface{}{
					"type":        "string",
					"description": "Optional extra Chinese details for the decision-makers channel.",
				},
				"next_step": map[string]interface{}{
					"type":        "string",
					"description": "Optional Chinese next step recommendation.",
				},
			},
			"required": []string{"summary"},
		},
	})

	if db.hasDigitalHumans() {
		// Tool: send a user message / instruction to a specific digital human.
		definitions = append(definitions, llm.ToolDef{
			Name:        "Tool-SendMessageToDigitalHuman",
			Description: "Send an instruction or message to a specific digital human by persona name, regardless of whether they are busy or idle.",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"persona_name": map[string]interface{}{
						"type":        "string",
						"description": "The persona name of the target digital human (must match a name from the DigitalHumanRoster). (required)",
					},
					"message": map[string]interface{}{
						"type":        "string",
						"description": "The instruction or message to send to the digital human. (required)",
					},
				},
				"required": []string{"persona_name", "message"},
			},
		})
	}

	definitions = append(definitions, llm.ToolDef{
		Name:        "Tool-ReadReportChunkTool",
		Description: "Read a report incrementally by reportId. Use offset and limit to inspect the report in small chunks instead of dumping the whole file at once.",
		Parameters: map[string]interface{}{
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
		},
	})

	if db.enableExploitFlow && db.memory != nil && db.memory.IsCompressed() {
		definitions = append(definitions, llm.ToolDef{
			Name:        "Tool-SearchExploitIdeaTool",
			Description: "Search exploitable exploitIdeas stored in SQLite and return a list of matches with brief fields (ID/state/harm/condition/title/type/file/route).",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"query": map[string]interface{}{
						"type":        "string",
						"description": "Search query for exploitable exploitIdeas (matches harm/condition/title/type/file/route/json). Empty means list latest.",
					},
					"limit": map[string]interface{}{
						"type":        "integer",
						"description": "Max results (1-100). Default 20.",
					},
				},
			},
		})

		definitions = append(definitions, llm.ToolDef{
			Name:        "Tool-GetExploitIdeaByIdTool",
			Description: "Fetch full JSON of an exploitable exploitIdea by ID from SQLite. Use this when the status panel is in compressed mode.",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"exploitIdeaId": map[string]interface{}{
						"type":        "string",
						"description": "ExploitIdea ID to fetch full details from SQLite, e.g., E.1 (required)",
					},
				},
				"required": []string{"exploitIdeaId"},
			},
		})

		definitions = append(definitions, llm.ToolDef{
			Name:        "Tool-GetExploitChainByIdTool",
			Description: "Fetch full JSON of an exploitable exploitChain by ID from SQLite. Use this when the status panel is in compressed mode.",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"exploitChainId": map[string]interface{}{
						"type":        "string",
						"description": "ExploitChain ID to fetch full details from SQLite, e.g., C.1 (required)",
					},
				},
				"required": []string{"exploitChainId"},
			},
		})
	}
	definitions = append(definitions, db.getExternalTools()...)
	return definitions
}

func (db *DecisionBrain) SynthesizeChainTool(parameters map[string]interface{}) string {
	ids := parameters["exploitIdeaIdSet"]
	if ids == nil {
		return toolCalling.Fail("exploitIdeaIdSet is required")
	}
	idea := parameters["idea"]
	if idea == nil {
		return toolCalling.Fail("idea is required")
	}
	var idSet []string
	switch v := ids.(type) {
	case []string:
		idSet = v
	case []interface{}:
		idSet = make([]string, 0, len(v))
		for _, one := range v {
			s, ok := one.(string)
			if !ok {
				return toolCalling.Fail("exploitIdeaIdSet format error")
			}
			idSet = append(idSet, s)
		}
	default:
		return toolCalling.Fail("exploitIdeaIdSet format error")
	}
	ideaString, e := idea.(string)
	if !e {
		return toolCalling.Fail("idea format error")
	}
	err := db.SynthesizeChain(idSet, ideaString)
	if err != nil {
		return toolCalling.Fail(err.Error())
	}
	return toolCalling.Success("success")
}

func (db *DecisionBrain) SearchExploitIdeaTool(parameters map[string]interface{}) string {
	if db.store == nil {
		return toolCalling.Fail("sqlite store not initialized")
	}
	q := ""
	if v := parameters["query"]; v != nil {
		qs, ok := v.(string)
		if ok {
			q = qs
		}
	}
	limit := 0
	if v := parameters["limit"]; v != nil {
		switch n := v.(type) {
		case float64:
			limit = int(n)
		case int:
			limit = n
		}
	}
	rows, err := db.store.SearchExploitableIdeas(q, limit)
	if err != nil {
		return toolCalling.Fail(err.Error())
	}
	js, _ := json.Marshal(rows)
	return toolCalling.Success(string(js))
}

func (db *DecisionBrain) GetExploitIdeaByIdTool(parameters map[string]interface{}) string {
	if db.store == nil {
		return toolCalling.Fail("sqlite store not initialized")
	}
	idV := parameters["exploitIdeaId"]
	if idV == nil {
		return toolCalling.Fail("exploitIdeaId is required")
	}
	id, ok := idV.(string)
	if !ok {
		return toolCalling.Fail("exploitIdeaId format error")
	}
	js, err := db.store.GetExploitableIdeaJSONById(id)
	if err != nil {
		return toolCalling.Fail(err.Error())
	}
	var idea taskManager.ExploitIdea
	if err := json.Unmarshal([]byte(js), &idea); err != nil {
		return toolCalling.Fail(err.Error())
	}
	pretty, _ := json.MarshalIndent(idea, "", "  ")
	return toolCalling.Success(string(pretty))
}

func (db *DecisionBrain) ReadReportChunkTool(parameters map[string]interface{}) string {
	idV := parameters["reportId"]
	if idV == nil {
		return toolCalling.Fail("reportId is required")
	}
	reportID, ok := idV.(string)
	if !ok || reportID == "" {
		return toolCalling.Fail("reportId format error")
	}
	offset := 0
	if v := parameters["offset"]; v != nil {
		switch n := v.(type) {
		case float64:
			offset = int(n)
		case int:
			offset = n
		default:
			return toolCalling.Fail("offset format error")
		}
	}
	limit := defaultReportChunkLimit
	if v := parameters["limit"]; v != nil {
		switch n := v.(type) {
		case float64:
			limit = int(n)
		case int:
			limit = n
		default:
			return toolCalling.Fail("limit format error")
		}
	}
	chunk, err := db.ReadReportChunk(reportID, offset, limit)
	if err != nil {
		return toolCalling.Fail(err.Error())
	}
	js, _ := json.Marshal(chunk)
	return toolCalling.Success(string(js))
}

func (db *DecisionBrain) WaitTool() string {
	db.setWaiting(true)
	return toolCalling.Success("Entering wait state. You will be woken when new events arrive.")
}

func (db *DecisionBrain) FinishTaskTool(parameters map[string]interface{}) string {
	summary, _ := parameters["summary"].(string)
	summary = strings.TrimSpace(summary)
	if summary == "" {
		return toolCalling.Fail("summary is required")
	}
	details, _ := parameters["details"].(string)
	nextStep, _ := parameters["next_step"].(string)

	reportArgs := map[string]interface{}{
		"report_type": "completion",
		"summary":     summary,
	}
	if strings.TrimSpace(details) != "" {
		reportArgs["details"] = strings.TrimSpace(details)
	}
	if strings.TrimSpace(nextStep) != "" {
		reportArgs["next_step"] = strings.TrimSpace(nextStep)
	}
	if result, ok := db.callExternalTool("Tool-ReportToDecisionMakers", reportArgs); ok {
		var payload struct {
			Success bool `json:"success"`
		}
		if err := json.Unmarshal([]byte(result), &payload); err == nil && payload.Success {
			db.setBrainFinished(true)
		}
		return result
	}
	db.EmitBrainMessage("完成总结：" + summary)
	db.setBrainFinished(true)
	return toolCalling.Success("Completion summary recorded locally")
}

func (db *DecisionBrain) SendMessageToDigitalHumanTool(parameters map[string]interface{}) string {
	nameV := parameters["persona_name"]
	if nameV == nil {
		return toolCalling.Fail("persona_name is required")
	}
	personaName, ok := nameV.(string)
	if !ok || personaName == "" {
		return toolCalling.Fail("persona_name must be a non-empty string")
	}
	msgV := parameters["message"]
	if msgV == nil {
		return toolCalling.Fail("message is required")
	}
	msg, ok := msgV.(string)
	if !ok || msg == "" {
		return toolCalling.Fail("message must be a non-empty string")
	}
	// Push to chat panel: 决策大脑 @数字人: message
	chatText := "@" + personaName + " " + msg
	db.AppendChatMessage(ChatMessage{Role: "system", Text: chatText, Ts: time.Now().Format("15:04:05"), PersonaName: "决策大脑", AvatarFile: "system.png"})
	if db.webOutputChan != nil {
		wsMsg := WebMsg{Type: "UserMessage", Data: map[string]interface{}{
			"persona_name": "决策大脑",
			"avatar_file":  "system.png",
			"agent_id":     "",
			"message":      chatText,
		}, ProjectName: db.projectName}
		if b, err := json.Marshal(wsMsg); err == nil {
			db.trySendWS(string(b))
		}
	}
	result := db.TeamChat("@"+personaName+" "+msg, "决策大脑")
	if result == "" {
		return toolCalling.Success("message sent to " + personaName)
	}
	return toolCalling.Success(result)
}

func (db *DecisionBrain) GetExploitChainByIdTool(parameters map[string]interface{}) string {
	if db.store == nil {
		return toolCalling.Fail("sqlite store not initialized")
	}
	idV := parameters["exploitChainId"]
	if idV == nil {
		return toolCalling.Fail("exploitChainId is required")
	}
	id, ok := idV.(string)
	if !ok {
		return toolCalling.Fail("exploitChainId format error")
	}
	js, err := db.store.GetExploitableChainJSONById(id)
	if err != nil {
		return toolCalling.Fail(err.Error())
	}
	var chain taskManager.ExploitChain
	if err := json.Unmarshal([]byte(js), &chain); err != nil {
		return toolCalling.Fail(err.Error())
	}
	pretty, _ := json.MarshalIndent(chain, "", "  ")
	return toolCalling.Success(string(pretty))
}
