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
