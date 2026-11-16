package tasks

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"AIxVuln/dispatch/runtime"
	"AIxVuln/dispatch/worker"
	"AIxVuln/graph"
	"AIxVuln/graphserver"
	"AIxVuln/misc"

	yaml "github.com/goccy/go-yaml"
)

// Explore runtime defaults. The runner signature carries no dispatcher config,
// so these mirror the graph protocol / native driver defaults.
const (
	explorePromptDir       = "prompts/graph/default"
	exploreWorkerName      = "explore"
	exploreIntentLease     = 5 * time.Minute
	exploreConcludeTimeout = 2 * time.Minute
	exploreObservationType = "observation"
	exploreMaxReasonRunes  = 160
	exploreFailurePrefix   = "explore 未完成："
)

// exploreDeps groups the injectable lookups so tests can exercise the runner
// without touching the disk and can observe the heartbeat at a small interval.
type exploreDeps struct {
	loadPrompt        func(string, worker.TaskType, bool) (string, error)
	heartbeatInterval time.Duration
	concludeTimeout   time.Duration
}

var defaultExploreDeps = exploreDeps{
	loadPrompt:        worker.LoadPrompt,
	heartbeatInterval: exploreIntentLease / 2,
	concludeTimeout:   exploreConcludeTimeout,
}

// verdictWriter is optionally implemented by a graph client that can persist
// verdict facts. The mandated graphserver.Client interface has no AddVerdict
// method (the same optional-seam pattern the reason runner uses for GapScore),
// so dispatcher wiring must supply a store-backed wrapper when verdicts are
// expected. A protocol-only client that cannot write verdicts surfaces an
// error instead of silently dropping R10 evidence.
type verdictWriter interface {
	AddVerdict(ctx context.Context, projectID, targetID, verdict, description, evidenceRef string) (graph.Node, error)
}

// RunExplore validates one claimed intent inside the sandbox and writes an
// objective conclusion back to the graph (R9), plus an optional evidence
// verdict (R10):
//
//	claim intent -> render explore prompt -> driver execute ->
//	ConcludeIntent(observation) -> optional AddVerdict(support|refute)
//
// A claim conflict means another worker owns the intent and is a silent skip. A
// missing, inactive or otherwise unusable claim is an error. The claim is
// heartbeated at lease/2 while the driver executes and is released on any
// failure before a conclusion; it is never released after ConcludeIntent.
//
// A timeout, execute error, contract violation or refusal is still a valid
// conclusion: the runner either asks the same session to summarise through the
// driver's conclude prompt (with the caller's cancellation detached and a fresh
// timeout) or writes an objective "explore 未完成" failure fact. It never
// fabricates a successful conclusion.
//
// rt is the sandbox execution seam. The runner itself does not exec commands:
// the worker driver owns tool execution, so rt is carried for dispatcher wiring
// rather than used here.
func RunExplore(ctx context.Context, c graphserver.Client, d worker.Driver, rt runtime.SandboxRuntime, p graph.Project, intentID string) error {
	return runExplore(ctx, c, d, rt, p, intentID, defaultExploreDeps)
}

