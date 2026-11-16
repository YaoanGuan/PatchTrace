package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"AIxVuln/llm"
	"AIxVuln/misc"
)

const (
	defaultNativeName = "native"
	defaultSection    = "main_setting"
	defaultPromptDir  = "prompts/graph/default"
	healthProbePrompt = "ping"
)

// NativeConfig configures the in-process native driver.
//
// Context supplies the values used to render the shipped prompt templates
// ({graph_yaml}, {fact_ids}, {open_intents}, {intent_id},
// {intent_description}). A nil Context acts as an empty map; callers that use
// the template fallback (empty Task.Prompt) must provide every placeholder the
// template needs.
//
// Complete is the injectable one-shot completion used by the driver. When it
// is nil the driver falls back to llm.GetResponsesClient(Section,
// "main_setting") and the configured model. Tests and the dispatcher's fake
// infrastructure can supply a closure instead of touching the network.
type NativeConfig struct {
	Name      string
	Section   string
	Model     string
	PromptDir string
	Context   func(Task) map[string]string
	Complete  func(ctx context.Context, prompt string) (string, error)
}

// NewNative returns a Driver backed by the in-process LLM client.
func NewNative(cfg NativeConfig) Driver {
	if strings.TrimSpace(cfg.Name) == "" {
		cfg.Name = defaultNativeName
	}
	if strings.TrimSpace(cfg.Section) == "" {
		cfg.Section = defaultSection
	}
	if strings.TrimSpace(cfg.PromptDir) == "" {
		cfg.PromptDir = defaultPromptDir
	}
	return &nativeDriver{cfg: cfg}
}

type nativeDriver struct {
	cfg NativeConfig
}

func (d *nativeDriver) Name() string { return d.cfg.Name }

func (d *nativeDriver) Supports(tt TaskType) bool {
	switch tt {
	case TaskBootstrap, TaskReason, TaskExplore:
		return true
	default:
		return false
	}
}

func (d *nativeDriver) SupportsConclude() bool { return true }

func (d *nativeDriver) HealthCheck(ctx context.Context) error {
	_, err := d.complete(ctx, healthProbePrompt)
	return err
}

func (d *nativeDriver) Execute(ctx context.Context, t Task) (Result, error) {
	tt, err := taskTypeOf(t.Type)
	if err != nil {
		return Result{}, err
	}
	prompt, err := d.promptFor(tt, t, false)
	if err != nil {
		return Result{}, err
	}
	callCtx, cancel := timeoutContext(ctx, t.Timeout)
	defer cancel()
	raw, err := d.complete(callCtx, prompt)
	if err != nil {
		return Result{}, err
	}
	return ParseResult(raw, tt)
}

func (d *nativeDriver) Conclude(ctx context.Context, t Task) (Result, error) {
	tt, err := taskTypeOf(t.Type)
	if err != nil {
		return Result{}, err
	}
	if tt == TaskReason {
		return Result{}, fmt.Errorf("worker: reason task has no conclude prompt")
	}
	prompt, err := d.promptFor(tt, t, true)
	if err != nil {
		return Result{}, err
	}
	timeout := t.ConcludeTimeout
	if timeout <= 0 {
		timeout = t.Timeout
	}
	callCtx, cancel := timeoutContext(ctx, timeout)
	defer cancel()
	raw, err := d.complete(callCtx, prompt)
	if err != nil {
		return Result{}, err
	}
	return ParseResult(raw, tt)
}

// promptFor resolves the prompt for Execute or Conclude.
//
// Execute uses a caller-rendered Task.Prompt verbatim when present; otherwise
// it loads the per-type template and renders it from Context. Conclude always
// loads the *_conclude template and ignores Task.Prompt, so reusing the
// executed Task cannot resubmit the execute prompt.
func (d *nativeDriver) promptFor(tt TaskType, t Task, conclude bool) (string, error) {
	if !conclude && strings.TrimSpace(t.Prompt) != "" {
		return t.Prompt, nil
	}
	template, err := LoadPrompt(d.cfg.PromptDir, tt, conclude)
	if err != nil {
		return "", err
	}
	return renderTemplate(template, d.contextFor(t))
}

func (d *nativeDriver) contextFor(t Task) map[string]string {
	if d.cfg.Context == nil {
		return nil
	}
	return d.cfg.Context(t)
}

// promptPlaceholders is the fixed set understood by the shipped templates.
var promptPlaceholders = []string{"graph_yaml", "fact_ids", "open_intents", "intent_id", "intent_description"}

// renderTemplate substitutes Context values and refuses to send a prompt that
// still contains a known placeholder, which would otherwise reach the model as
// broken context.
func renderTemplate(template string, values map[string]string) (string, error) {
	rendered := RenderPrompt(template, values)
	for _, name := range promptPlaceholders {
		if strings.Contains(rendered, "{"+name+"}") {
			return "", fmt.Errorf("worker: unresolved prompt placeholder {%s}", name)
		}
	}
	return rendered, nil
}

