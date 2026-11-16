package graph

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	yaml "github.com/goccy/go-yaml"
)

// protocolTimeLayout is the human-readable timestamp format used by both
// export formats.
const protocolTimeLayout = "2006-01-02 15:04:05"

func protocolTime(t time.Time) string {
	return t.UTC().Format(protocolTimeLayout)
}

func protocolTimeValue(t *time.Time) string {
	if t == nil {
		return ""
	}
	return protocolTime(*t)
}

// exportSnapshot is the four-section YAML protocol shape. Field order is
// load-bearing: project, hints, facts, intents.
type exportSnapshot struct {
	Project exportProject  `yaml:"project"`
	Hints   []exportHint   `yaml:"hints"`
	Facts   []exportFact   `yaml:"facts"`
	Intents []exportIntent `yaml:"intents"`
}

type exportProject struct {
	Title            string `yaml:"title"`
	Origin           string `yaml:"origin"`
	Goal             string `yaml:"goal"`
	BootstrapEnabled bool   `yaml:"bootstrap_enabled"`
}

type exportHint struct {
	ID        string `yaml:"id"`
	Content   string `yaml:"content"`
	Creator   string `yaml:"creator"`
	CreatedAt string `yaml:"created_at"`
}

type exportFact struct {
	ID        string `yaml:"id"`
	Type      string `yaml:"type"`
	Content   string `yaml:"content"`
	CreatedAt string `yaml:"created_at"`
}

// exportIntent deliberately omits lease internals (lease_until,
// last_heartbeat_at) while keeping the current worker.
type exportIntent struct {
	ID          string   `yaml:"id"`
	Type        string   `yaml:"type"`
	Content     string   `yaml:"content"`
	From        []string `yaml:"from"`
	To          *string  `yaml:"to"`
	State       string   `yaml:"state"`
	Worker      string   `yaml:"worker"`
	Creator     string   `yaml:"creator"`
	CreatedAt   string   `yaml:"created_at"`
	ConcludedAt *string  `yaml:"concluded_at"`
}

// ExportYAML renders the project graph as the protocol YAML snapshot used to
// build prompts. It includes every fact type, retains open intents with a null
// `to`, and never exposes the reason lease.
func (s *Store) ExportYAML(ctx context.Context, projectID string) (string, error) {
	projectID = strings.TrimSpace(projectID)
	if projectID == "" {
		return "", fmt.Errorf("%w: project is required", ErrInvalid)
	}
	p, err := s.GetProject(ctx, projectID)
	if err != nil {
		return "", err
	}
	nodes, err := s.Nodes(ctx, projectID)
	if err != nil {
		return "", err
	}

	snapshot := exportSnapshot{
		Project: exportProject{
			Title:            p.Title,
			Origin:           p.Origin,
			Goal:             p.Goal,
			BootstrapEnabled: p.BootstrapEnabled,
		},
		Hints:   make([]exportHint, 0),
		Facts:   make([]exportFact, 0),
		Intents: make([]exportIntent, 0),
	}
	for _, n := range nodes {
		switch n.Kind {
		case "hint":
			snapshot.Hints = append(snapshot.Hints, exportHint{
				ID:        n.ID,
				Content:   n.Content,
				Creator:   n.Creator,
				CreatedAt: protocolTimeValue(n.CreatedAt),
			})
		case "fact":
			snapshot.Facts = append(snapshot.Facts, exportFact{
				ID:        n.ID,
				Type:      n.Type,
				Content:   n.Content,
				CreatedAt: protocolTimeValue(n.CreatedAt),
			})
		case "intent":
			from := n.From
			if from == nil {
				from = []string{}
			}
			var to *string
			if n.To != nil {
				v := *n.To
				to = &v
			}
			var concludedAt *string
			if n.ConcludedAt != nil {
				v := protocolTime(*n.ConcludedAt)
				concludedAt = &v
			}
			snapshot.Intents = append(snapshot.Intents, exportIntent{
				ID:          n.ID,
				Type:        n.Type,
				Content:     n.Content,
				From:        from,
				To:          to,
				State:       n.State,
				Worker:      n.Worker,
				Creator:     n.Creator,
				CreatedAt:   protocolTimeValue(n.CreatedAt),
				ConcludedAt: concludedAt,
			})
		}
	}

	out, err := yaml.Marshal(snapshot)
	if err != nil {
		return "", fmt.Errorf("graph: encode yaml export: %w", err)
	}
	return string(out), nil
}

type timelineEvent struct {
	at       time.Time
	priority int
	id       string
	line     string
}

// ExportTimeline renders the project history as chronological plain text. The
// event order is stable within a timestamp (created before concluded before
// completed), and a completion removed by Reopen no longer appears.
func (s *Store) ExportTimeline(ctx context.Context, projectID string) (string, error) {
	projectID = strings.TrimSpace(projectID)
	if projectID == "" {
		return "", fmt.Errorf("%w: project is required", ErrInvalid)
	}
	p, err := s.GetProject(ctx, projectID)
	if err != nil {
		return "", err
	}
	nodes, err := s.Nodes(ctx, projectID)
	if err != nil {
		return "", err
	}

	events := make([]timelineEvent, 0, len(nodes)+2)
	add := func(at *time.Time, priority int, id, event, content string) {
		if at == nil {
			return
		}
		events = append(events, timelineEvent{
			at:       *at,
			priority: priority,
			id:       id,
			line:     fmt.Sprintf("%s %s %s: %s", protocolTime(*at), event, id, content),
		})
	}

	created := p.CreatedAt
	add(&created, 0, p.ID, "PROJECT CREATED", p.Title)

	var completion *Node
	for i := range nodes {
		n := &nodes[i]
		switch n.Kind {
		case "hint":
			add(n.CreatedAt, 1, n.ID, "HINT", n.Content)
		case "intent":
			add(n.CreatedAt, 2, n.ID, "INTENT DECLARED", n.Content)
			if n.ConcludedAt != nil {
				conclusion := n.Content
				if n.To != nil {
					conclusion += " -> " + *n.To
				}
				add(n.ConcludedAt, 3, n.ID, "INTENT CONCLUDED", conclusion)
			}
			if n.To != nil && *n.To == "goal" {
				completion = n
			}
		}
	}
	if completion != nil {
		add(completion.ConcludedAt, 4, p.ID, "PROJECT COMPLETED", completion.Content)
	}

	sort.SliceStable(events, func(i, j int) bool {
		if !events[i].at.Equal(events[j].at) {
			return events[i].at.Before(events[j].at)
		}
		if events[i].priority != events[j].priority {
			return events[i].priority < events[j].priority
		}
		return events[i].id < events[j].id
	})

	var b strings.Builder
	for _, e := range events {
		b.WriteString(e.line)
		b.WriteByte('\n')
	}
	return b.String(), nil
}