func runExplore(ctx context.Context, c graphserver.Client, d worker.Driver, rt runtime.SandboxRuntime, p graph.Project, intentID string, deps exploreDeps) error {
	if c == nil || d == nil {
		return errors.New("explore: graph client and worker driver are required")
	}
	_ = rt

	intentID = strings.TrimSpace(intentID)
	workerName := strings.TrimSpace(d.Name())
	if workerName == "" {
		workerName = exploreWorkerName
	}

	if _, err := c.ClaimIntent(ctx, p.ID, intentID, workerName, exploreIntentLease); err != nil {
		if errors.Is(err, graph.ErrConflict) {
			// Another worker owns the intent; a later tick can retry.
			return nil
		}
		return fmt.Errorf("explore: claim intent %s: %w", intentID, err)
	}

	// fail releases the claim only before a conclusion. Once ConcludeIntent has
	// succeeded the intent is resolved and must never be reopened here.
	concluded := false
	fail := func(cause error) error {
		if concluded {
			return cause
		}
		if _, relErr := c.ReleaseIntent(ctx, p.ID, intentID, workerName); relErr != nil {
			misc.Warn("dispatch.tasks", fmt.Sprintf("explore: release intent %s: %v", intentID, relErr), nil)
			return errors.Join(cause, fmt.Errorf("explore: release intent %s: %w", intentID, relErr))
		}
		return cause
	}

	export, err := c.ExportYAML(ctx, p.ID)
	if err != nil {
		return fail(fmt.Errorf("explore: read graph for project %s: %w", p.ID, err))
	}
	intentDescription, nodeIDs, err := parseExploreExport(export, intentID)
	if err != nil {
		return fail(err)
	}

	template, err := deps.loadPrompt(explorePromptDir, worker.TaskExplore, false)
	if err != nil {
		return fail(fmt.Errorf("explore: load prompt: %w", err))
	}
	prompt, err := explorePrompt(template, strings.TrimSpace(export), intentID, intentDescription)
	if err != nil {
		return fail(err)
	}

	task := worker.Task{
		ProjectID: p.ID,
		IntentID:  intentID,
		Type:      string(worker.TaskExplore),
		Prompt:    prompt,
	}

	// The driver may run for minutes; keep the claim alive while it does. The
	// deferred stop makes a panic inside Execute unable to leak the goroutine.
	stopHeartbeat := startExploreHeartbeat(ctx, c, p.ID, intentID, workerName, deps.heartbeatInterval)
	defer stopHeartbeat()

	result, execErr := d.Execute(ctx, task)
	stopHeartbeat()

	// conclusion is always non-empty. verdictRaw is set only for an accepted
	// execute result: a verdict is evidence produced by a completed
	// exploration, not by a timeout fallback or a refusal.
	var (
		conclusion string
		verdictRaw string
	)
	switch {
	case execErr != nil:
		conclusion = exploreFallbackConclusion(ctx, d, task, deps, fmt.Errorf("explore: execute task: %w", execErr))
	case !result.Accepted:
		conclusion = exploreFailureDescription("worker refused the task")
	case strings.TrimSpace(result.Description) == "":
		conclusion = exploreFallbackConclusion(ctx, d, task, deps, errors.New("explore: worker returned no description"))
	default:
		conclusion = strings.TrimSpace(result.Description)
		verdictRaw = result.Raw
	}

	if _, _, err := c.ConcludeIntent(ctx, p.ID, intentID, workerName, conclusion, exploreObservationType); err != nil {
		return fail(fmt.Errorf("explore: conclude intent %s: %w", intentID, err))
	}
	concluded = true

	if verdictRaw == "" {
		return nil
	}
	return recordExploreVerdict(ctx, c, p.ID, intentID, conclusion, verdictRaw, nodeIDs)
}

// exploreFallbackConclusion asks the same session for a closing summary when it
// supports the conclude prompt, and otherwise writes the objective failure fact
// R9 requires. The summary must be a non-empty accepted description; any
// failure joins the original cause and falls back to the failure fact.
func exploreFallbackConclusion(ctx context.Context, d worker.Driver, task worker.Task, deps exploreDeps, cause error) string {
	if d.SupportsConclude() {
		description, err := runExploreConclude(ctx, d, task, deps)
		if err == nil {
			return description
		}
		cause = errors.Join(cause, err)
	}
	return exploreFailureDescription(cause.Error())
}

// runExploreConclude runs the driver's conclude prompt against a context that
// ignores the caller's cancellation but keeps its values, bounded by a fresh
// timeout. Task.Prompt is cleared because the native driver always loads the
// explore_conclude template and must never resubmit the execute prompt.
func runExploreConclude(ctx context.Context, d worker.Driver, task worker.Task, deps exploreDeps) (string, error) {
	timeout := deps.concludeTimeout
	if timeout <= 0 {
		timeout = exploreConcludeTimeout
	}
	concludeCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), timeout)
	defer cancel()

	task.Prompt = ""
	task.ConcludeTimeout = timeout
	result, err := d.Conclude(concludeCtx, task)
	if err != nil {
		return "", fmt.Errorf("explore: conclude task: %w", err)
	}
	if !result.Accepted {
		return "", errors.New("explore: worker refused conclude")
	}
	description := strings.TrimSpace(result.Description)
	if description == "" {
		return "", errors.New("explore: conclude returned no description")
	}
	return description, nil
}

// exploreFailureDescription renders the objective failure fact. The reason is
// collapsed to one line and truncated so long driver errors cannot flood the
// graph.
func exploreFailureDescription(reason string) string {
	reason = strings.Join(strings.Fields(reason), " ")
	if reason == "" {
		reason = "未知原因"
	}
	runes := []rune(reason)
	if len(runes) > exploreMaxReasonRunes {
		reason = string(runes[:exploreMaxReasonRunes]) + "…"
	}
	return exploreFailurePrefix + reason
}

// exploreExportDoc is the subset of the YAML export the explore runner needs:
// the intent description and every node id (for data.target validation).
type exploreExportDoc struct {
	Hints []struct {
		ID string `yaml:"id"`
	} `yaml:"hints"`
	Facts []struct {
		ID string `yaml:"id"`
	} `yaml:"facts"`
	Intents []struct {
		ID      string `yaml:"id"`
		Content string `yaml:"content"`
	} `yaml:"intents"`
}

