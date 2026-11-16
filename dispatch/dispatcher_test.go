package dispatch

import (
	"context"
	"testing"

	"AIxVuln/dispatch/worker"
	"AIxVuln/graph"
)

func TestTickDoesNotAdvanceReasonBaselineAtPlanTime(t *testing.T) {
	facts := append(originGoalFacts(), factFixture{"f001", "patch_profile"})
	client := &fakeClient{
		projects: []graph.Project{activeProject("p1", false)},
		exports:  map[string]string{"p1": projectExport(facts, 0, nil)},
	}
	d := testDispatcher(t, client, newFakeDriver("agent", worker.TaskReason))

	// Without an executor nothing actually runs, so the baseline must never
	// advance and every tick keeps planning the same reason work.
	for i := 0; i < 2; i++ {
		decisions, err := d.tick(context.Background())
		if err != nil {
			t.Fatalf("tick %d: %v", i, err)
		}
		if len(decisions) != 1 || decisions[0].TaskType != worker.TaskReason {
			t.Fatalf("tick %d decisions = %+v, want one reason decision", i, decisions)
		}
	}
	if _, ok := d.lastReason["p1"]; ok {
		t.Fatal("tick must not advance the reason baseline at plan time")
	}
}
