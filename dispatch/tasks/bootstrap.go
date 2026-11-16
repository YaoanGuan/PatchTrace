// Package tasks contains the executable graph-lane tasks (bootstrap, reason,
// explore). Each runner turns one dispatcher decision into worker output and
// protocol writes; the graphserver.Client is the only write path.
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
	"AIxVuln/patchintel"
)

// invertPatch is the semantic-inversion seam. The default runner binds it to
// patchintel.Invert; bootstrapDeps.invert lets unit tests run fully offline.
var invertPatch = patchintel.Invert

// bootstrapDeps groups the process-level lookups so tests can exercise the
// runner without opening the SQLite rule base, reading the data directory, or
// calling the LLM.
type bootstrapDeps struct {
	searchRules func(string) []patchintel.Rule
	loadScene   func(string) (string, string)
	loadPrompt  func(string, worker.TaskType, bool) (string, error)
	invert      func(context.Context, *patchintel.PatchProfile, []patchintel.Rule, string) (*patchintel.InversionOutput, error)
	// heartbeatInterval is how often the reserved intent lease is refreshed
	// while the worker runs. Zero means bootstrapIntentLease/2.
	heartbeatInterval time.Duration
}

var defaultBootstrapDeps = bootstrapDeps{
	searchRules: searchDefaultRules,
	loadScene:   patchintel.LoadSceneTemplate,
	loadPrompt:  worker.LoadPrompt,
	invert: func(ctx context.Context, profile *patchintel.PatchProfile, rules []patchintel.Rule, scene string) (*patchintel.InversionOutput, error) {
		return invertPatch(ctx, nil, profile, rules, scene)
	},
	heartbeatInterval: bootstrapIntentLease / 2,
}

// RunBootstrap ingests the project patch, inverts it through the worker and
// writes the result into the provenance graph:
//
//	reserved intent -> claim -> patch profile -> prompt -> driver execute ->
//	inversion -> patch_profile fact -> exploit_idea fact -> inversion intents
//
// A disabled project is a no-op. A project with no usable patch source fails
// before any graph write. Any failure before the reserved intent concludes
// releases it (joining release errors) so a later tick can retry; a claimed or
// already concluded reserved intent is left untouched.
func RunBootstrap(ctx context.Context, c graphserver.Client, d worker.Driver, p graph.Project) error {
	return runBootstrap(ctx, c, d, p, defaultBootstrapDeps)
}

