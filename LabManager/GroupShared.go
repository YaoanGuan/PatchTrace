package LabManager

import (
	"AIxVuln/agents"
	"AIxVuln/prompts"
	"AIxVuln/taskManager"
	"fmt"
	"reflect"
	"strings"
)

func parseChatAddress(raw string) (target string, body string, addressed bool) {
	if !strings.HasPrefix(raw, "@") {
		return "", raw, false
	}
	spaceIdx := strings.IndexByte(raw, ' ')
	if spaceIdx < 0 {
		return strings.TrimSpace(strings.TrimPrefix(raw, "@")), "", true
	}
	target = strings.TrimSpace(raw[1:spaceIdx])
	body = strings.TrimSpace(raw[spaceIdx+1:])
	return target, body, true
}

func buildDecisionCoordinationPrompt(group researchGroup, sender string, raw string) string {
	groupName := ""
	brainName := ""
	if group != nil {
		groupName = group.Name()
		brainName = group.DecisionBrainName()
	}
	return prompts.BuildDecisionCoordinationPrompt(sender, groupName, brainName, raw)
}

func groupDecisionBrainRelayRulePrompt() string {
	return prompts.GroupDecisionBrainRelayRulePrompt()
}

func groupDecisionBrainKnowledgePrompt() string {
	return prompts.GroupDecisionBrainKnowledgePrompt()
}

func decisionReportLabel(reportType string) string {
	switch strings.ToLower(strings.TrimSpace(reportType)) {
	case "finding":
		return "发现"
	case "blocker":
		return "阻塞"
	case "completion":
		return "完成"
	case "status":
		return "状态"
	default:
		return "进展"
	}
}

func formatDecisionMakersReport(reportType string, summary string, details string, nextStep string) string {
	lines := []string{fmt.Sprintf("%s：%s", decisionReportLabel(reportType), strings.TrimSpace(summary))}
	if strings.TrimSpace(details) != "" {
		lines = append(lines, strings.TrimSpace(details))
	}
	if strings.TrimSpace(nextStep) != "" {
		lines = append(lines, "下一步："+strings.TrimSpace(nextStep))
	}
	return strings.Join(lines, "\n")
}

func totalBrainReceiptText(groupName string, reportType string) string {
	switch strings.ToLower(strings.TrimSpace(reportType)) {
	case "finding":
		return fmt.Sprintf("总脑已收到来自%s的阶段性发现，继续沿当前方向协调推进。", groupName)
	case "blocker":
		return fmt.Sprintf("总脑已收到来自%s的阻塞汇报，将评估是否需要调整分工或引入协作组。", groupName)
	case "completion":
		return fmt.Sprintf("总脑已收到来自%s的完成汇报，会继续检查是否还需要补充协作。", groupName)
	case "status":
		return fmt.Sprintf("总脑已收到来自%s的状态汇报，将继续跟踪该组推进。", groupName)
	default:
		return fmt.Sprintf("总脑已收到来自%s的进展汇报，将继续跟踪该组推进。", groupName)
	}
}

func buildTotalBrainReportFollowUpPrompt(reply string) string {
	return prompts.BuildTotalBrainReportFollowUpPrompt(reply)
}

func buildCollaborationWakePrompt(sourceGroupName string, reason string) string {
	return prompts.BuildCollaborationWakePrompt(sourceGroupName, reason)
}

func stringArg(args map[string]interface{}, key string) string {
	if args == nil {
		return ""
	}
	value, ok := args[key]
	if !ok || value == nil {
		return ""
	}
	switch v := value.(type) {
	case string:
		return strings.TrimSpace(v)
	default:
		return strings.TrimSpace(fmt.Sprint(v))
	}
}

func rosterHasPersona(roster map[string]interface{}, persona string) bool {
	persona = strings.TrimSpace(persona)
	if persona == "" {
		return false
	}
	for _, value := range roster {
		rv := reflect.ValueOf(value)
		if !rv.IsValid() || rv.Kind() != reflect.Slice {
			continue
		}
		for i := 0; i < rv.Len(); i++ {
			item := rv.Index(i).Interface()
			switch v := item.(type) {
			case map[string]interface{}:
				if strings.TrimSpace(fmt.Sprintf("%v", v["persona_name"])) == persona {
					return true
				}
			default:
				iv := reflect.ValueOf(item)
				if iv.IsValid() && iv.Kind() == reflect.Struct {
					field := iv.FieldByName("PersonaName")
					if field.IsValid() && strings.TrimSpace(fmt.Sprintf("%v", field.Interface())) == persona {
						return true
					}
				}
			}
		}
	}
	return false
}

func rosterSize(roster map[string]interface{}) int {
	total := 0
	for _, value := range roster {
		rv := reflect.ValueOf(value)
		if !rv.IsValid() {
			continue
		}
		if rv.Kind() == reflect.Slice || rv.Kind() == reflect.Array {
			total += rv.Len()
		}
	}
	return total
}

func configuredDigitalHumanCount(pool map[string][]agents.AgentProfile) int {
	total := 0
	for _, profiles := range pool {
		total += len(profiles)
	}
	return total
}

