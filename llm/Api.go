package llm

import (
	"AIxVuln/misc"
	"context"
	"errors"
	"fmt"
	"runtime"
	"time"
)

// RequestLLMOpts carries optional metadata for a single LLM request.
type RequestLLMOpts struct {
	ProjectName string // project-level token accumulation
	AgentLabel  string // per-agent token accumulation (e.g. persona name or "决策大脑")
}

// RequestLLM sends a chat request through the abstract Client interface.
// Per-API-key concurrency limiting is handled by the rateLimitedClient wrapper
// created in client_factory.go.
// The variadic projectName is kept for backward compatibility; prefer RequestLLMWithOpts for new code.
func RequestLLM(cli Client, ctx context.Context, model string, messages []Message, tools []ToolDef, projectName ...string) (Response, error) {
	opts := RequestLLMOpts{}
	if len(projectName) > 0 {
		opts.ProjectName = projectName[0]
	}
	return RequestLLMWithOpts(cli, ctx, model, messages, tools, opts)
}

// RequestLLMWithOpts sends a chat request and accumulates token usage per project and per agent.
func RequestLLMWithOpts(cli Client, ctx context.Context, model string, messages []Message, tools []ToolDef, opts RequestLLMOpts) (Response, error) {
	if cli == nil {
		return Response{}, fmt.Errorf("nil llm client")
	}
	pc, f, _, ok := runtime.Caller(1)
	callSite := ""
	if ok {
		callSite = fmt.Sprintf("%s-%s", f, runtime.FuncForPC(pc).Name())
	}
	maxRetry := misc.GetMaxTryCount()
	attempt := 0
	var (
		resp Response
		err  error
	)
	for {
		resp, err = cli.Chat(ctx, model, messages, tools)
		if err == nil || !shouldRetryLLMError(err) || attempt >= maxRetry {
			break
		}
		misc.Debug("API RETRY: attempt=%d/%d err=%s from=%s", attempt+1, maxRetry, err.Error(), callSite)
		if sleepErr := sleepWithContext(ctx, 5*time.Second); sleepErr != nil {
			err = sleepErr
			break
		}
		attempt++
	}
	if err != nil {
		if callSite != "" {
			misc.Debug("API ERR: %s. from: %s", err.Error(), callSite)
		} else {
			misc.Debug("API ERR: %s", err.Error())
		}
	}
	// Accumulate token usage for the project (and optionally per-agent).
	if err == nil && resp.Usage.TotalTokens > 0 && opts.ProjectName != "" {
		if opts.AgentLabel != "" {
			AddProjectAgentTokenUsage(opts.ProjectName, opts.AgentLabel, resp.Usage)
		} else {
			AddProjectTokenUsage(opts.ProjectName, resp.Usage)
		}
	}
	return resp, err
}

func shouldRetryLLMError(err error) bool {
	if err == nil {
		return false
	}
	if IsContextLimitError(err) {
		return false
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}
	return true
}

func sleepWithContext(ctx context.Context, delay time.Duration) error {
	if delay <= 0 {
		return nil
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

// ResponseToMessage converts a Response into an assistant Message,
// preserving any tool calls.
func ResponseToMessage(resp Response) Message {
	return Message{
		Role:      RoleAssistant,
		Content:   resp.Content,
		ToolCalls: resp.ToolCalls,
	}
}
