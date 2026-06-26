package cache

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

var ctx = context.Background()

type RedisClient struct {
	client *redis.Client
}

func NewRedisClient(addr string) *RedisClient {
	rdb := redis.NewClient(&redis.Options{
		Addr: addr,
	})
	return &RedisClient{client: rdb}
}

func (r *RedisClient) Blacklist(token string, ttl time.Duration) error {
	return r.client.Set(ctx, "blacklist:"+token, "1", ttl).Err()
}

func (r *RedisClient) IsBlacklisted(token string) bool {
	val, err := r.client.Get(ctx, "blacklist:"+token).Result()
	return err == nil && val == "1"
}

func (r *RedisClient) IncrementRateLimit(key string, window time.Duration) (int64, error) {
	count, err := r.client.Incr(ctx, key).Result()
	if err != nil {
		return 0, err
	}
	if count == 1 {
		r.client.Expire(ctx, key, window)
	}
	return count, nil
}

func (r *RedisClient) IncrementFailedAttempts(email string, window time.Duration) (int64, error) {
	key := "failcount:" + email
	count, err := r.client.Incr(ctx, key).Result()
	if err != nil {
		return 0, err
	}
	if count == 1 {
		r.client.Expire(ctx, key, window)
	}
	return count, nil
}

func (r *RedisClient) IsLocked(email string) bool {
	val, err := r.client.Get(ctx, "lock:"+email).Result()
	return err == nil && val == "1"
}

func (r *RedisClient) LockAccount(email string, duration time.Duration) error {
	return r.client.Set(ctx, "lock:"+email, "1", duration).Err()
}

func (r *RedisClient) ClearFailedAttempts(email string) error {
	return r.client.Del(ctx, "failcount:"+email).Err()
}
