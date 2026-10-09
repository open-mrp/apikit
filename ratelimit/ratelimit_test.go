package ratelimit

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func TestMemory_AllowsUpToTheLimitThenBacksOff(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	m := NewMemory(2, time.Minute)
	m.now = func() time.Time { return now }

	for i, wantRemaining := range []int{1, 0} {
		d, _ := m.Allow(context.Background(), "ip")
		if !d.Allowed || d.Remaining != wantRemaining || d.Limit != 2 {
			t.Fatalf("request %d: %+v", i, d)
		}
	}
	first, _ := m.Allow(context.Background(), "ip")
	second, _ := m.Allow(context.Background(), "ip")
	if first.Allowed || second.Allowed || first.RetryAfter <= 0 {
		t.Fatalf("over the limit: %+v %+v", first, second)
	}
	if second.RetryAfter <= first.RetryAfter/2 {
		t.Errorf("backoff did not grow: %v then %v", first.RetryAfter, second.RetryAfter)
	}
	if d, _ := m.Allow(context.Background(), "other"); !d.Allowed {
		t.Error("keys are not independent")
	}

	now = now.Add(time.Minute + time.Second)
	if d, _ := m.Allow(context.Background(), "ip"); !d.Allowed {
		t.Errorf("window did not slide: %+v", d)
	}
}

func TestMemory_CheckAndRecordFailure(t *testing.T) {
	t.Parallel()
	m := NewMemory(2, time.Minute)
	for range 2 {
		if ok, _ := m.Check("user"); !ok {
			t.Fatal("refused before the limit")
		}
		m.RecordFailure("user")
	}
	if ok, wait := m.Check("user"); ok || wait <= 0 {
		t.Errorf("after the limit: ok=%v wait=%v", ok, wait)
	}
}

func TestRedis_FixedWindow(t *testing.T) {
	t.Parallel()
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	r := NewRedis(client, "rl:", 2, time.Minute)
	ctx := context.Background()

	for i, wantRemaining := range []int{1, 0} {
		d, err := r.Allow(ctx, "ip")
		if err != nil || !d.Allowed || d.Remaining != wantRemaining {
			t.Fatalf("request %d: %+v %v", i, d, err)
		}
	}
	d, err := r.Allow(ctx, "ip")
	if err != nil || d.Allowed || d.RetryAfter <= 0 || d.RetryAfter > time.Minute {
		t.Fatalf("over the limit: %+v %v", d, err)
	}

	mr.FastForward(time.Minute + time.Second)
	if d, _ := r.Allow(ctx, "ip"); !d.Allowed {
		t.Errorf("window did not reset: %+v", d)
	}
}
