package middleware

import (
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/AshrafAhmed9/assignment-golang/cache"
	"github.com/gin-gonic/gin"
)

type entry struct {
	count       int
	windowStart time.Time
}

var (
	mu      sync.Mutex
	clients = make(map[string]*entry)
)

func init() {
	go func() {
		ticker := time.NewTicker(5 * time.Minute)
		for range ticker.C {
			mu.Lock()
			for ip, e := range clients {
				if time.Since(e.windowStart) > 5*time.Minute {
					delete(clients, ip)
				}
			}
			mu.Unlock()
		}
	}()
}

func ResetRateLimiter() {
	mu.Lock()
	clients = make(map[string]*entry)
	mu.Unlock()
}

func RateLimitMiddleware(rdb *cache.RedisClient, limit int, window time.Duration) gin.HandlerFunc {
	return func(c *gin.Context) {
		ip := c.ClientIP()

		if rdb != nil {
			windowKey := time.Now().Unix() / int64(window.Seconds())
			key := fmt.Sprintf("ratelimit:%s:%d", ip, windowKey)

			count, err := rdb.IncrementRateLimit(key, window)
			if err == nil {
				if count > int64(limit) {
					c.JSON(http.StatusTooManyRequests, gin.H{"error": "too many requests, try again later"})
					c.Abort()
					return
				}
				c.Next()
				return
			}
		}

		mu.Lock()
		e, exists := clients[ip]
		if !exists || time.Since(e.windowStart) > window {
			clients[ip] = &entry{count: 1, windowStart: time.Now()}
			mu.Unlock()
			c.Next()
			return
		}
		e.count++
		if e.count > limit {
			mu.Unlock()
			c.JSON(http.StatusTooManyRequests, gin.H{"error": "too many requests, try again later"})
			c.Abort()
			return
		}
		mu.Unlock()
		c.Next()
	}
}

func PerUserRateLimitMiddleware(rdb *cache.RedisClient, limit int, window time.Duration) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := c.GetUint("userID")
		if userID == 0 {
			c.Next()
			return
		}

		if rdb != nil {
			windowKey := time.Now().Unix() / int64(window.Seconds())
			key := fmt.Sprintf("ratelimit:user:%d:%d", userID, windowKey)

			count, err := rdb.IncrementRateLimit(key, window)
			if err == nil {
				if count > int64(limit) {
					c.JSON(http.StatusTooManyRequests, gin.H{"error": "too many requests, try again later"})
					c.Abort()
					return
				}
			}
		}

		c.Next()
	}
}
