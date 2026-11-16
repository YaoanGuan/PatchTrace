package agents

import (
	"AIxVuln/llm"
	"AIxVuln/misc"
	"AIxVuln/prompts"
	"AIxVuln/skill"
	"AIxVuln/taskManager"
	"AIxVuln/toolCalling"
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// debugLastMessages prints the last 2 messages from the context before an LLM request.
func debugLastMessages(personaName string, msgList []llm.Message) {
	start := len(msgList) - 2
	if start < 0 {
		start = 0
	}
	for i := start; i < len(msgList); i++ {
		content := msgList[i].Content
		if len([]rune(content)) > 30 {
			content = string([]rune(content)[:30]) + "..."
		}
		misc.Debug("[%s] ctx[%d] %s: %s", personaName, i, msgList[i].Role, content)
	}
}

func DebugLastMessages(personaName string, msgList []llm.Message) {
	debugLastMessages(personaName, msgList)
}

// extractAgentFinishSummary checks if any tool message contains the AgentFinishMarker.
// Returns (summary, true) if found, ("", false) otherwise.
func extractAgentFinishSummary(toolMessages []llm.Message) (string, bool) {
	for _, tm := range toolMessages {
		if strings.HasPrefix(tm.Content, toolCalling.AgentFinishMarker) {
			return strings.TrimPrefix(tm.Content, toolCalling.AgentFinishMarker), true
		}
	}
	return "", false
}

func ExtractAgentFinishSummary(toolMessages []llm.Message) (string, bool) {
	return extractAgentFinishSummary(toolMessages)
}

type AgentProfile struct {
	DigitalHumanID string `json:"digital_human_id"`
	PersonaName    string `json:"persona_name"`
	Gender         string `json:"gender"`
	AvatarFile     string `json:"avatar_file"`
	Personality    string `json:"personality"`
	Age            int    `json:"age"`
	ExtraSysPrompt string `json:"extra_sys_prompt"`
}

// TaskAssignment carries the information needed for one task run.
type TaskAssignment struct {
	ArgsJson string
	DoneCb   func(*StartResp) // called when the agent finishes this task
}

type Agent interface {
	Name() string
	// StartTask is a long-running loop. It blocks waiting for tasks via AssignTask,
	// executes each one, calls the DoneCb, then waits for the next task.
	// It only returns when ctx is cancelled (project shutdown).
	StartTask(ctx context.Context) *StartResp
	// AssignTask sends a new task to the agent. The agent's StartTask loop picks it up.
	AssignTask(assignment TaskAssignment)
	GetMemory() llm.Memory
	SetMemory(llm.Memory)
	GetTask() *taskManager.Task
	SetKeyMessage(k map[string][]interface{})
	GetId() string
	SetId(id string)
	GetState() string
	GetProfile() AgentProfile
	SetProfile(p AgentProfile)
	SetEnvInfo(map[string]interface{})
	SetStateHandler(func(string))
	SetState(state string)
}

type StartResp struct {
	Err     error
	Memory  llm.Memory
	EvnInfo map[string]interface{}
	Summary string
}

func SkillsPromptFragment() string {
	return prompts.SkillsPromptFragment(availableSkillsPromptSummary())
}

const (
	maxPromptSkillHints        = 8
	maxPromptSkillDescRunes    = 120
	maxPromptSkillSummaryRunes = 1600
)

func availableSkillsPromptSummary() string {
	items, err := skill.List()
	if err != nil || len(items) == 0 {
		return ""
	}
	var b strings.Builder
	count := 0
	for _, item := range items {
		name := strings.TrimSpace(item.Name)
		if name == "" {
			continue
		}
		desc := normalizeSkillPromptText(item.Description)
		if desc == "" {
			desc = "Local reusable workflow instructions."
		}
		desc = truncateRunes(desc, maxPromptSkillDescRunes)
		line := fmt.Sprintf("- %s: %s\n", name, desc)
		if b.Len()+len(line) > maxPromptSkillSummaryRunes {
			break
		}
		b.WriteString(line)
		count++
		if count >= maxPromptSkillHints {
			break
		}
	}
	if count == 0 {
		return ""
	}
	if len(items) > count {
		b.WriteString("- More skills may exist. Use ListSkillsTool for the full catalog when needed.\n")
	}
	return strings.TrimRight(b.String(), "\n")
}

func normalizeSkillPromptText(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	s = strings.ReplaceAll(s, "\r", " ")
	s = strings.ReplaceAll(s, "\n", " ")
	return strings.Join(strings.Fields(s), " ")
}

func truncateRunes(s string, limit int) string {
	if limit <= 0 {
		return ""
	}
	runes := []rune(s)
	if len(runes) <= limit {
		return s
	}
	if limit <= 3 {
		return string(runes[:limit])
	}
	return string(runes[:limit-3]) + "..."
}

func teammateHandoffPrompt(agentToolName string) string {
	return prompts.TeammateHandoffPrompt(agentToolName)
}

func CommonSystemPrompt() string {
	return CommonSystemPromptForAgent("")
}

func KnowledgeLookupPriorityPrompt() string {
	return prompts.KnowledgeLookupPriorityPrompt()
}

func CommonSystemPromptForAgent(agentToolName string) string {
	skillsFragment := ""
	if commonSystemPromptNeedsSkills(agentToolName) {
		skillsFragment = SkillsPromptFragment()
	}
	return prompts.CommonSystemPromptForAgent(
		agentToolName,
		skillsFragment,
	)
}

func commonSystemPromptNeedsSkills(agentToolName string) bool {
	name := strings.ToLower(strings.TrimSpace(agentToolName))
	if name == "" || strings.Contains(name, "report") || strings.Contains(name, "analyze") {
		return false
	}
	for _, marker := range []string{"ops", "verifier", "pentest", "postexploitation", "srccommon", "reverse", "android", "general", "projectoverview"} {
		if strings.Contains(name, marker) {
			return true
		}
	}
	return false
}

func commonSystemPromptNeedsAgentTools(agentToolName string) bool {
	name := strings.ToLower(strings.TrimSpace(agentToolName))
	for _, marker := range []string{"ops", "verifier", "pentest", "postexploitation", "srccommon", "reverse", "android", "general"} {
		if strings.Contains(name, marker) {
			return true
		}
	}
	return false
}

type AgentDefinition struct {
	Name        string                                         `json:"Name"`
	Description string                                         `json:"Description"`
	Args        SchemaObject                                   `json:"args"`
	NewFunc     func(*taskManager.Task, string) (Agent, error) `json:"-"`
}

func (d AgentDefinition) ToMap() map[string]interface{} {
	var m map[string]interface{}
	b, _ := json.Marshal(d)
	_ = json.Unmarshal(b, &m)
	return m
}

type SchemaObject struct {
	Type       string                `json:"type"`
	Properties map[string]SchemaProp `json:"properties"`
}

type SchemaProp struct {
	Type        string   `json:"type"`
	Enum        []string `json:"enum,omitempty"`
	Description string   `json:"description"`
}

func (s SchemaObject) ToMap() map[string]interface{} {
	var out map[string]interface{}
	b, _ := json.Marshal(s)
	_ = json.Unmarshal(b, &out)
	return out
}

type schemaObject = SchemaObject
type schemaProp = SchemaProp

func ObjSchema(props map[string]SchemaProp) SchemaObject {
	return SchemaObject{Type: "object", Properties: props}
}

func StrProp(desc string, enums []string) SchemaProp {
	return SchemaProp{Type: "string", Enum: enums, Description: desc}
}

func objSchema(props map[string]schemaProp) schemaObject {
	return ObjSchema(props)
}

func strProp(desc string, enums []string) schemaProp {
	return StrProp(desc, enums)
}
