package DecisionBrain

import (
	"AIxVuln/agents"
	"AIxVuln/llm"
	"encoding/json"
)

func (db *DecisionBrain) SetGlobalStatusPanel(payload map[string]interface{}) {
	if db == nil {
		return
	}
	if payload == nil {
		payload = map[string]interface{}{}
	}
	cloned, encoded := cloneGlobalStatusPanel(payload)

	db.mu.Lock()
	db.globalStatusPanel = cloned
	unchanged := encoded == db.globalStatusJSON
	db.globalStatusJSON = encoded
	db.mu.Unlock()

	if unchanged {
		return
	}
	db.broadcastGlobalStatusPanelToAgents(cloned)
}

func (db *DecisionBrain) currentGlobalStatusPanel() map[string]interface{} {
	if db == nil {
		return nil
	}
	db.mu.Lock()
	defer db.mu.Unlock()
	if db.globalStatusPanel == nil {
		return nil
	}
	cloned, _ := cloneGlobalStatusPanel(db.globalStatusPanel)
	return cloned
}

func (db *DecisionBrain) injectGlobalStatusPanel(a agents.Agent, agentToolName string) {
	if a == nil || a.GetMemory() == nil {
		return
	}
	panel := db.currentGlobalStatusPanel()
	if len(panel) == 0 {
		return
	}
	a.GetMemory().AddKeyMessage(&llm.EnvMessageX{
		Key:       "GlobalStatusPanel",
		Content:   db.sanitizeForDigitalHumanTool(agentToolName, panel),
		AppendEnv: false,
	})
	if sessions := extractBackgroundSessionsFromGlobalStatus(panel); sessions != nil {
		a.GetMemory().AddKeyMessage(&llm.EnvMessageX{
			Key:       "BackgroundCommandSessions",
			Content:   db.sanitizeForDigitalHumanTool(agentToolName, sessions),
			AppendEnv: false,
		})
	}
}

func (db *DecisionBrain) broadcastGlobalStatusPanelToAgents(panel map[string]interface{}) {
	type runningAgentRef struct {
		agent    agents.Agent
		toolName string
	}
	db.mu.Lock()
	runningAgents := make([]runningAgentRef, 0, len(db.runAgentList))
	for _, ar := range db.runAgentList {
		if ar != nil && ar.agent != nil && !ar.done {
			runningAgents = append(runningAgents, runningAgentRef{
				agent:    ar.agent,
				toolName: ar.agentToolName,
			})
		}
	}
	db.mu.Unlock()

	for _, ref := range runningAgents {
		ref.agent.GetMemory().AddKeyMessage(&llm.EnvMessageX{
			Key:       "GlobalStatusPanel",
			Content:   db.sanitizeForDigitalHumanTool(ref.toolName, panel),
			AppendEnv: false,
		})
		if sessions := extractBackgroundSessionsFromGlobalStatus(panel); sessions != nil {
			ref.agent.GetMemory().AddKeyMessage(&llm.EnvMessageX{
				Key:       "BackgroundCommandSessions",
				Content:   db.sanitizeForDigitalHumanTool(ref.toolName, sessions),
				AppendEnv: false,
			})
		}
	}
}

func extractBackgroundSessionsFromGlobalStatus(panel map[string]interface{}) map[string]interface{} {
	if panel == nil {
		return nil
	}
	raw, ok := panel["background_processes"]
	if !ok || raw == nil {
		return nil
	}
	sessionMap, _ := raw.(map[string]interface{})
	return sessionMap
}

func cloneGlobalStatusPanel(payload map[string]interface{}) (map[string]interface{}, string) {
	data, err := json.Marshal(payload)
	if err != nil {
		return map[string]interface{}{}, ""
	}
	cloned := make(map[string]interface{})
	if err := json.Unmarshal(data, &cloned); err != nil {
		return map[string]interface{}{}, string(data)
	}
	return cloned, string(data)
}
