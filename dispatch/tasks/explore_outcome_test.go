package tasks

import (
	"context"
	"errors"
	"testing"

	"AIxVuln/dispatch/worker"
)

// TestExploreOnVerdictHookRunsAfterVerdict proves the optional outcome hook is
// invoked exactly once, after the verdict fact is written, with the verdict's
// project and fact id.
func TestExploreOnVerdictHookRunsAfterVerdict(t *testing.T) {
	exploreWorkdir(t)
	client := newExploreClient()
	driver := &fakeExploreDriver{
		name:             "native-test",
		supportsConclude: true,
		execute: func(context.Context, worker.Task) (worker.Result, error) {
			return acceptedResult("证据成立",
				`{"accepted":true,"data":{"description":"证据成立","verdict":"support"}}`), nil
		},
	}

	type call struct{ projectID, factID string }
	var calls []call
	hook := func(_ context.Context, projectID, factID string) error {
		if len(client.verdicts) != 1 {
			t.Errorf("hook ran before the verdict was written: verdicts=%d", len(client.verdicts))
		}
		calls = append(calls, call{projectID: projectID, factID: factID})
		return nil
	}

	if err := RunExploreWithOptions(context.Background(), client, driver, &fakeSandboxRuntime{},
		exploreProject(), "i001", ExploreOptions{OnVerdict: hook}); err != nil {
		t.Fatalf("RunExploreWithOptions: %v", err)
	}
	if len(calls) != 1 {
		t.Fatalf("OnVerdict calls = %d, want 1", len(calls))
	}
	if calls[0].projectID != "p1" || calls[0].factID != "f200" {
		t.Fatalf("OnVerdict args = %+v, want project p1 and fact f200", calls[0])
	}
}

// TestExploreOnVerdictHookSkippedWithoutVerdict proves the hook only fires for a
// successfully written verdict; an accepted result without one leaves it alone.
func TestExploreOnVerdictHookSkippedWithoutVerdict(t *testing.T) {
	exploreWorkdir(t)
	client := newExploreClient()
	driver := &fakeExploreDriver{
		name:             "native-test",
		supportsConclude: true,
		execute: func(context.Context, worker.Task) (worker.Result, error) {
			return acceptedResult("没有明确判决", `{"accepted":true,"data":{"description":"没有明确判决"}}`), nil
		},
	}

	calls := 0
	if err := RunExploreWithOptions(context.Background(), client, driver, &fakeSandboxRuntime{},
		exploreProject(), "i001", ExploreOptions{
			OnVerdict: func(context.Context, string, string) error {
				calls++
				return nil
			},
		}); err != nil {
		t.Fatalf("RunExploreWithOptions: %v", err)
	}
	if calls != 0 {
		t.Fatalf("OnVerdict calls = %d, want 0 without a verdict", calls)
	}
}

// TestExploreOnVerdictHookSkippedOnVerdictFailure proves a failed verdict write
// neither runs the hook nor hides the write error.
func TestExploreOnVerdictHookSkippedOnVerdictFailure(t *testing.T) {
	exploreWorkdir(t)
	client := newExploreClient()
	client.verdictErr = errors.New("graph write failed")
	driver := &fakeExploreDriver{
		name:             "native-test",
		supportsConclude: true,
		execute: func(context.Context, worker.Task) (worker.Result, error) {
			return acceptedResult("证据成立",
				`{"accepted":true,"data":{"description":"证据成立","verdict":"support"}}`), nil
		},
	}

	calls := 0
	err := RunExploreWithOptions(context.Background(), client, driver, &fakeSandboxRuntime{},
		exploreProject(), "i001", ExploreOptions{
			OnVerdict: func(context.Context, string, string) error {
				calls++
				return nil
			},
		})
	if err == nil {
		t.Fatal("RunExploreWithOptions succeeded despite the failed verdict write")
	}
	if calls != 0 {
		t.Fatalf("OnVerdict calls = %d, want 0 when the verdict write failed", calls)
	}
}

// TestExploreOnVerdictHookErrorIsReturned proves a feedback failure surfaces
// after the verdict fact has been written instead of being silently dropped.
func TestExploreOnVerdictHookErrorIsReturned(t *testing.T) {
	exploreWorkdir(t)
	client := newExploreClient()
	driver := &fakeExploreDriver{
		name:             "native-test",
		supportsConclude: true,
		execute: func(context.Context, worker.Task) (worker.Result, error) {
			return acceptedResult("证据成立",
				`{"accepted":true,"data":{"description":"证据成立","verdict":"support"}}`), nil
		},
	}

	hookErr := errors.New("rule base unavailable")
	err := RunExploreWithOptions(context.Background(), client, driver, &fakeSandboxRuntime{},
		exploreProject(), "i001", ExploreOptions{
			OnVerdict: func(context.Context, string, string) error { return hookErr },
		})
	if !errors.Is(err, hookErr) {
		t.Fatalf("RunExploreWithOptions error = %v, want the hook error", err)
	}
	if len(client.verdicts) != 1 {
		t.Fatalf("verdicts = %d, want the fact written exactly once", len(client.verdicts))
	}
}
