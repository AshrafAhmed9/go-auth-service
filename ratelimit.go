package main

// Per-IP rate limiting for /login. Redis is the source of truth because it
// lets every instance of this service share one counter; if Redis is
// unreachable we fall back to an in-process counter so the endpoint keeps
// working (degraded to per-instance limits) instead of failing every
// request. This is a deliberate contrast with the Java service's circuit
// breaker, which fails closed — rate limiting is a nice-to-have, auth isn't.
import (
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

type rateLimiter struct {
	redis  *RedisClient
	limit  int
	window time.Duration

	mu      sync.Mutex
	local   map[string]*localWindow
}

type localWindow struct {
	count int
	start time.Time
}

func newRateLimiter(redis *RedisClient, limit int, window time.Duration) *rateLimiter {
	return &rateLimiter{redis: redis, limit: limit, window: window, local: make(map[string]*localWindow)}
}

func (rl *rateLimiter) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		ip := c.ClientIP()

		allowed, err := rl.allowViaRedis(ip)
		if err != nil {
			allowed = rl.allowViaMemory(ip)
		}

		if !allowed {
			fail(c, http.StatusTooManyRequests, "too many requests, try again later")
			c.Abort()
			return
		}
		c.Next()
	}
}

func (rl *rateLimiter) allowViaRedis(ip string) (bool, error) {
	if rl.redis == nil {
		return false, fmt.Errorf("redis not configured")
	}
	windowKey := time.Now().Unix() / int64(rl.window.Seconds())
	count, err := rl.redis.Increment(fmt.Sprintf("ratelimit:%s:%d", ip, windowKey), rl.window)
	if err != nil {
		return false, err
	}
	return count <= int64(rl.limit), nil
}

func (rl *rateLimiter) allowViaMemory(ip string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	w, exists := rl.local[ip]
	if !exists || time.Since(w.start) > rl.window {
		rl.local[ip] = &localWindow{count: 1, start: time.Now()}
		return true
	}
	w.count++
	return w.count <= rl.limit
}
