package web

import (
	shared "AIxVuln/agents"
	"AIxVuln/misc"
	"strings"
)

func GetAgentDescription() ([]shared.AgentDefinition, []map[string]interface{}) {
	return GetAgentDescriptionForMode(misc.GetEffectiveWebResearchMode())
}

func GetAgentDescriptionForMode(mode string) ([]shared.AgentDefinition, []map[string]interface{}) {
	defs := []shared.AgentDefinition{
		{
			Name:        "Agent-Analyze-AnalyzeCommonAgent",
			Description: "This is an intelligent agent designed for vulnerability mining and code analysis, equipped with the capability to perform vulnerability mining and code analysis for multiple programming languages.",
			Args: shared.ObjSchema(map[string]shared.SchemaProp{
				"exploitIdeaMaxCount": shared.StrProp("Per-task quota: maximum number of NEW exploit ideas the agent can submit in this task assignment (independent of previously discovered ones).(required)", nil),
				"task_content":        shared.StrProp("Task Content.(required)", nil),
			}),
			NewFunc: NewAnalyzeCommonAgent,
		},
		{
			Name:        "Agent-Ops-OpsCommonAgent",
			Description: "This is a general-purpose operations agent capable of handling setup and maintenance tasks for projects across multiple languages. It retains memory of previously completed tasks and has a good understanding of the environment. After it finishes a setup task, you can obtain details such as the environment's URL, username/password, cookies, database information used, and more. When assigning tasks to it, you need to specify whether it is an environment setup task or a maintenance task.Possesses various operational capabilities for containers.Suitable for setting up an environment from scratch or performing operational tasks within the container environment.",
			Args: shared.ObjSchema(map[string]shared.SchemaProp{
				"task_content": shared.StrProp("Task Content.(required)", nil),
			}),
			NewFunc: NewOpsCommonAgent,
		},
		{
			Name:        "Agent-Report-ReportCommonAgent",
			Description: "Report writing agent. It will automatically fetch the full exploit data (including runtime evidence and PoC) by the provided ID. You only need to pass the ID and report type; do NOT paste the full exploit JSON.",
			Args: shared.ObjSchema(map[string]shared.SchemaProp{
				"reportType":       shared.StrProp("Report type. Use 'analyzer' for an unverified (analysis-only) report, or 'verifier' for a verified report that must include runtime evidence and PoC.(required)", []string{"analyzer", "verifier"}),
				"exploit_idea_id":  shared.StrProp("The exploitIdeaId to write a report for (e.g. 'E.0'). Mutually exclusive with exploit_chain_id.", nil),
				"exploit_chain_id": shared.StrProp("The exploitChainId to write a report for (e.g. 'C.0'). Mutually exclusive with exploit_idea_id.", nil),
				"task_content":     shared.StrProp("Optional extra instructions or notes for the report agent.", nil),
			}),
			NewFunc: NewReportCommonAgent,
		},
		{
			Name:        "Agent-Verifier-VerifierCommonAgent",
			Description: "Verification agent. It will automatically fetch the full exploit data by the provided ID. You only need to pass the ID; do NOT paste the full exploit JSON.",
			Args: shared.ObjSchema(map[string]shared.SchemaProp{
				"exploit_idea_id":  shared.StrProp("The exploitIdeaId to verify (e.g. 'E.0'). Mutually exclusive with exploit_chain_id.", nil),
				"exploit_chain_id": shared.StrProp("The exploitChainId to verify (e.g. 'C.0'). Mutually exclusive with exploit_idea_id.", nil),
				"task_content":     shared.StrProp("Optional extra instructions or context for the verifier.", nil),
			}),
			NewFunc: NewVerifierCommonAgent,
		},
		{
			Name:        "Agent-Ops-OpsEnvScoutAgent",
			Description: "An AI-powered assistant that can retrieve information such as usernames, passwords, database details, URL routes, and login credentials from running web environments through SSH and Docker operations. Use it when working with externally pre-configured testing environments.Suitable for performing information collection or operational tasks in an existing testing environment.",
			Args: shared.ObjSchema(map[string]shared.SchemaProp{
				"task_content":    shared.StrProp("Task Content.(required)", nil),
				"env_base_config": shared.StrProp("Known environment information, such as SSH connection details for the test environment, container IDs, web system information, etc.", nil),
			}),
			NewFunc: NewOpsEnvScoutAgent,
		},
	}
	defs = filterAgentDefinitionsForMode(defs, mode)
	out := make([]map[string]interface{}, 0, len(defs))
	for _, d := range defs {
		out = append(out, d.ToMap())
	}
	return defs, out
}

func filterAgentDefinitionsForMode(defs []shared.AgentDefinition, mode string) []shared.AgentDefinition {
	normalized, err := misc.NormalizeWebResearchMode(mode)
	if err != nil {
		normalized = misc.WebResearchModeAudit
	}
	if normalized != misc.WebResearchModeComprehensive {
		return defs
	}
	allowed := map[string]struct{}{
		"Agent-Analyze-AnalyzeCommonAgent": {},
		"Agent-Report-ReportCommonAgent":   {},
	}
	filtered := make([]shared.AgentDefinition, 0, len(defs))
	for _, def := range defs {
		name := strings.TrimSpace(def.Name)
		if _, ok := allowed[name]; !ok {
			continue
		}
		filtered = append(filtered, def)
	}
	return filtered
}
