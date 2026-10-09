package ratelimit

import (
	"context"
	"sync"
	"time"

	"github.com/open-mrp/apikit/retry"
)

// Memory is an in-process sliding-window Limiter. Each process counts on its own, so behind several replicas the effective limit is the limit times the replica count; use Redis to share one count.
//
// A key that keeps exceeding the limit is told to wait exponentially longer, so a client that ignores Retry-After backs off instead of hammering the server.
type Memory struct {
	mu         sync.Mutex
	requests   map[string][]time.Time
	violations map[string]int
	limit      int
	window     time.Duration
	backoff    retry.Config
	now        func() time.Time
}

var _ Limiter = (*Memory)(nil)

// NewMemory allows limit requests per window for each key.
func NewMemory(limit int, window time.Duration) *Memory {
	return &Memory{
		requests:   map[string][]time.Time{},
		violations: map[string]int{},
		limit:      limit,
		window:     window,
		backoff: retry.Config{
			InitialWait:    time.Second,
			MaxWait:        15 * time.Minute,
			Multiplier:     1.5,
			JitterFraction: 0.1,
		},
		now: func() time.Time { return time.Now().UTC() },
	}
}

// Allow records a request for key and reports whether it is within the limit.
func (m *Memory) Allow(_ context.Context, key string) (Decision, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := m.now()
	valid := m.prune(key, now)
	d := Decision{Limit: m.limit}
	if len(valid) < m.limit {
		valid = append(valid, now)
		m.requests[key] = valid
		m.violations[key] = 0
		d.Allowed = true
		d.Remaining = m.limit - len(valid)
		d.Reset = valid[0].Add(m.window).Sub(now)
		return d, nil
	}
	m.violations[key]++
	d.Reset = valid[0].Add(m.window).Sub(now)
	d.RetryAfter = retry.CalculateDelay(&m.backoff, m.violations[key])
	return d, nil
}

// Check reports whether key is within the limit without recording a request, and how long to wait if not. Pair it with RecordFailure to throttle only some outcomes, such as failed sign-ins.
func (m *Memory) Check(key string) (allowed bool, retryAfter time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()

	valid := m.prune(key, m.now())
	if len(valid) < m.limit {
		return true, 0
	}
	violations := m.violations[key]
	if violations <= 0 {
		violations = len(valid) - m.limit + 1
	}
	return false, retry.CalculateDelay(&m.backoff, violations)
}

// RecordFailure records one attempt for key, advancing its window and, past the limit, its backoff.
func (m *Memory) RecordFailure(key string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := m.now()
	valid := append(m.prune(key, now), now)
	m.requests[key] = valid
	if len(valid) > m.limit {
		m.violations[key]++
	}
}

// prune drops key's requests older than the window and returns the rest, oldest first.
func (m *Memory) prune(key string, now time.Time) []time.Time {
	cutoff := now.Add(-m.window)
	reqs := m.requests[key]
	i := 0
	for i < len(reqs) && !reqs[i].After(cutoff) {
		i++
	}
	reqs = reqs[i:]
	if len(reqs) == 0 {
		delete(m.requests, key)
		return nil
	}
	m.requests[key] = reqs
	return reqs
}