func webGroupDescription() string {
	return "能力：代码审计组面向拥有源码或可获取到核心代码/脚本/前端产物的目标，负责静态代码审计与白盒漏洞挖掘，覆盖鉴权、注入、反序列化、文件上传、路径穿越、源码级命令执行/RCE，以及基于代码证据构建攻击链。不适合纯黑盒入口探测或仅凭外部行为开展的渗透任务。"
}

func binaryGroupDescription() string {
	return "能力：二进制研究组已接入原生二进制逆向数字人与 Android 逆向数字人，可分别通过必须配置的 ida-pro-mcp 与 jadx-ai-mcp 对可执行文件、样本、APK、DEX、JAR、AAB 进行逆向分析，并可直接提交组共享报告。"
}

func pentestGroupDescription() string {
	return "能力：渗透测试组面向黑盒渗透与目标深挖场景，当前内置渗透测试工程师、SRC 深挖工程师与后渗透工程师，可围绕入口探测、初始立足点、单目标逻辑漏洞与攻击面深挖、以及后渗透链路进行研判，并可直接提交组共享报告。"
}

func groupPlanningDescription(groupID string) string {
	switch strings.TrimSpace(groupID) {
	case webGroupID:
		return webGroupDescription()
	case binaryGroupID:
		return binaryGroupDescription()
	case pentestGroupID:
		return pentestGroupDescription()
	default:
		return ""
	}
}

func cloneMap(src map[string]interface{}) map[string]interface{} {
	if src == nil {
		return nil
	}
	dst := make(map[string]interface{}, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}

func ternaryState(ok bool, yes string, no string) string {
	if ok {
		return yes
	}
	return no
}

func isAllAlias(target string) bool {
	switch strings.ToLower(strings.TrimSpace(target)) {
	case "all", "全体":
		return true
	default:
		return false
	}
}

func isTotalBrainAlias(target string) bool {
	switch strings.ToLower(strings.TrimSpace(target)) {
	case "总脑", "lab", "brain", "control":
		return true
	default:
		return false
	}
}

func isWebGroupAlias(target string) bool {
	switch strings.ToLower(strings.TrimSpace(target)) {
	case "web", "web安全", "web安全组", "web安全研究组", "web-security", "web组决策大脑", "web安全研究组决策大脑", "web安全研究组·决策大脑":
		return true
	default:
		return false
	}
}

func isBinaryGroupAlias(target string) bool {
	switch strings.ToLower(strings.TrimSpace(target)) {
	case "binary", "bin", "二进制", "二进制安全", "二进制安全组", "二进制安全研究组", "binary-security", "二进制组决策大脑", "二进制安全研究组决策大脑", "二进制安全研究组·决策大脑":
		return true
	default:
		return false
	}
}

func isPentestGroupAlias(target string) bool {
	switch strings.ToLower(strings.TrimSpace(target)) {
	case "pentest", "pentest-security", "渗透", "渗透组", "渗透测试", "渗透测试组", "渗透测试研究组", "渗透测试组决策大脑", "渗透测试研究组决策大脑", "渗透测试组·决策大脑":
		return true
	default:
		return false
	}
}

func decisionBrainDisplayName(groupName string) string {
	groupName = strings.TrimSpace(groupName)
	if groupName == "" {
		return "决策大脑"
	}
	return groupName + "·决策大脑"
}

func groupChannelID(groupID string) string {
	return "group:" + strings.TrimSpace(groupID)
}

func groupChannelName(groupName string) string {
	groupName = strings.TrimSpace(groupName)
	if groupName == "" {
		return "组内群"
	}
	return groupName + "·组内群"
}

func buildSharedReportWakePrompt(sourceGroupName string, report taskManager.ReportMeta) string {
	sourceGroupName = strings.TrimSpace(sourceGroupName)
	if sourceGroupName == "" {
		sourceGroupName = "其它研究组"
	}
	reportID := strings.TrimSpace(report.ReportID)
	if reportID == "" {
		reportID = "unknown"
	}
	fileName := strings.TrimSpace(report.FileName)
	if fileName == "" {
		fileName = reportID
	}
	summary := strings.TrimSpace(report.Summary)
	if summary == "" {
		summary = "无摘要"
	}
	return strings.TrimSpace(fmt.Sprintf(`[System Event] 共享报告目录新增了一份报告。
来源研究组: %s
report_id: %s
file_name: %s
summary: %s

请立即判断这份新报告是否会影响你当前的组内计划、协作需求或完成状态；如果会，立刻调整分工或同步新的状态。`, sourceGroupName, reportID, fileName, summary))
}

func normalizeChannelID(channelID string) string {
	channelID = strings.TrimSpace(channelID)
	if channelID == "" {
		return decisionMakersChannelID
	}
	return channelID
}

func channelGroupID(channelID string) string {
	if strings.HasPrefix(channelID, "group:") {
		return strings.TrimSpace(strings.TrimPrefix(channelID, "group:"))
	}
	return ""
}

func decisionBrainAliasMatches(group researchGroup, target string) bool {
	if group == nil {
		return false
	}
	target = strings.TrimSpace(target)
	if target == "" {
		return false
	}
	want := strings.TrimSpace(group.DecisionBrainName())
	if target == want {
		return true
	}
	switch group.ID() {
	case webGroupID:
		return isWebGroupAlias(target)
	case binaryGroupID:
		return isBinaryGroupAlias(target)
	case pentestGroupID:
		return isPentestGroupAlias(target)
	default:
		return false
	}
}
