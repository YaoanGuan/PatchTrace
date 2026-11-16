package tasks

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"AIxVuln/dispatch/worker"
	"AIxVuln/graph"
	"AIxVuln/graphserver"
	"AIxVuln/misc"

	yaml "github.com/goccy/go-yaml"
)

// Reason runtime defaults. The runner signature carries no dispatcher config,
// so these mirror the graph protocol / native driver defaults.
const (
	reasonPromptDir  = "prompts/graph/default"
	reasonWorkerName = "reason"
	reasonTrigger    = "reason"
	reasonLease      = 5 * time.Minute
	reasonGapTopK    = 5
)

// ErrReasonLeaseHeld reports that the project reason lease could not be
// claimed: another worker owns it, the project is gone, or it is not active.
// The reason runner writes nothing in that case; the dispatcher treats it as a
// skip (neither a failure nor a successful run that advances the baseline).
var ErrReasonLeaseHeld = errors.New("dispatch.tasks: reason lease held by another worker")

// fallbackReasonBody is used when the shipped template cannot be read. It must
// carry the same placeholders as reason.md so a missing file never silently
// drops the graph snapshot, fact ids or open intents from the model context.
const fallbackReasonBody = `# 任务

阅读溯源图快照，判断目标是否达成；未达成时提出新的探索方向。

## 图谱快照

` + "```yaml\n{graph_yaml}\n```" + `

## 可用事实编号

` + "```json\n{fact_ids}\n```" + `

## 开放意图

` + "```json\n{open_intents}\n```" + "\n"

// reasonPromptLoader reads the shipped reason contract. It is a variable so
// tests can force the fallback path and future wiring can substitute a
// template without touching the disk.
var reasonPromptLoader = func(dir string, tt worker.TaskType, conclude bool) (string, error) {
	return worker.LoadPrompt(dir, tt, conclude)
}

// reasonDeps groups the process-level lookups so tests can exercise the
// runner without reading prompt files or opening a graph store.
type reasonDeps struct {
	// gaps ranks the project's open intents. A graphserver.Client has no edge
	// endpoint, so only a client that also implements GapScore can provide the
	// authoritative ranking; otherwise gaps are omitted from the prompt.
	gaps func(context.Context, graphserver.Client, string) ([]graph.Gap, error)
	// heartbeatInterval is how often the project reason lease is refreshed
	// while the worker runs. Zero means reasonLease/2.
	heartbeatInterval time.Duration
}

var defaultReasonDeps = reasonDeps{
	gaps:              reasonGaps,
	heartbeatInterval: reasonLease / 2,
}

// gapScorer is optionally implemented by a graph client that can rank open
// intents from the authoritative edge set.
type gapScorer interface {
	GapScore(ctx context.Context, projectID string) ([]graph.Gap, error)
}

func reasonGaps(ctx context.Context, c graphserver.Client, projectID string) ([]graph.Gap, error) {
	scorer, ok := c.(gapScorer)
	if !ok {
		// The HTTP protocol has no edge endpoint, so a client that only speaks
		// the protocol can never rank gaps. Keep running (the gap section is an
		// ordering aid, not a precondition) but make the omission observable.
		misc.Warn("dispatch.tasks", "reason: graph client does not implement GapScore; gap ranking omitted", nil)
		return nil, nil
	}
	return scorer.GapScore(ctx, projectID)
}

// RunReason reads the provenance graph, asks the worker whether the goal is
// covered, and applies exactly one of the reason outcomes:
//
//	complete -> Client.Complete(from, description)
//	intent   -> Client.DeclareIntent(from, description)
//	empty    -> no write
//
// The project-level reason lease is claimed with trigger "reason" and a 5m
// lease. A lease held elsewhere, or a missing/inactive project, returns an
// error wrapping ErrReasonLeaseHeld and writes nothing so another worker (or a
// later tick) can take it; the dispatcher treats that sentinel as a skip. While the
// worker runs the lease is heartbeated at lease/2, and it is always released
// before returning, including on failure; release errors are joined.
//
// A refused result returns an error and writes nothing. Complete and intent
// from facts are validated against the exported fact set first, so an unknown
// fact yields graph.ErrInvalid with no writes.
func RunReason(ctx context.Context, c graphserver.Client, d worker.Driver, p graph.Project) error {
	return runReason(ctx, c, d, p, defaultReasonDeps)
}

