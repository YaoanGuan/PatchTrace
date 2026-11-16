// Package worker defines the contract between the dispatcher and the
// executable agents ("workers") that turn a task prompt into a structured
// result. Drivers hide how a particular backend (native LLM, CLI, mock, ...)
// runs the task; the dispatcher only sees Task in and Result out.
package worker

import (
	"context"
	"time"
)

// TaskType is the closed set of task kinds understood by the graph lane.
type TaskType string

const (
	// TaskBootstrap ingests a patch and inverts it into graph facts/intents.
	TaskBootstrap TaskType = "bootstrap"
	// TaskReason reads the provenance graph and decides complete / new intents.
	TaskReason TaskType = "reason"
	// TaskExplore validates one claimed intent inside the sandbox.
	TaskExplore TaskType = "explore"
)

// Task is a single unit of work handed to a driver. Prompt is the fully
// rendered task instruction; when it is empty the native driver loads the
// per-type template from its prompt directory.
type Task struct {
	ProjectID string
	IntentID  string
	Type      string
	Prompt    string

	Timeout         time.Duration
	ConcludeTimeout time.Duration
}

// IntentSpec is one declared exploration direction produced by a model.
type IntentSpec struct {
	From        []string
	Description string
}

// Result is the contract-validated model output for one task.
//
//   - Fact:         bootstrap fact description (data.fact.description)
//   - Complete:     completion description (data.complete.description)
//   - CompleteFrom: fact ids the completion is derived from (data.complete.from)
//   - Intent:       singular intent description (data.intent.description)
//   - Description:  explore fact description (data.description)
//   - Intents:      declared intent specs (data.intents[])
//   - Raw:          the raw model output, preserved for audit
//
// Accepted is false for an explicit policy refusal (accepted:false); a
// refused result never carries writable fields.
type Result struct {
	Accepted     bool
	Fact         *string
	Complete     *string
	CompleteFrom []string
	Intent       *string
	Description  string
	Intents      []IntentSpec
	Raw          string
}

// Driver executes one task against one worker backend.
type Driver interface {
	Name() string
	Supports(TaskType) bool
	HealthCheck(ctx context.Context) error
	Execute(ctx context.Context, t Task) (Result, error)
	Conclude(ctx context.Context, t Task) (Result, error)
	SupportsConclude() bool
}