// parseExploreExport finds the claimed intent's description and collects every
// node id in the project. A missing or descriptionless intent is an error: the
// prompt cannot carry the exploration direction otherwise.
func parseExploreExport(export, intentID string) (string, map[string]struct{}, error) {
	var doc exploreExportDoc
	if strings.TrimSpace(export) != "" {
		if err := yaml.Unmarshal([]byte(export), &doc); err != nil {
			return "", nil, fmt.Errorf("explore: parse graph export: %w", err)
		}
	}

	nodeIDs := make(map[string]struct{}, len(doc.Hints)+len(doc.Facts)+len(doc.Intents))
	for _, h := range doc.Hints {
		if id := strings.TrimSpace(h.ID); id != "" {
			nodeIDs[id] = struct{}{}
		}
	}
	for _, f := range doc.Facts {
		if id := strings.TrimSpace(f.ID); id != "" {
			nodeIDs[id] = struct{}{}
		}
	}

	var description string
	found := false
	for _, in := range doc.Intents {
		id := strings.TrimSpace(in.ID)
		if id == "" {
			continue
		}
		nodeIDs[id] = struct{}{}
		if id == intentID {
			found = true
			description = strings.TrimSpace(in.Content)
		}
	}
	if !found {
		return "", nil, fmt.Errorf("explore: intent %s not found in graph export", intentID)
	}
	if description == "" {
		return "", nil, fmt.Errorf("explore: intent %s has no description", intentID)
	}
	return description, nodeIDs, nil
}

// explorePrompt renders the explore contract. Substitution is ordered so a
// placeholder that a value itself injects is still detected as unresolved
// instead of reaching the model as broken context.
func explorePrompt(template, graphYAML, intentID, intentDescription string) (string, error) {
	values := []struct{ name, value string }{
		{"graph_yaml", graphYAML},
		{"intent_id", intentID},
		{"intent_description", intentDescription},
	}
	rendered := template
	for _, v := range values {
		rendered = strings.ReplaceAll(rendered, "{"+v.name+"}", v.value)
	}
	for _, v := range values {
		if strings.Contains(rendered, "{"+v.name+"}") {
			return "", fmt.Errorf("explore: unresolved prompt placeholder {%s}", v.name)
		}
	}
	return rendered, nil
}

// exploreVerdictPayload is the optional R10 section of the explore result.
type exploreVerdictPayload struct {
	Data struct {
		Verdict     string `json:"verdict"`
		EvidenceRef string `json:"evidence_ref"`
		Target      string `json:"target"`
	} `json:"data"`
}

var exploreFenceRE = regexp.MustCompile("(?is)```(?:json)?\\s*(.*?)```")

func stripExploreFence(raw string) string {
	if match := exploreFenceRE.FindStringSubmatch(raw); match != nil {
		return strings.TrimSpace(match[1])
	}
	return raw
}

// recordExploreVerdict parses the optional verdict fields and writes a verdict
// fact through AddVerdict. The default target is the explored intent; an
// explicit data.target only overrides it when it names an existing project
// node. A verdict failure is returned after the observation fact is already
// written.
func recordExploreVerdict(ctx context.Context, c graphserver.Client, projectID, intentID, description, raw string, nodeIDs map[string]struct{}) error {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil
	}
	var envelope exploreVerdictPayload
	if err := json.Unmarshal([]byte(stripExploreFence(trimmed)), &envelope); err != nil {
		return fmt.Errorf("explore: parse verdict data: %w", err)
	}
	verdict := strings.TrimSpace(envelope.Data.Verdict)
	if verdict == "" {
		return nil
	}
	if verdict != "support" && verdict != "refute" {
		return fmt.Errorf("%w: explore: verdict %q must be support or refute", graph.ErrInvalid, verdict)
	}

	targetID := intentID
	if requested := strings.TrimSpace(envelope.Data.Target); requested != "" {
		if _, ok := nodeIDs[requested]; ok {
			targetID = requested
		}
	}

	writer, ok := c.(verdictWriter)
	if !ok {
		return errors.New("explore: graph client cannot record verdicts (missing AddVerdict)")
	}
	if _, err := writer.AddVerdict(ctx, projectID, targetID, verdict, description, strings.TrimSpace(envelope.Data.EvidenceRef)); err != nil {
		return fmt.Errorf("explore: record verdict: %w", err)
	}
	return nil
}

// startExploreHeartbeat refreshes the claimed intent lease until the returned
// stop function is called. The stop function waits for the goroutine, so the
// caller sees a fully quiesced heartbeat before concluding, and a second call
// is a no-op.
func startExploreHeartbeat(ctx context.Context, c graphserver.Client, projectID, intentID, workerName string, interval time.Duration) func() {
	if interval <= 0 {
		interval = exploreIntentLease / 2
	}
	hbCtx, cancel := context.WithCancel(ctx)
	done := make(chan struct{})
	go func() {
		defer close(done)
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-hbCtx.Done():
				return
			case <-ticker.C:
				if _, err := c.HeartbeatIntent(hbCtx, projectID, intentID, workerName, exploreIntentLease); err != nil {
					misc.Warn("dispatch.tasks", fmt.Sprintf("explore: heartbeat intent %s: %v", intentID, err), nil)
				}
			}
		}
	}()
	return func() {
		cancel()
		<-done
	}
}