func runBootstrap(ctx context.Context, c graphserver.Client, d worker.Driver, p graph.Project, deps bootstrapDeps) error {
	if !p.BootstrapEnabled {
		return nil
	}
	if c == nil || d == nil {
		return errors.New("bootstrap: graph client and worker driver are required")
	}

	// Resolve the patch source first: a project without a diff must not leave
	// any graph writes behind, including the reserved intent.
	profile, err := loadPatchProfile(ctx, p.Origin)
	if err != nil {
		return err
	}

	export, err := c.ExportYAML(ctx, p.ID)
	if err != nil {
		return fmt.Errorf("bootstrap: read graph for project %s: %w", p.ID, err)
	}
	reserved, err := findReservedBootstrapIntent(export)
	if err != nil {
		return err
	}
	// The planner already suppresses these states; the runner must not
	// double-bootstrap if it is invoked directly.
	if reserved.Claimed || reserved.Concluded {
		return nil
	}

	workerName := strings.TrimSpace(d.Name())
	if workerName == "" {
		workerName = bootstrapWorkerName
	}

	intentID := reserved.OpenID
	if intentID == "" {
		intent, err := c.DeclareIntent(ctx, p.ID, graph.Node{
			Content: reservedBootstrapContent,
			From:    []string{reservedOriginFact},
			Creator: reservedBootstrapCreator,
		})
		if err != nil {
			return fmt.Errorf("bootstrap: create reserved intent: %w", err)
		}
		intentID = intent.ID
	}

	if _, err := c.ClaimIntent(ctx, p.ID, intentID, workerName, bootstrapIntentLease); err != nil {
		return fmt.Errorf("bootstrap: claim reserved intent %s: %w", intentID, err)
	}

	contract, err := deps.loadPrompt(bootstrapPromptDir, worker.TaskBootstrap, false)
	if err != nil {
		return bootstrapFailure(ctx, c, p.ID, intentID, workerName, fmt.Errorf("bootstrap: load prompt: %w", err))
	}
	scene, _ := deps.loadScene(profile.Scene)
	template := strings.TrimRight(contract, "\n") + "\n\n## 场景模板\n\n" + scene + "\n"
	rules := deps.searchRules(bootstrapRuleQuery(profile))
	prompt := bootstrapPrompt(p, profile, rules, template)

	// The worker may run for minutes; keep the claim alive while it does.
	stopHeartbeat := startBootstrapHeartbeat(ctx, c, p.ID, intentID, workerName, deps.heartbeatInterval)
	result, execErr := d.Execute(ctx, worker.Task{
		ProjectID: p.ID,
		IntentID:  intentID,
		Type:      string(worker.TaskBootstrap),
		Prompt:    prompt,
	})
	stopHeartbeat()
	if execErr != nil {
		return bootstrapFailure(ctx, c, p.ID, intentID, workerName, fmt.Errorf("bootstrap: execute task: %w", execErr))
	}
	if !result.Accepted {
		return bootstrapFailure(ctx, c, p.ID, intentID, workerName, errors.New("bootstrap: worker refused the task"))
	}
	if result.Fact == nil || strings.TrimSpace(*result.Fact) == "" {
		return bootstrapFailure(ctx, c, p.ID, intentID, workerName, errors.New("bootstrap: worker returned no fact"))
	}

	// Semantic inversion produces the exploit hypothesis R7 requires. It runs
	// before any conclusion so a failed inversion releases the claimed intent
	// and leaves no half-written facts behind.
	inversion, err := deps.invert(ctx, profile, rules, profile.Scene)
	if err != nil {
		return bootstrapFailure(ctx, c, p.ID, intentID, workerName, fmt.Errorf("bootstrap: inversion failed: %w", err))
	}
	if inversion == nil || inversion.Idea == nil {
		return bootstrapFailure(ctx, c, p.ID, intentID, workerName, errors.New("bootstrap: inversion returned no exploit idea"))
	}
	ideaDescription, err := formatExploitIdea(inversion.Idea)
	if err != nil {
		return bootstrapFailure(ctx, c, p.ID, intentID, workerName, err)
	}

	fact, _, err := c.ConcludeIntent(ctx, p.ID, intentID, workerName,
		strings.TrimSpace(*result.Fact), patchProfileFactType)
	if err != nil {
		return bootstrapFailure(ctx, c, p.ID, intentID, workerName,
			fmt.Errorf("bootstrap: conclude reserved intent %s: %w", intentID, err))
	}
	if strings.TrimSpace(fact.ID) == "" {
		// The intent is already resolved; without a fact id every dependent
		// write would dangle, so surface the inconsistency instead.
		return errors.New("bootstrap: conclude returned no fact id")
	}

	// R7 requires both facts. The exploit idea is written through the same
	// intent protocol as everything else in the graph.
	if err := concludeExploitIdea(ctx, c, p.ID, fact.ID, workerName, ideaDescription); err != nil {
		return err
	}

	// Declare every inversion intent; one failure must not abandon the rest.
	var errs []error
	for i, spec := range result.Intents {
		description := strings.TrimSpace(spec.Description)
		if description == "" {
			continue
		}
		if _, err := c.DeclareIntent(ctx, p.ID, graph.Node{
			Content: description,
			From:    []string{fact.ID},
			Creator: workerName,
		}); err != nil {
			errs = append(errs, fmt.Errorf("bootstrap: declare inversion intent %d: %w", i, err))
		}
	}
	return errors.Join(errs...)
}

