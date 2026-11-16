package binary

import shared "AIxVuln/agents"

func GetAgentDescription() ([]shared.AgentDefinition, []map[string]interface{}) {
	defs := []shared.AgentDefinition{
		{
			Name:        "Agent-Binary-ReverseCommonAgent",
			Description: "Binary reverse engineering agent for executable/sample analysis. It relies on IDA Pro MCP plus local command-line tooling to inspect binaries, recover behavior, and hand findings back to the decision brain for coordination.",
			Args: shared.ObjSchema(map[string]shared.SchemaProp{
				"task_content":     shared.StrProp("Task Content.(required)", nil),
				"binary_file_path": shared.StrProp("Target binary file path or sample path that should be analyzed.", nil),
			}),
			NewFunc: NewReverseCommonAgent,
		},
		{
			Name:        "Agent-Binary-AndroidCommonAgent",
			Description: "An Android offensive/defensive agent for APK/DEX/JAR analysis and various debugging tasks. It can leverage common tools such as jadx, frida, adb, apktool, etc., to perform APK analysis and runtime debugging.",
			Args: shared.ObjSchema(map[string]shared.SchemaProp{
				"task_content":  shared.StrProp("Task Content.(required)", nil),
				"apk_file_path": shared.StrProp("Target APK, DEX, AAB, or Android-related package path that should be analyzed.", nil),
			}),
			NewFunc: NewAndroidCommonAgent,
		},
	}
	out := make([]map[string]interface{}, 0, len(defs))
	for _, d := range defs {
		out = append(out, d.ToMap())
	}
	return defs, out
}
