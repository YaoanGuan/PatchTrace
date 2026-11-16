package worker

import (
	"context"
	"sync"
	"time"
)

// DefaultUnhealthyCooldown is the short window a worker is skipped after a
// failed health check. It matches the Cairn dispatcher retry window: the
// worker is probed again on a later scheduler tick once the window expires.
const DefaultUnhealthyCooldown = 5 * time.Second

// Registry owns the registered drivers and the local "retry after" window for
// workers whose last health check failed. It is safe for concurrent use.
type Registry struct {
	mu             sync.Mutex
	drivers        []Driver
	unhealthyUntil map[string]time.Time
	cooldown       time.Duration
	now            func() time.Time
}

// NewRegistry builds a registry from the supplied drivers. A non-positive
// cooldown falls back to DefaultUnhealthyCooldown.
func NewRegistry(cooldown time.Duration, drivers ...Driver) *Registry {
	if cooldown <= 0 {
		cooldown = DefaultUnhealthyCooldown
	}
	r := &Registry{
		unhealthyUntil: make(map[string]time.Time),
		cooldown:       cooldown,
		now:            time.Now,
	}
	r.drivers = append(r.drivers, drivers...)
	return r
}

// SetClock overrides the registry clock. It exists so tests can cross the
// unhealthy retry window without sleeping; production code never calls it.
func (r *Registry) SetClock(now func() time.Time) {
	if r == nil || now == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.now = now
}

// Register adds a driver to the registry.
func (r *Registry) Register(d Driver) {
	if d == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.drivers = append(r.drivers, d)
}

// Drivers returns a snapshot of all registered drivers.
func (r *Registry) Drivers() []Driver {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]Driver, len(r.drivers))
	copy(out, r.drivers)
	return out
}

// Find returns the first registered driver with the given name.
func (r *Registry) Find(name string) (Driver, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, d := range r.drivers {
		if d.Name() == name {
			return d, true
		}
	}
	return nil, false
}

// Available returns the drivers that support tt and are outside their
// unhealthy cooldown window.
func (r *Registry) Available(tt TaskType) []Driver {
	r.mu.Lock()
	defer r.mu.Unlock()
	now := r.now()
	out := make([]Driver, 0, len(r.drivers))
	for _, d := range r.drivers {
		if !d.Supports(tt) {
			continue
		}
		if until, ok := r.unhealthyUntil[d.Name()]; ok && now.Before(until) {
			continue
		}
		out = append(out, d)
	}
	return out
}

// MarkUnhealthy starts the cooldown window for the named worker.
func (r *Registry) MarkUnhealthy(name string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.unhealthyUntil[name] = r.now().Add(r.cooldown)
}

// MarkHealthy clears any cooldown window for the named worker.
func (r *Registry) MarkHealthy(name string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.unhealthyUntil, name)
}

// IsUnhealthy reports whether the named worker is still inside its cooldown.
func (r *Registry) IsUnhealthy(name string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	until, ok := r.unhealthyUntil[name]
	return ok && r.now().Before(until)
}

// RetryAfter returns the remaining cooldown for the named worker, or zero when
// it is healthy.
func (r *Registry) RetryAfter(name string) time.Duration {
	r.mu.Lock()
	defer r.mu.Unlock()
	until, ok := r.unhealthyUntil[name]
	if !ok {
		return 0
	}
	remaining := until.Sub(r.now())
	if remaining < 0 {
		return 0
	}
	return remaining
}

// Check probes a driver and updates its health window: failure enters the
// cooldown, success clears it.
func (r *Registry) Check(ctx context.Context, d Driver) error {
	if err := d.HealthCheck(ctx); err != nil {
		r.MarkUnhealthy(d.Name())
		return err
	}
	r.MarkHealthy(d.Name())
	return nil
}
