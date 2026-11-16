package DecisionBrain

import (
	"AIxVuln/llm"
	"AIxVuln/misc"
	"AIxVuln/toolCalling"
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"
)

func (db *DecisionBrain) getAgentTools() []llm.ToolDef {
	names := make([]string, 0, len(db.agentToolDefs))
	for name := range db.agentToolDefs {
		if _, ok := db.agentHandler[name]; !ok {
			continue
		}
		names = append(names, name)
	}
	sort.Strings(names)
	definitions := make([]llm.ToolDef, 0, len(names))
	for _, name := range names {
		definitions = append(definitions, db.agentToolDefs[name])
	}
	return definitions
}

func (db *DecisionBrain) Start() {
	var ready bool
	contextLimitRecoveries := 0
	consecutiveErrors := 0
	for {
		select {
		case <-db.done:
			return
		default:
		}
		if db.IsBrainFinished() {
			return
		}
		db.memory.SetNoAgentRunning(!db.hasRunningAgents())
		msgList := db.memory.GetContext()
		if msgList == nil {
			select {
			case <-db.done:
				return
			case <-time.After(1 * time.Second):
			}
			continue
		}
		memLastHash := db.memory.GetLastHash()
		if !ready && memLastHash == db.lastMsgHash && db.lastMsgHash != "" {
			select {
			case <-db.notify:
			case <-db.done:
				return
			case <-time.After(30 * time.Second):
			}
			continue
		}
		db.lastMsgHash = memLastHash
		db.memory.CaptureLastLLMContext(msgList)
		assistantMessage, toolMessage, err := db.ToolCallRequest(msgList)
		if err != nil {
			if llm.IsContextLimitError(err) && contextLimitRecoveries < 2 {
				contextLimitRecoveries++
				beforeTokens := llm.CountMessagesTokens(msgList)
				if compressErr := db.memory.CompressIfNeeded(llm.GetResponsesClient("decision", "main_setting"), db.model); compressErr != nil {
					misc.Error("decision", compressErr.Error(), db.SubmitEventHandler)
				}
				afterTokens := llm.CountMessagesTokens(db.memory.GetContext())
				misc.Error("decision", fmt.Sprintf("context limit hit, compressed brain memory and retrying (attempt=%d before=%d after=%d): %s", contextLimitRecoveries, beforeTokens, afterTokens, err.Error()), db.SubmitEventHandler)
				continue
			}
			contextLimitRecoveries = 0
			consecutiveErrors++
			misc.Error("decision", err.Error(), db.SubmitEventHandler)
			if consecutiveErrors >= 2 {
				summary := fmt.Sprintf("决策大脑连续调用模型失败，已暂停本轮组内研判以避免长时间卡住：%s", err.Error())
				db.AppendChatMessage(ChatMessage{Role: "system", Text: summary, Ts: time.Now().Format("15:04:05"), PersonaName: "决策大脑", AvatarFile: "system.png"})
				db.setBrainFinished(true)
				return
			}
			select {
			case <-db.notify:
			case <-db.done:
				return
			case <-time.After(5 * time.Second):
			}
			continue
		}
		consecutiveErrors = 0
		contextLimitRecoveries = 0
		// If the LLM returned an empty response with no tool calls, do NOT add it
		// to memory — it would just bloat the context and cause an infinite loop.
		if len(toolMessage) == 0 && (strings.TrimSpace(assistantMessage.Content) == "" ||
			strings.HasPrefix(assistantMessage.Content, "empty response")) {
			misc.Debug("决策大脑：跳过空响应，等待新事件")
			ready = false
			select {
			case <-db.notify:
			case <-db.done:
				return
			case <-time.After(30 * time.Second):
			}
			continue
		}
		db.memory.AddMessage(assistantMessage)
		if err := db.memory.CompressIfNeeded(llm.GetResponsesClient("decision", "main_setting"), db.model); err != nil {
			misc.Error("decision", err.Error(), db.SubmitEventHandler)
		}

		// Check for special tool markers before adding tool messages to memory.
		hasWait := false
		launchedAgentThisRound := false
		for _, tc := range assistantMessage.ToolCalls {
			if strings.HasPrefix(tc.Name, "Agent-") {
				launchedAgentThisRound = true
			}
		}
		for _, tm := range toolMessage {
			if tm.Content == "__WAIT_PENDING__" {
				hasWait = true
			}
		}

		// Handle Tool-Wait: enter blocking wait after adding tool messages.
		if hasWait {
			// Replace marker with actual result.
			for i := range toolMessage {
				if toolMessage[i].Content == "__WAIT_PENDING__" {
					toolMessage[i].Content = db.WaitTool()
					break
				}
			}
		}

		if len(toolMessage) == 0 {
			if !db.hasRunningAgents() && db.hasRegisteredAgents() {
				db.nudgeAllIdleDecisionBrain("The decision brain produced no tool calls while everyone was idle.")
				ready = true
			} else {
				ready = false
			}
		} else {
			ready = true
		}
		for _, message := range toolMessage {
			db.memory.AddMessage(message)
		}
		if err := db.memory.CompressIfNeeded(llm.GetResponsesClient("decision", "main_setting"), db.model); err != nil {
			misc.Error("decision", err.Error(), db.SubmitEventHandler)
		}
		if db.memoryFilePath != "" {
			_ = db.memory.SaveMemoryToFile(db.memoryFilePath)
		}

		// If Tool-Wait was called, block here until new events arrive.
		if hasWait {
			// Fallback: if all digital humans are idle while the brain chose to wait,
			// AND the brain did NOT launch any agents in this round,
			// inject a user message to nudge the brain into action instead of blocking.
			if !db.hasRunningAgents() && !launchedAgentThisRound && db.hasRegisteredAgents() {
				db.nudgeAllIdleDecisionBrain("The decision brain called Tool-Wait while everyone was idle.")
				misc.Debug("决策大脑兜底：所有数字人空闲且大脑选择等待，插入催促消息")
			} else {
				select {
				case <-db.notify:
				case <-db.done:
					return
				case <-time.After(60 * time.Second):
				}
			}
		}
	}
}

