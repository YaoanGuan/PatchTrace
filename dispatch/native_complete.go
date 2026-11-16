package dispatch

import (
	"context"
	"fmt"
	"strings"

	"AIxVuln/llm"
	"AIxVuln/misc"
)

// NativeComplete builds the one-shot LLM completion used by the native worker
// driver. It mirrors worker.NewNative's fallback: the model is resolved from
// section first, then main_setting, and the client is built with
// llm.GetResponsesClient(section, "main_setting").
//
// A missing model or unconfigured API key is a clear error so main.go can log
// it and keep the existing closed loop running when the graph lane is enabled
// but the LLM is not configured.
func NativeComplete(section string) (func(ctx context.Context, prompt string) (string, error), error) {
	section = strings.TrimSpace(section)
	if section == "" {
		section = "main_setting"
	}
	model := strings.TrimSpace(misc.GetConfigValueDefault(section, "MODEL",
		misc.GetConfigValueDefault("main_setting", "MODEL", "")))
	if model == "" {
		return nil, fmt.Errorf("dispatch: native completion: model is not configured for section %q", section)
	}
	client := llm.GetResponsesClient(section, "main_setting")
	if client == nil {
		return nil, fmt.Errorf("dispatch: native completion: llm client is not configured for section %q", section)
	}
	return func(ctx context.Context, prompt string) (string, error) {
		resp, err := llm.RequestLLM(client, ctx, model, []llm.Message{
			{Role: llm.RoleUser, Content: prompt},
		}, nil)
		if err != nil {
			return "", fmt.Errorf("dispatch: native completion: %w", err)
		}
		return resp.Content, nil
	}, nil
}
