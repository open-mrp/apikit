package ratelimit

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// Redis is a fixed-window Limiter shared by every replica through Redis: each key gets one counter per window, which expires with it.
type Redis struct {
	client *redis.Client
	prefix string
	limit  int
	window time.Duration
}

var _ Limiter = (*Redis)(nil)

// NewRedis allows limit requests per window for each key, counting in client under keys starting with prefix.
func NewRedis(client *redis.Client, prefix string, limit int, window time.Duration) *Redis {
	return &Redis{client: client, prefix: prefix, limit: limit, window: window}
}

// incrScript increments the window's counter and starts its expiry on the first request, returning the count and the milliseconds left in the window.
var incrScript = redis.NewScript(`
local count = redis.call("INCR", KEYS[1])
if count == 1 then
  redis.call("PEXPIRE", KEYS[1], ARGV[1])
end
return {count, redis.call("PTTL", KEYS[1])}
`)

// Allow records a request for key and reports whether it is within the limit.
func (r *Redis) Allow(ctx context.Context, key string) (Decision, error) {
	res, err := incrScript.Run(ctx, r.client, []string{r.prefix + key}, r.window.Milliseconds()).Int64Slice()
	if err != nil {
		return Decision{}, fmt.Errorf("ratelimit: redis: %w", err)
	}
	count, ttl := int(res[0]), time.Duration(res[1])*time.Millisecond
	if ttl < 0 {
		ttl = r.window
	}
	d := Decision{Limit: r.limit, Reset: ttl}
	if count <= r.limit {
		d.Allowed = true
		d.Remaining = r.limit - count
		return d, nil
	}
	d.RetryAfter = ttl
	return d, nil
}
