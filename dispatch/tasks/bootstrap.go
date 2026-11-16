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
//	reserved intent -> claim -> prompt -> driver execute -> inversion ->
//	patch_profile fact -> inversion intents -> exploit_idea fact
//
// The runner is resumable. If the reserved bootstrap intent is already
// concluded, it takes the patch_profile fact id from the intent and completes
// only the missing work: it re-runs the driver (same prompt), declares only
// intents whose content is absent from the export, and writes the exploit_idea
// fact unless a concluded or claimed reserved idea intent already exists.
//
// A disabled project is a no-op. A project with no usable patch source fails
// before any graph write. Any failure before the reserved intent concludes
// releases it (joining release errors) so a later tick can retry; a claimed
// bootstrap intent is left untouched for its owner.
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
	state, err := inspectBootstrapGraph(export)
	if err != nil {
		return err
	}
	// A claimed bootstrap is owned by another worker; it owns the project
	// until it concludes. A claimed idea intent is owned elsewhere but does
	// not block the rest of the work.
	if state.BootstrapClaimed {
		return nil
	}

	workerName := strings.TrimSpace(d.Name())
	if workerName == "" {
		workerName = bootstrapWorkerName
	}

	bootstrapIntentID := state.BootstrapID
	patchFactID := state.PatchProfileFactID
	if state.BootstrapConcluded {
		// Resume: the patch profile already exists. Its fact id is the parent
		// of every other artifact, so a missing id is a hard error.
		if strings.TrimSpace(patchFactID) == "" {
			return errors.New("bootstrap: concluded reserved intent has no patch_profile fact id")
		}
	} else {
		if bootstrapIntentID == "" {
			intent, err := c.DeclareIntent(ctx, p.ID, graph.Node{
				Content: reservedBootstrapContent,
				From:    []string{reservedOriginFact},
				Creator: reservedBootstrapCreator,
			})
			if err != nil {
				return fmt.Errorf("bootstrap: create reserved intent: %w", err)
			}
			bootstrapIntentID = intent.ID
		}
		if _, err := c.ClaimIntent(ctx, p.ID, bootstrapIntentID, workerName, bootstrapIntentLease); err != nil {
			return fmt.Errorf("bootstrap: claim reserved intent %s: %w", bootstrapIntentID, err)
		}
	}

	// fail releases the claim only while this call owns it. A resume must not
	// touch the already resolved bootstrap intent.
	fail := func(cause error) error {
		if state.BootstrapConcluded {
			return cause
		}
		return bootstrapFailure(ctx, c, p.ID, bootstrapIntentID, workerName, cause)
	}

	contract, err := deps.loadPrompt(bootstrapPromptDir, worker.TaskBootstrap, false)
	if err != nil {
		return fail(fmt.Errorf("bootstrap: load prompt: %w", err))
	}
	scene, _ := deps.loadScene(profile.Scene)
	template := strings.TrimRight(contract, "\n") + "\n\n## 场景模板\n\n" + scene + "\n"
	rules := deps.searchRules(bootstrapRuleQuery(profile))
	prompt := bootstrapPrompt(p, profile, rules, template)

	// The worker may run for minutes; keep the claim alive while it does. On a
	// resume there is no claim to refresh. The deferred stop makes a panic in
	// Execute unable to leak the heartbeat goroutine.
	stopHeartbeat := func() {}
	if !state.BootstrapConcluded {
		stopHeartbeat = startBootstrapHeartbeat(ctx, c, p.ID, bootstrapIntentID, workerName, deps.heartbeatInterval)
	}
	defer stopHeartbeat()

	result, execErr := d.Execute(ctx, worker.Task{
		ProjectID: p.ID,
		IntentID:  bootstrapIntentID,
		Type:      string(worker.TaskBootstrap),
		Prompt:    prompt,
	})
	stopHeartbeat()
	if execErr != nil {
		return fail(fmt.Errorf("bootstrap: execute task: %w", execErr))
	}
	if !result.Accepted {
		return fail(errors.New("bootstrap: worker refused the task"))
	}
	if !state.BootstrapConcluded && (result.Fact == nil || strings.TrimSpace(*result.Fact) == "") {
		return fail(errors.New("bootstrap: worker returned no fact"))
	}

	needIdea := !state.IdeaConcluded && !state.IdeaClaimed

	// Semantic inversion produces the exploit hypothesis R7 requires. On a
	// first run it runs before any conclusion so a failed inversion releases
	// the claim and writes no facts; on a resume the patch_profile fact already
	// exists, so a failed inversion simply returns and retries later.
	var ideaDescription string
	if needIdea {
		inversion, err := deps.invert(ctx, profile, rules, profile.Scene)
		switch {
		case err != nil:
			return fail(fmt.Errorf("bootstrap: inversion failed: %w", err))
		case inversion == nil || inversion.Idea == nil:
			return fail(errors.New("bootstrap: inversion returned no exploit idea"))
		}
		ideaDescription, err = formatExploitIdea(inversion.Idea)
		if err != nil {
			return fail(err)
		}
	}

	if !state.BootstrapConcluded {
		fact, _, err := c.ConcludeIntent(ctx, p.ID, bootstrapIntentID, workerName,
			strings.TrimSpace(*result.Fact), patchProfileFactType)
		if err != nil {
			return fail(fmt.Errorf("bootstrap: conclude reserved intent %s: %w", bootstrapIntentID, err))
		}
		if strings.TrimSpace(fact.ID) == "" {
			// The intent is already resolved; without a fact id every dependent
			// write would dangle, so surface the inconsistency instead.
			return errors.New("bootstrap: conclude returned no fact id")
		}
		patchFactID = fact.ID
	}

	// Inversion intents go first: an idea failure must not skip them, and each
	// declaration failure must not abandon the rest. The export content is the
	// dedupe key, so a resumed bootstrap never re-declares an existing intent.
	var errs []error
	seen := state.IntentContents
	for i, spec := range result.Intents {
		description := strings.TrimSpace(spec.Description)
		if description == "" {
			continue
		}
		if _, exists := seen[description]; exists {
			continue
		}
		if _, err := c.DeclareIntent(ctx, p.ID, graph.Node{
			Content: description,
			From:    []string{patchFactID},
			Creator: workerName,
		}); err != nil {
			errs = append(errs, fmt.Errorf("bootstrap: declare inversion intent %d: %w", i, err))
			continue
		}
		seen[description] = struct{}{}
	}

	// R7 requires both facts. The exploit idea is written through the same
	// intent protocol as everything else in the graph.
	if needIdea {
		if err := ensureExploitIdea(ctx, c, p.ID, patchFactID, workerName, ideaDescription, state); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

// ensureExploitIdea writes the exploit_idea fact through a reserved intent. It
// reuses an existing reserved intent by state: open → claim → conclude;
// claimed → owned elsewhere; concluded → already done. It never declares a
// second reserved idea intent. The intent is released on failure and a
// successful conclusion is never released.
func ensureExploitIdea(ctx context.Context, c graphserver.Client, projectID, patchFactID, workerName, description string, state bootstrapGraphState) error {
	if state.IdeaConcluded || state.IdeaClaimed {
		return nil
	}
	intentID := state.IdeaOpenID
	if intentID == "" {
		intent, err := c.DeclareIntent(ctx, projectID, graph.Node{
			Content: exploitIdeaContent,
			From:    []string{patchFactID},
			Creator: reservedBootstrapCreator,
		})
		if err != nil {
			return fmt.Errorf("bootstrap: declare exploit_idea intent: %w", err)
		}
		intentID = intent.ID
	}
	if _, err := c.ClaimIntent(ctx, projectID, intentID, workerName, bootstrapIntentLease); err != nil {
		return fmt.Errorf("bootstrap: claim exploit_idea intent %s: %w", intentID, err)
	}
	fact, _, err := c.ConcludeIntent(ctx, projectID, intentID, workerName,
		description, exploitIdeaFactType)
	if err != nil {
		return bootstrapFailure(ctx, c, projectID, intentID, workerName,
			fmt.Errorf("bootstrap: conclude exploit_idea intent %s: %w", intentID, err))
	}
	if strings.TrimSpace(fact.ID) == "" {
		return errors.New("bootstrap: exploit_idea conclude returned no fact id")
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
