package agents

func GeneralAgentDefinition() AgentDefinition {
	return AgentDefinition{
		Name:        "Agent-General-GeneralCommonAgent",
		Description: "A general-purpose technical agent for miscellaneous tasks such as repository operations, code search, script execution, environment inspection, and MCP-backed collaboration workflows.",
		Args: ObjSchema(map[string]SchemaProp{
			"task_content": StrProp("Task Content.(required)", nil),
		}),
		NewFunc: NewGeneralCommonAgent,
	}
}
