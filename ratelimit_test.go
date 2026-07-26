package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func rateLimitedRouter(limit int, window time.Duration) *gin.Engine {
	gin.SetMode(gin.TestMode)
	limiter := newRateLimiter(nil, limit, window) // nil redis: exercises the in-memory fallback path
	r := gin.New()
	r.POST("/login", limiter.Middleware(), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})
	return r
}

func requestFrom(r *gin.Engine, ip string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/login", nil)
	req.RemoteAddr = ip + ":12345"
	r.ServeHTTP(w, req)
	return w
}

func TestRateLimit_BlocksAfterLimit(t *testing.T) {
	r := rateLimitedRouter(3, time.Minute)

	var codes []int
	for i := 0; i < 4; i++ {
		codes = append(codes, requestFrom(r, "192.168.1.1").Code)
	}

	for i := 0; i < 3; i++ {
		if codes[i] != http.StatusOK {
			t.Errorf("request %d: expected 200, got %d", i+1, codes[i])
		}
	}
	if codes[3] != http.StatusTooManyRequests {
		t.Errorf("request 4: expected 429, got %d", codes[3])
	}
}

func TestRateLimit_DifferentIPsAreIndependent(t *testing.T) {
	r := rateLimitedRouter(2, time.Minute)

	requestFrom(r, "10.0.0.1")
	requestFrom(r, "10.0.0.1")
	requestFrom(r, "10.0.0.1") // this IP is now over its limit

	w := requestFrom(r, "10.0.0.2")
	if w.Code != http.StatusOK {
		t.Errorf("a different IP should be unaffected, got %d", w.Code)
	}
}

func TestRateLimit_FallsBackToMemoryWithoutRedis(t *testing.T) {
	// newRateLimiter was given a nil redis client above, so every request
	// in this file already exercises the fallback — this test just makes
	// the intent explicit: no panic, and the limit is still enforced.
	r := rateLimitedRouter(1, time.Minute)

	first := requestFrom(r, "172.16.0.1")
	second := requestFrom(r, "172.16.0.1")

	if first.Code != http.StatusOK {
		t.Errorf("first request: expected 200, got %d", first.Code)
	}
	if second.Code != http.StatusTooManyRequests {
		t.Errorf("second request: expected 429, got %d", second.Code)
	}
}