func (db *DecisionBrain) ToolCallRequest(msgList []llm.Message) (llm.Message, []llm.Message, error) {
	sendWS := func(typ string, data interface{}) {
		if typ == "BrainMessage" || typ == "BrainToolCall" {
			if m, ok := data.(map[string]interface{}); ok {
				db.AppendBrainFeed(typ, m)
			} else {
				db.AppendBrainFeed(typ, map[string]interface{}{"value": data})
			}
			// No longer push execution process to frontend.
			return
		}
		if db.webOutputChan == nil {
			return
		}
		msg := WebMsg{Type: typ, Data: data, ProjectName: db.projectName}
		if b, err := json.Marshal(msg); err == nil {
			db.trySendWS(string(b))
		}
	}
	agentTool := db.getAgentTools()
	tools := db.GetTools()
	tools = append(tools, agentTool...)
	cli := llm.GetResponsesClient("decision", "main_setting")
	ctx, c := context.WithTimeout(db.ctx, 180*time.Second)
	defer c()
	resp, err := llm.RequestLLMWithOpts(cli, ctx, db.model, msgList, tools, llm.RequestLLMOpts{ProjectName: db.projectName, AgentLabel: "决策大脑"})
	if err != nil {
		return llm.Message{}, nil, err
	}
	message := llm.ResponseToMessage(resp)
	db.pushTokenUsage()
	db.pushContextBreakdown()
	toolSchemaByName := make(map[string]map[string]interface{}, len(tools))
	for _, def := range tools {
		toolSchemaByName[def.Name] = def.Parameters
	}
	misc.Debug("决策大脑： %s 本次请求消息大小 %d", message.Content, db.memory.GetMsgSize())
	sendWS("BrainMessage", map[string]interface{}{
		"role":    message.Role,
		"content": message.Content,
	})
	// Detect <UserMessage> in brain reply and push to chat panel.
	if umsg := llm.ExtractTag(message.Content, "UserMessage"); umsg != "" {
		db.AppendChatMessage(ChatMessage{Role: "system", Text: umsg, Ts: time.Now().Format("15:04:05"), PersonaName: "决策大脑", AvatarFile: "system.png"})
		sendWS("UserMessage", map[string]interface{}{
			"persona_name": "决策大脑",
			"avatar_file":  "system.png",
			"agent_id":     "",
			"message":      umsg,
		})
	}
	var toolMessage []llm.Message
	for _, tc := range message.ToolCalls {
		sendWS("BrainToolCall", map[string]interface{}{
			"stage":      "call",
			"toolCallID": tc.ID,
			"name":       tc.Name,
			"arguments":  tc.Arguments,
		})
		if strings.HasPrefix(tc.Name, "Agent-") {
			var resultJSON string
			args, argsJSON, err := toolCalling.NormalizeAndValidateToolArguments(tc.Arguments, toolSchemaByName[tc.Name])
			if err != nil {
				errMsg := "invalid arguments: " + err.Error()
				resultJSON = toolCalling.InvalidToolArgumentsFailure(tc.Name, tc.Arguments, err)
				sendWS("BrainToolCall", map[string]interface{}{
					"stage":      "result",
					"toolCallID": tc.ID,
					"name":       tc.Name,
					"error":      errMsg,
				})
				toolMessage = append(toolMessage, llm.Message{
					Role:       llm.RoleTool,
					Content:    resultJSON,
					ToolCallID: tc.ID,
				})
				continue
			}

			// Verifier/Report agents use exploit_idea_id/exploit_chain_id; others require task_content.
			isVerifier := strings.Contains(tc.Name, "Verifier")
			isReport := strings.Contains(tc.Name, "Report")
			if !isVerifier && !isReport {
				taskContent, ok := args["task_content"]
				if !ok || strings.TrimSpace(fmt.Sprint(taskContent)) == "" {
					errMsg := "task_content is required for Agent tools"
					resultJSON = toolCalling.ToolCallFailure("invalid_arguments", tc.Name, tc.Arguments, errMsg)
					sendWS("BrainToolCall", map[string]interface{}{
						"stage":      "result",
						"toolCallID": tc.ID,
						"name":       tc.Name,
						"error":      errMsg,
					})
					toolMessage = append(toolMessage, llm.Message{
						Role:       llm.RoleTool,
						Content:    resultJSON,
						ToolCallID: tc.ID,
					})
					continue
				}
			}

			resultJSON = db.startAgent(tc.Name, argsJSON)
			sendWS("BrainToolCall", map[string]interface{}{
				"stage":      "result",
				"toolCallID": tc.ID,
				"name":       tc.Name,
				"result":     resultJSON,
			})
			toolMessage = append(toolMessage, llm.Message{
				Role:       llm.RoleTool,
				Content:    resultJSON,
				ToolCallID: tc.ID,
			})
		} else {
			toolMessage = append(toolMessage, db.executeDecisionToolCall(tc, toolSchemaByName, sendWS))
		}
	}
	return message, toolMessage, nil
}

