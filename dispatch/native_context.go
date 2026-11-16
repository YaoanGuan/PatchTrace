package dispatch

import (
	"context"
	"strings"
	"time"

	"AIxVuln/dispatch/worker"
	"AIxVuln/graphserver"

	yaml "github.com/goccy/go-yaml"
)

// nativeContextTimeout bounds the graph export read used to render a driver's
// conclude prompt. Conclude runs on a fresh detached context after the execute
// deadline, so it must not depend on the caller's (possibly expired) context.
const nativeContextTimeout = 5 * time.Second

// NativeContext builds worker.NativeConfig.Context from the project graph.
// The *_conclude templates need graph_yaml, intent_id and intent_description;
// the callback resolves them through ExportYAML so a driver can render the
// conclude contract without the runner having to carry a second prompt.
//
// Every returned value is placeholder-free: an unknown intent keeps the
// requested id and an empty description, and an export failure yields empty
// graph context. An empty id is replaced with "unknown" so {intent_id} can
// never survive rendering and reach the model as broken text.
func NativeContext(c graphserver.Client) func(worker.Task) map[string]string {
	return func(t worker.Task) map[string]string {
		intentID := strings.TrimSpace(t.IntentID)
		if intentID == "" {
			intentID = "unknown"
		}
		values := map[string]string{
			"graph_yaml":         "",
			"fact_ids":           "[]",
			"open_intents":       "[]",
			"intent_id":          intentID,
			"intent_description": "",
		}
		if c == nil {
			return values
		}
		ctx, cancel := context.WithTimeout(context.Background(), nativeContextTimeout)
		defer cancel()
		export, err := c.ExportYAML(ctx, t.ProjectID)
		if err != nil {
			return values
		}
		values["graph_yaml"] = strings.TrimSpace(export)
		if description, ok := nativeExportIntentDescription(export, intentID); ok {
			values["intent_description"] = description
		}
		return values
	}
}

type nativeContextExport struct {
	Intents []struct {
		ID      string `yaml:"id"`
		Content string `yaml:"content"`
	} `yaml:"intents"`
}

func nativeExportIntentDescription(export, intentID string) (string, bool) {
	var doc nativeContextExport
	if strings.TrimSpace(export) == "" {
		return "", false
	}
	if err := yaml.Unmarshal([]byte(export), &doc); err != nil {
		return "", false
	}
	for _, intent := range doc.Intents {
		if strings.TrimSpace(intent.ID) == intentID {
			return strings.TrimSpace(intent.Content), true
		}
	}
	return "", false
}
