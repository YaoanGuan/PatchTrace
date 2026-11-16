package dispatch

import (
	"sort"

	"AIxVuln/dispatch/worker"
)

// Optional worker attributes. Drivers are not required to implement either:
//
//   - Priority() int defaults to 0. Lower values are selected first.
//   - MaxRunning() int defaults to unlimited (0 or negative means unlimited).
//
// Both are read through interface assertions so dispatch/worker stays a thin
// contract and future drivers can opt in.
type workerWithPriority interface {
	Priority() int
}

type workerWithMaxRunning interface {
	MaxRunning() int
}

func driverPriority(d worker.Driver) int {
	if p, ok := d.(workerWithPriority); ok {
		return p.Priority()
	}
	return 0
}

func driverMaxRunning(d worker.Driver) int {
	if m, ok := d.(workerWithMaxRunning); ok {
		return m.MaxRunning()
	}
	return 0
}

type workerCandidate struct {
	driver   worker.Driver
	name     string
	priority int
	running  int
}

// selectWorker chooses the best healthy driver for tt.
//
// Order: task type support -> not in the unhealthy cooldown (both handled by
// Registry.Available) -> MaxRunning cap -> priority ascending -> running-count
// ascending -> injectable random tie-break.
func (d *Dispatcher) selectWorker(tt worker.TaskType) (worker.Driver, bool) {
	available := d.registry.Available(tt)
	candidates := make([]workerCandidate, 0, len(available))
	for _, driver := range available {
		name := driver.Name()
		running := d.running[name]
		if max := driverMaxRunning(driver); max > 0 && running >= max {
			continue
		}
		candidates = append(candidates, workerCandidate{
			driver:   driver,
			name:     name,
			priority: driverPriority(driver),
			running:  running,
		})
	}
	if len(candidates) == 0 {
		return nil, false
	}

	// Name is the final key so the candidate order is stable and an injected
	// random index is reproducible in tests.
	sort.SliceStable(candidates, func(i, j int) bool {
		if candidates[i].priority != candidates[j].priority {
			return candidates[i].priority < candidates[j].priority
		}
		if candidates[i].running != candidates[j].running {
			return candidates[i].running < candidates[j].running
		}
		return candidates[i].name < candidates[j].name
	})

	tied := 1
	for tied < len(candidates) &&
		candidates[tied].priority == candidates[0].priority &&
		candidates[tied].running == candidates[0].running {
		tied++
	}

	index := 0
	if tied > 1 && d.randIntn != nil {
		index = d.randIntn(tied)
		if index < 0 || index >= tied {
			index = 0
		}
	}
	return candidates[index].driver, true
}