func (db *DecisionBrain) executeDecisionToolCall(tc llm.ToolCall, toolSchemaByName map[string]map[string]interface{}, sendWS func(string, interface{})) llm.Message {
	resultJSON := ""
	if tc.Name == "Tool-Wait" {
		resultJSON = "__WAIT_PENDING__"
		sendWS("BrainToolCall", map[string]interface{}{
			"stage":      "result",
			"toolCallID": tc.ID,
			"name":       tc.Name,
			"result":     "Entering wait state...",
		})
		return llm.Message{
			Role:       llm.RoleTool,
			Content:    resultJSON,
			ToolCallID: tc.ID,
		}
	}

	args, _, err := toolCalling.NormalizeAndValidateToolArguments(tc.Arguments, toolSchemaByName[tc.Name])
	if err != nil {
		errMsg := "invalid arguments: " + err.Error()
		sendWS("BrainToolCall", map[string]interface{}{
			"stage":      "result",
			"toolCallID": tc.ID,
			"name":       tc.Name,
			"error":      errMsg,
		})
		return llm.Message{
			Role:       llm.RoleTool,
			Content:    toolCalling.InvalidToolArgumentsFailure(tc.Name, tc.Arguments, err),
			ToolCallID: tc.ID,
		}
	}

	if tc.Name == "Tool-FinishTask" {
		activeIDs := db.getActiveAgentIDs()
		if len(activeIDs) > 0 {
			errMsg := fmt.Sprintf("Cannot publish completion summary: %d agent(s) still running: %s. Wait for all agents to complete before calling Tool-FinishTask. Use Tool-Wait to wait.", len(activeIDs), strings.Join(activeIDs, ", "))
			resultJSON = toolCalling.Fail(errMsg)
			sendWS("BrainToolCall", map[string]interface{}{
				"stage":      "result",
				"toolCallID": tc.ID,
				"name":       tc.Name,
				"error":      errMsg,
			})
			return llm.Message{
				Role:       llm.RoleTool,
				Content:    resultJSON,
				ToolCallID: tc.ID,
			}
		}
		resultJSON = db.FinishTaskTool(args)
		sendWS("BrainToolCall", map[string]interface{}{
			"stage":      "result",
			"toolCallID": tc.ID,
			"name":       tc.Name,
			"result":     "Asking user for confirmation...",
		})
		return llm.Message{
			Role:       llm.RoleTool,
			Content:    resultJSON,
			ToolCallID: tc.ID,
		}
	}

	switch tc.Name {
	case "Tool-SynthesizeChainTool":
		resultJSON = db.SynthesizeChainTool(args)
	case "Tool-SearchExploitIdeaTool":
		resultJSON = db.SearchExploitIdeaTool(args)
	case "Tool-GetExploitIdeaByIdTool":
		resultJSON = db.GetExploitIdeaByIdTool(args)
	case "Tool-GetExploitChainByIdTool":
		resultJSON = db.GetExploitChainByIdTool(args)
	case "Tool-ReadReportChunkTool":
		resultJSON = db.ReadReportChunkTool(args)
	case "Tool-SendMessageToDigitalHuman":
		resultJSON = db.SendMessageToDigitalHumanTool(args)
	default:
		if externalResult, handled := db.callExternalTool(tc.Name, args); handled {
			resultJSON = externalResult
		} else {
			resultJSON = toolCalling.UnknownToolFailure(tc.Name, tc.Arguments)
		}
	}
	sendWS("BrainToolCall", map[string]interface{}{
		"stage":      "result",
		"toolCallID": tc.ID,
		"name":       tc.Name,
		"result":     resultJSON,
	})
	return llm.Message{
		Role:       llm.RoleTool,
		Content:    resultJSON,
		ToolCallID: tc.ID,
	}
}