func (d *nativeDriver) complete(ctx context.Context, prompt string) (string, error) {
	if d.cfg.Complete != nil {
		return d.cfg.Complete(ctx, prompt)
	}
	model := strings.TrimSpace(d.cfg.Model)
	if model == "" {
		model = strings.TrimSpace(misc.GetConfigValueDefault(d.cfg.Section, "MODEL",
			misc.GetConfigValueDefault("main_setting", "MODEL", "")))
	}
	if model == "" {
		return "", fmt.Errorf("native worker: model is not configured")
	}
	cli := llm.GetResponsesClient(d.cfg.Section, "main_setting")
	if cli == nil {
		return "", fmt.Errorf("native worker: llm client is not configured for section %q", d.cfg.Section)
	}
	resp, err := llm.RequestLLM(cli, ctx, model, []llm.Message{
		{Role: llm.RoleUser, Content: prompt},
	}, nil)
	if err != nil {
		return "", fmt.Errorf("native worker: llm call failed: %w", err)
	}
	return resp.Content, nil
}

func taskTypeOf(raw string) (TaskType, error) {
	tt := TaskType(strings.TrimSpace(raw))
	switch tt {
	case TaskBootstrap, TaskReason, TaskExplore:
		return tt, nil
	default:
		return "", fmt.Errorf("worker: unknown task type %q", raw)
	}
}

func timeoutContext(ctx context.Context, timeout time.Duration) (context.Context, context.CancelFunc) {
	if timeout <= 0 {
		return context.WithCancel(ctx)
	}
	return context.WithTimeout(ctx, timeout)
}

// LoadPrompt reads a task prompt template from dir. Prompt files are data
// distributed alongside the code, not state-machine logic.
func LoadPrompt(dir string, tt TaskType, conclude bool) (string, error) {
	name, err := promptFileName(tt, conclude)
	if err != nil {
		return "", err
	}
	data, err := os.ReadFile(filepath.Join(dir, name))
	if err != nil {
		return "", fmt.Errorf("worker: load prompt %s: %w", name, err)
	}
	return string(data), nil
}

func promptFileName(tt TaskType, conclude bool) (string, error) {
	switch tt {
	case TaskBootstrap:
		if conclude {
			return "bootstrap_conclude.md", nil
		}
		return "bootstrap.md", nil
	case TaskReason:
		if conclude {
			return "", fmt.Errorf("worker: reason task has no conclude prompt")
		}
		return "reason.md", nil
	case TaskExplore:
		if conclude {
			return "explore_conclude.md", nil
		}
		return "explore.md", nil
	default:
		return "", fmt.Errorf("worker: unknown task type %q", tt)
	}
}

// RenderPrompt replaces {placeholder} occurrences with the supplied values.
func RenderPrompt(template string, values map[string]string) string {
	rendered := template
	for key, value := range values {
		rendered = strings.ReplaceAll(rendered, "{"+key+"}", value)
	}
	return rendered
}

var jsonFenceRE = regexp.MustCompile("(?is)```(?:json)?\\s*(.*?)```")

func stripJSONFence(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if match := jsonFenceRE.FindStringSubmatch(trimmed); match != nil {
		return strings.TrimSpace(match[1])
	}
	return trimmed
}

type resultEnvelope struct {
	Accepted *bool           `json:"accepted"`
	Reason   string          `json:"reason"`
	Data     json.RawMessage `json:"data"`
}

type intentPayload struct {
	From        []string `json:"from"`
	Description string   `json:"description"`
}

type factPayload struct {
	Description string `json:"description"`
}

type completePayload struct {
	From        []string `json:"from"`
	Description string   `json:"description"`
}

// ParseResult validates one raw model output against the per-task-type JSON
// contract. It strips Markdown code fences, treats accepted:false as a task
// failure without writable fields, and rejects missing required fields.
func ParseResult(raw string, tt TaskType) (Result, error) {
	var env resultEnvelope
	if err := json.Unmarshal([]byte(stripJSONFence(raw)), &env); err != nil {
		return Result{}, fmt.Errorf("worker: invalid result JSON: %w", err)
	}
	if env.Accepted == nil {
		return Result{}, fmt.Errorf("worker: result is missing accepted")
	}
	if !*env.Accepted {
		return Result{Accepted: false, Raw: raw}, nil
	}
	if len(env.Data) == 0 || string(env.Data) == "null" {
		return Result{}, fmt.Errorf("worker: accepted result is missing data")
	}

	var data map[string]json.RawMessage
	if err := json.Unmarshal(env.Data, &data); err != nil {
		return Result{}, fmt.Errorf("worker: data must be an object: %w", err)
	}

	result := Result{Accepted: true, Raw: raw}
	switch tt {
	case TaskBootstrap:
		if err := parseBootstrapData(data, &result); err != nil {
			return Result{}, err
		}
	case TaskReason:
		if err := parseReasonData(data, &result); err != nil {
			return Result{}, err
		}
	case TaskExplore:
		description, err := parseDescription(data, "data.description")
		if err != nil {
			return Result{}, err
		}
		result.Description = description
	default:
		return Result{}, fmt.Errorf("worker: unknown task type %q", tt)
	}
	return result, nil
}

