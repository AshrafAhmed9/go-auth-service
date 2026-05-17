package middleware

import (
	"net/http"
	"sync"
	"time"

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
				if time.Since(e.windowStart) > time.Minute {
					delete(clients, ip)
				}
			}
			mu.Unlock()
		}
	}()
}

func RateLimitMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		ip := c.ClientIP()

		mu.Lock()
		e, exists := clients[ip]
		if !exists || time.Since(e.windowStart) > time.Minute {
			clients[ip] = &entry{count: 1, windowStart: time.Now()}
			mu.Unlock()
			c.Next()
			return
		}
		e.count++
		if e.count > 5 {
			mu.Unlock()
			c.JSON(http.StatusTooManyRequests, gin.H{"error": "too many requests, try again later"})
			c.Abort()
			return
		}
		mu.Unlock()
		c.Next()
	}
}
