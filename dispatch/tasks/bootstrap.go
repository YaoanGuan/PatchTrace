// Package tasks contains the executable graph-lane tasks (bootstrap, reason,
// explore). Each runner turns one dispatcher decision into worker output and
// protocol writes; the graphserver.Client is the only write path.
package tasks

import (
	"context"
	"fmt"
	"strings"

	"AIxVuln/dispatch/worker"
	"AIxVuln/graph"
	"AIxVuln/graphserver"
	"AIxVuln/patchintel"
)

// bootstrapDeps groups the process-level lookups so tests can exercise the
// runner without opening the SQLite rule base or reading the data directory.
type bootstrapDeps struct {
	searchRules func(string) []patchintel.Rule
	loadScene   func(string) (string, string)
	loadPrompt  func(string, worker.TaskType, bool) (string, error)
}

var defaultBootstrapDeps = bootstrapDeps{
	searchRules: searchDefaultRules,
	loadScene:   patchintel.LoadSceneTemplate,
	loadPrompt:  worker.LoadPrompt,
}

// RunBootstrap ingests the project patch, inverts it through the worker and
// writes the result into the provenance graph:
//
//	reserved intent -> claim -> patch profile -> prompt -> driver execute ->
//	conclude patch_profile fact -> declare inversion intents
//
// A project with no usable patch source fails before any graph write. Any
// failure after claiming releases the reserved intent so a later tick can
// retry it; a claimed or already concluded reserved intent is left untouched.
func RunBootstrap(ctx context.Context, c graphserver.Client, d worker.Driver, p graph.Project) error {
	return runBootstrap(ctx, c, d, p, defaultBootstrapDeps)
}

func runBootstrap(ctx context.Context, c graphserver.Client, d worker.Driver, p graph.Project, deps bootstrapDeps) error {
	if c == nil || d == nil {
		return fmt.Errorf("bootstrap: graph client and worker driver are required")
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
		releaseBootstrapIntent(ctx, c, p.ID, intentID, workerName)
		return fmt.Errorf("bootstrap: load prompt: %w", err)
	}
	scene, _ := deps.loadScene(profile.Scene)
	template := strings.TrimRight(contract, "\n") + "\n\n## 场景模板\n\n" + scene + "\n"
	rules := deps.searchRules(bootstrapRuleQuery(profile))
	prompt := bootstrapPrompt(p, profile, rules, template)

	result, err := d.Execute(ctx, worker.Task{
		ProjectID: p.ID,
		IntentID:  intentID,
		Type:      string(worker.TaskBootstrap),
		Prompt:    prompt,
	})
	if err != nil {
		releaseBootstrapIntent(ctx, c, p.ID, intentID, workerName)
		return fmt.Errorf("bootstrap: execute task: %w", err)
	}
	if !result.Accepted {
		releaseBootstrapIntent(ctx, c, p.ID, intentID, workerName)
		return fmt.Errorf("bootstrap: worker refused the task")
	}
	if result.Fact == nil || strings.TrimSpace(*result.Fact) == "" {
		releaseBootstrapIntent(ctx, c, p.ID, intentID, workerName)
		return fmt.Errorf("bootstrap: worker returned no fact")
	}

	fact, _, err := c.ConcludeIntent(ctx, p.ID, intentID, workerName,
		strings.TrimSpace(*result.Fact), patchProfileFactType)
	if err != nil {
		releaseBootstrapIntent(ctx, c, p.ID, intentID, workerName)
		return fmt.Errorf("bootstrap: conclude reserved intent %s: %w", intentID, err)
	}
	if strings.TrimSpace(fact.ID) == "" {
		return fmt.Errorf("bootstrap: conclude returned no fact id")
	}

	// worker.Result has no exploit_idea field, so R7's exploit_idea fact is
	// intentionally not written here; only the patch_profile fact is created.
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
			return fmt.Errorf("bootstrap: declare inversion intent %d: %w", i, err)
		}
	}
	return nil
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
	b.WriteString(formatRules(rules))
	b.WriteString("\n")
	return b.String()
}

// releaseBootstrapIntent is best-effort: the original failure is what the
// caller needs to see.
func releaseBootstrapIntent(ctx context.Context, c graphserver.Client, projectID, intentID, workerName string) {
	_, _ = c.ReleaseIntent(ctx, projectID, intentID, workerName)
}