func parseBootstrapData(data map[string]json.RawMessage, result *Result) error {
	fact, err := parseFactObject(data["fact"], "data.fact")
	if err != nil {
		return err
	}
	if fact == nil {
		return fmt.Errorf("worker: bootstrap result requires data.fact.description")
	}
	result.Fact = fact

	if completeRaw, ok := data["complete"]; ok {
		complete, err := parseFactObject(completeRaw, "data.complete")
		if err != nil {
			return err
		}
		result.Complete = complete
	}

	intents, err := parseIntents(data["intents"])
	if err != nil {
		return err
	}
	result.Intents = intents
	return nil
}

func parseReasonData(data map[string]json.RawMessage, result *Result) error {
	completeRaw, hasComplete := data["complete"]
	intentsRaw, hasIntents := data["intents"]
	intentRaw, hasIntent := data["intent"]
	if hasComplete && (hasIntents || hasIntent) {
		return fmt.Errorf("worker: complete and intents cannot coexist")
	}
	if hasIntents && hasIntent {
		return fmt.Errorf("worker: intents and intent cannot coexist")
	}

	if hasComplete {
		var complete completePayload
		if err := json.Unmarshal(completeRaw, &complete); err != nil {
			return fmt.Errorf("worker: data.complete must be an object")
		}
		description := strings.TrimSpace(complete.Description)
		if description == "" {
			return fmt.Errorf("worker: data.complete.description is required")
		}
		from := trimmedStrings(complete.From)
		if len(from) == 0 {
			return fmt.Errorf("worker: data.complete.from is required")
		}
		result.Complete = &description
		result.CompleteFrom = from
		return nil
	}

	if hasIntents {
		intents, err := parseIntents(intentsRaw)
		if err != nil {
			return err
		}
		result.Intents = intents
		return nil
	}

	if hasIntent {
		var intent intentPayload
		if err := json.Unmarshal(intentRaw, &intent); err != nil {
			return fmt.Errorf("worker: data.intent must be an object")
		}
		description := strings.TrimSpace(intent.Description)
		if description == "" {
			return fmt.Errorf("worker: data.intent.description is required")
		}
		from := trimmedStrings(intent.From)
		if len(from) == 0 {
			return fmt.Errorf("worker: data.intent.from is required")
		}
		result.Intent = &description
		result.Intents = []IntentSpec{{From: from, Description: description}}
		return nil
	}

	return nil
}

func parseFactObject(raw json.RawMessage, label string) (*string, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	var fact factPayload
	if err := json.Unmarshal(raw, &fact); err != nil {
		return nil, fmt.Errorf("worker: %s must be an object", label)
	}
	description := strings.TrimSpace(fact.Description)
	if description == "" {
		return nil, fmt.Errorf("worker: %s.description is required", label)
	}
	return &description, nil
}

func parseDescription(data map[string]json.RawMessage, label string) (string, error) {
	raw, ok := data["description"]
	if !ok {
		return "", fmt.Errorf("worker: %s is required", label)
	}
	var description string
	if err := json.Unmarshal(raw, &description); err != nil {
		return "", fmt.Errorf("worker: %s must be a string", label)
	}
	description = strings.TrimSpace(description)
	if description == "" {
		return "", fmt.Errorf("worker: %s is required", label)
	}
	return description, nil
}

func parseIntents(raw json.RawMessage) ([]IntentSpec, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	var items []intentPayload
	if err := json.Unmarshal(raw, &items); err != nil {
		return nil, fmt.Errorf("worker: intents must be an array")
	}
	specs := make([]IntentSpec, 0, len(items))
	for i, item := range items {
		description := strings.TrimSpace(item.Description)
		if description == "" {
			return nil, fmt.Errorf("worker: intents[%d].description is required", i)
		}
		from := trimmedStrings(item.From)
		if len(from) == 0 {
			return nil, fmt.Errorf("worker: intents[%d].from is required", i)
		}
		specs = append(specs, IntentSpec{From: from, Description: description})
	}
	return specs, nil
}

func trimmedStrings(values []string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			out = append(out, value)
		}
	}
	return out
}
