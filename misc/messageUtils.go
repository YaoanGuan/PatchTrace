package misc

import "AIxVuln/prompts"

// 沙盒类工具补充提示词
func AttackSandboxPrompt() string {
	return prompts.AttackSandboxToolPrompt()
}