// concludeExploitIdea declares, claims and concludes the reserved exploit_idea
// intent, with the patch_profile fact as its parent. The intent is released on
// failure; a successful conclusion is never released.
func concludeExploitIdea(ctx context.Context, c graphserver.Client, projectID, patchFactID, workerName, description string) error {
	intent, err := c.DeclareIntent(ctx, projectID, graph.Node{
		Content: exploitIdeaContent,
		From:    []string{patchFactID},
		Creator: reservedBootstrapCreator,
	})
	if err != nil {
		return fmt.Errorf("bootstrap: declare exploit_idea intent: %w", err)
	}
	if _, err := c.ClaimIntent(ctx, projectID, intent.ID, workerName, bootstrapIntentLease); err != nil {
		return fmt.Errorf("bootstrap: claim exploit_idea intent %s: %w", intent.ID, err)
	}
	if _, _, err := c.ConcludeIntent(ctx, projectID, intent.ID, workerName,
		description, exploitIdeaFactType); err != nil {
		return bootstrapFailure(ctx, c, projectID, intent.ID, workerName,
			fmt.Errorf("bootstrap: conclude exploit_idea intent %s: %w", intent.ID, err))
	}
	return nil
}

// formatExploitIdea renders the inversion idea as the exploit_idea fact body.
// The idea is marshalled generically so dispatch/tasks does not need to import
// taskManager.
func formatExploitIdea(idea any) (string, error) {
	data, err := json.MarshalIndent(idea, "", "  ")
	if err != nil {
		return "", fmt.Errorf("bootstrap: format exploit idea: %w", err)
	}
	return "补丁语义反演得到的利用假设：\n" + string(data), nil
}

// bootstrapPrompt renders the bootstrap contract template and appends the
// patch profile and top-k rule context. The native driver sends a non-empty
// Task.Prompt verbatim, so every placeholder is resolved here.
func bootstrapPrompt(p graph.Project, profile *patchintel.PatchProfile, rules []patchintel.Rule, template string) string {
	rendered := worker.RenderPrompt(template, map[string]string{
		"graph_yaml": minimalGraphYAML(p),
		"fact_ids":   bootstrapFactIDsJSON(),
	})
	var b strings.Builder
	b.WriteString(strings.TrimRight(rendered, "\n"))
	b.WriteString("\n\n## 补丁档案\n\n")
	b.WriteString(formatPatchProfile(profile))
	b.WriteString("\n\n## 规则库注入（top-k）\n\n")
	b.WriteString(patchintel.FormatRules(rules))
	b.WriteString("\n\n## 输出要求\n\n")
	b.WriteString(`- ` + "`intents.from`" + ` 会被覆盖为随后写入的 patch_profile 事实 id，` +
		`请专注于给出可验证的意图描述，无需自行填写 from。` + "\n")
	return b.String()
}

// startBootstrapHeartbeat refreshes the claimed intent lease until the returned
// stop function is called. The stop function waits for the goroutine, so the
// caller sees a fully quiesced heartbeat before concluding.
func startBootstrapHeartbeat(ctx context.Context, c graphserver.Client, projectID, intentID, workerName string, interval time.Duration) func() {
	if interval <= 0 {
		interval = bootstrapIntentLease / 2
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
				if _, err := c.HeartbeatIntent(hbCtx, projectID, intentID, workerName, bootstrapIntentLease); err != nil {
					misc.Warn("dispatch.tasks", fmt.Sprintf("bootstrap: heartbeat intent %s: %v", intentID, err), nil)
				}
			}
		}
	}()
	return func() {
		cancel()
		<-done
	}
}

// bootstrapFailure joins the original failure with a release attempt so a
// failed release is never silently swallowed.
func bootstrapFailure(ctx context.Context, c graphserver.Client, projectID, intentID, workerName string, cause error) error {
	return errors.Join(cause, releaseBootstrapIntent(ctx, c, projectID, intentID, workerName))
}

// releaseBootstrapIntent returns the claimed intent to the open pool.
func releaseBootstrapIntent(ctx context.Context, c graphserver.Client, projectID, intentID, workerName string) error {
	if _, err := c.ReleaseIntent(ctx, projectID, intentID, workerName); err != nil {
		misc.Warn("dispatch.tasks", fmt.Sprintf("bootstrap: release intent %s: %v", intentID, err), nil)
		return fmt.Errorf("bootstrap: release intent %s: %w", intentID, err)
	}
	return nil
}