func runReason(ctx context.Context, c graphserver.Client, d worker.Driver, p graph.Project, deps reasonDeps) (err error) {
	if c == nil || d == nil {
		return errors.New("reason: graph client and worker driver are required")
	}

	workerName := strings.TrimSpace(d.Name())
	if workerName == "" {
		workerName = reasonWorkerName
	}

	if _, claimErr := c.ClaimReason(ctx, p.ID, workerName, reasonTrigger, reasonLease); claimErr != nil {
		if errors.Is(claimErr, graph.ErrConflict) ||
			errors.Is(claimErr, graph.ErrNotFound) ||
			errors.Is(claimErr, graph.ErrForbidden) {
			return fmt.Errorf("%w: claim project %s: %w", ErrReasonLeaseHeld, p.ID, claimErr)
		}
		return fmt.Errorf("reason: claim reason lease for project %s: %w", p.ID, claimErr)
	}
	defer func() {
		releaseCtx, cancelRelease := detachedGraphWriteContext(ctx)
		defer cancelRelease()
		if _, releaseErr := c.ReleaseReason(releaseCtx, p.ID, workerName); releaseErr != nil {
			misc.Warn("dispatch.tasks", fmt.Sprintf("reason: release reason lease for project %s: %v", p.ID, releaseErr), nil)
			err = errors.Join(err, fmt.Errorf("reason: release reason lease for project %s: %w", p.ID, releaseErr))
		}
	}()

	export, err := c.ExportYAML(ctx, p.ID)
	if err != nil {
		return fmt.Errorf("reason: read graph for project %s: %w", p.ID, err)
	}
	inputs, err := parseReasonExport(export)
	if err != nil {
		return err
	}

	scored, err := deps.gaps(ctx, c, p.ID)
	if err != nil {
		return fmt.Errorf("reason: score gaps for project %s: %w", p.ID, err)
	}
	if len(scored) > reasonGapTopK {
		scored = scored[:reasonGapTopK]
	}

	prompt := reasonPrompt(p, export, inputs.FactIDs, inputs.OpenIntentIDs, formatGapLines(scored), inputs.Hints)

	stopHeartbeat := startReasonHeartbeat(ctx, c, p.ID, workerName, deps.heartbeatInterval)
	defer stopHeartbeat()
	result, execErr := d.Execute(ctx, worker.Task{
		ProjectID: p.ID,
		Type:      string(worker.TaskReason),
		Prompt:    prompt,
	})
	stopHeartbeat()
	if execErr != nil {
		return fmt.Errorf("reason: execute task: %w", execErr)
	}
	if !result.Accepted {
		return errors.New("reason: worker refused the task")
	}
	return applyReasonResult(ctx, c, p.ID, workerName, inputs.FactSet, result)
}

