package main

// A thin wrapper around go-redis with just the two things this service
// needs from Redis: a token blacklist (for logout) and a request counter
// (for rate limiting). Nothing here is Redis-specific business logic —
// that lives in ratelimit.go and auth.go.
import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

type RedisClient struct {
	client *redis.Client
}

func newRedisClient(addr string) *RedisClient {
	return &RedisClient{client: redis.NewClient(&redis.Options{Addr: addr})}
}

func (r *RedisClient) Blacklist(jti string, ttl time.Duration) error {
	return r.client.Set(context.Background(), "blacklist:"+jti, "1", ttl).Err()
}

func (r *RedisClient) IsBlacklisted(jti string) bool {
	val, err := r.client.Get(context.Background(), "blacklist:"+jti).Result()
	return err == nil && val == "1"
}

// Increment bumps the counter for key and sets its expiry the first time
// it's created, giving a fixed-window counter in two Redis calls.
func (r *RedisClient) Increment(key string, window time.Duration) (int64, error) {
	count, err := r.client.Incr(context.Background(), key).Result()
	if err != nil {
		return 0, err
	}
	if count == 1 {
		r.client.Expire(context.Background(), key, window)
	}
	return count, nil
}