// applyReasonResult validates and writes one accepted reason result. Validation
// happens before any write so an invalid result leaves the graph untouched.
func applyReasonResult(ctx context.Context, c graphserver.Client, projectID, workerName string, facts map[string]struct{}, result worker.Result) error {
	switch {
	case result.Complete != nil && strings.TrimSpace(*result.Complete) != "":
		if len(result.CompleteFrom) == 0 {
			return fmt.Errorf("%w: reason: complete requires at least one from fact", graph.ErrInvalid)
		}
		from := make([]string, 0, len(result.CompleteFrom))
		for _, raw := range result.CompleteFrom {
			id := strings.TrimSpace(raw)
			if id == "" {
				return fmt.Errorf("%w: reason: complete references an empty fact id", graph.ErrInvalid)
			}
			if _, ok := facts[id]; !ok {
				return fmt.Errorf("%w: reason: complete references unknown fact %q", graph.ErrInvalid, id)
			}
			from = append(from, id)
		}
		if _, err := c.Complete(ctx, projectID, from, strings.TrimSpace(*result.Complete), workerName); err != nil {
			return fmt.Errorf("reason: complete project %s: %w", projectID, err)
		}
		return nil

	case result.Intent != nil || len(result.Intents) > 0:
		if len(result.Intents) == 0 {
			return fmt.Errorf("%w: reason: intent is missing its from facts", graph.ErrInvalid)
		}
		descriptions := make([]string, len(result.Intents))
		froms := make([][]string, len(result.Intents))
		for i, spec := range result.Intents {
			description := strings.TrimSpace(spec.Description)
			if description == "" {
				return fmt.Errorf("%w: reason: intent %d description is required", graph.ErrInvalid, i)
			}
			if len(spec.From) == 0 {
				return fmt.Errorf("%w: reason: intent %d requires at least one from fact", graph.ErrInvalid, i)
			}
			from := make([]string, 0, len(spec.From))
			for _, raw := range spec.From {
				id := strings.TrimSpace(raw)
				if id == "" {
					return fmt.Errorf("%w: reason: intent %d references an empty fact id", graph.ErrInvalid, i)
				}
				if _, ok := facts[id]; !ok {
					return fmt.Errorf("%w: reason: intent %d references unknown fact %q", graph.ErrInvalid, i, id)
				}
				from = append(from, id)
			}
			descriptions[i] = description
			froms[i] = from
		}
		var errs []error
		for i := range descriptions {
			if _, err := c.DeclareIntent(ctx, projectID, graph.Node{
				Content: descriptions[i],
				From:    froms[i],
				Creator: workerName,
			}); err != nil {
				errs = append(errs, fmt.Errorf("reason: declare intent %d: %w", i, err))
			}
		}
		return errors.Join(errs...)

	default:
		return nil
	}
}

// reasonExportDoc is the subset of the YAML export the reason runner needs.
type reasonExportDoc struct {
	Hints []struct {
		Content string `yaml:"content"`
	} `yaml:"hints"`
	Facts []struct {
		ID string `yaml:"id"`
	} `yaml:"facts"`
	Intents []struct {
		ID      string `yaml:"id"`
		Content string `yaml:"content"`
		State   string `yaml:"state"`
	} `yaml:"intents"`
}

type reasonInputs struct {
	FactIDs       []string
	FactSet       map[string]struct{}
	OpenIntentIDs []string
	Hints         []string
}

// parseReasonExport extracts the prompt context and the fact set used to
// validate complete/intent references.
func parseReasonExport(export string) (reasonInputs, error) {
	var doc reasonExportDoc
	if strings.TrimSpace(export) != "" {
		if err := yaml.Unmarshal([]byte(export), &doc); err != nil {
			return reasonInputs{}, fmt.Errorf("reason: parse graph export: %w", err)
		}
	}
	inputs := reasonInputs{
		FactIDs:       make([]string, 0, len(doc.Facts)),
		FactSet:       make(map[string]struct{}, len(doc.Facts)),
		OpenIntentIDs: make([]string, 0),
		Hints:         make([]string, 0, len(doc.Hints)),
	}
	for _, f := range doc.Facts {
		id := strings.TrimSpace(f.ID)
		if id == "" {
			continue
		}
		inputs.FactIDs = append(inputs.FactIDs, id)
		inputs.FactSet[id] = struct{}{}
	}
	for _, h := range doc.Hints {
		if content := strings.TrimSpace(h.Content); content != "" {
			inputs.Hints = append(inputs.Hints, content)
		}
	}
	for _, in := range doc.Intents {
		if strings.TrimSpace(in.State) != "open" {
			continue
		}
		id := strings.TrimSpace(in.ID)
		if id == "" {
			continue
		}
		if content := strings.TrimSpace(in.Content); content != "" {
			id += "：" + content
		}
		inputs.OpenIntentIDs = append(inputs.OpenIntentIDs, id)
	}
	return inputs, nil
}

func formatGapLines(gaps []graph.Gap) []string {
	lines := make([]string, 0, len(gaps))
	for _, g := range gaps {
		lines = append(lines, fmt.Sprintf("%s：未满足阻塞 %d，距原点深度 %d；%s",
			g.IntentID, g.OpenBlockers, g.DepthFromOrigin, strings.TrimSpace(g.Description)))
	}
	return lines
}

// reasonPrompt loads the shipped reason contract and renders every placeholder
// plus the ranked-gap and hint context. The native driver sends a non-empty
// Task.Prompt verbatim, so the runner never relies on template loading.
//
// The mandated signature carries no template parameter; the shipped template is
// read from reasonPromptDir through reasonPromptLoader. A missing or unusable
// template falls back to fallbackReasonBody, which carries the same
// placeholders, so the model still receives the graph snapshot, fact ids and
// open intents.
func reasonPrompt(p graph.Project, graphYAML string, factIDs, openIntentIDs, gaps, hints []string) string {
	template, err := reasonPromptLoader(reasonPromptDir, worker.TaskReason, false)
	if err != nil {
		misc.Warn("dispatch.tasks", fmt.Sprintf("reason: load prompt: %v; using fallback template", err), nil)
		template = fallbackReasonBody
	}
	prompt, err := renderReasonPrompt(template, graphYAML, factIDs, openIntentIDs, gaps, hints)
	if err == nil {
		return prompt
	}
	misc.Warn("dispatch.tasks", fmt.Sprintf("reason: render prompt: %v; using fallback template", err), nil)
	prompt, _ = renderReasonPrompt(fallbackReasonBody, graphYAML, factIDs, openIntentIDs, gaps, hints)
	return prompt
}

// renderReasonPrompt substitutes the three context placeholders in a fixed
// order, rejects a template that still contains one afterwards, and appends the
// gap ranking and hint sections.
func renderReasonPrompt(template string, graphYAML string, factIDs, openIntentIDs, gaps, hints []string) (string, error) {
	values := []struct{ name, value string }{
		{"graph_yaml", strings.TrimSpace(graphYAML)},
		{"fact_ids", jsonStringList(factIDs)},
		{"open_intents", jsonStringList(openIntentIDs)},
	}
	rendered := template
	for _, v := range values {
		rendered = strings.ReplaceAll(rendered, "{"+v.name+"}", v.value)
	}
	for _, v := range values {
		if strings.Contains(rendered, "{"+v.name+"}") {
			return "", fmt.Errorf("reason: unresolved prompt placeholder {%s}", v.name)
		}
	}

	var b strings.Builder
	b.WriteString(strings.TrimRight(rendered, "\n"))
	b.WriteString("\n\n## 缺口排序（top-5）\n\n```json\n")
	b.WriteString(jsonStringList(topStrings(gaps, reasonGapTopK)))
	b.WriteString("\n```\n\n## 人工提示\n\n```json\n")
	b.WriteString(jsonStringList(hints))
	b.WriteString("\n```\n")
	return b.String(), nil
}

func topStrings(values []string, n int) []string {
	if n <= 0 || len(values) <= n {
		return values
	}
	return values[:n]
}

func jsonStringList(values []string) string {
	if len(values) == 0 {
		return "[]"
	}
	data, err := json.Marshal(values)
	if err != nil {
		return "[]"
	}
	return string(data)
}

// startReasonHeartbeat refreshes the project reason lease until the returned
// stop function is called. The stop function waits for the goroutine, so the
// caller sees a fully quiesced heartbeat before releasing.
func startReasonHeartbeat(ctx context.Context, c graphserver.Client, projectID, workerName string, interval time.Duration) func() {
	if interval <= 0 {
		interval = reasonLease / 2
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
				if _, err := c.HeartbeatReason(hbCtx, projectID, workerName, reasonLease); err != nil {
					misc.Warn("dispatch.tasks", fmt.Sprintf("reason: heartbeat project %s: %v", projectID, err), nil)
				}
			}
		}
	}()
	return func() {
		cancel()
		<-done
	}
}
