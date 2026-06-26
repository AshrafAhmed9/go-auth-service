package tests

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/AshrafAhmed9/assignment-golang/middleware"
	"github.com/gin-gonic/gin"
)

func setupRateLimitRouter(limit int, window time.Duration) *gin.Engine {
	middleware.ResetRateLimiter()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/login", middleware.RateLimitMiddleware(nil, limit, window), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})
	return r
}

func rateLimitRequest(r *gin.Engine, ip string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/login", nil)
	req.RemoteAddr = ip + ":12345"
	r.ServeHTTP(w, req)
	return w
}

func TestRateLimit_LoginExceeded(t *testing.T) {
	r := setupRateLimitRouter(5, time.Minute)

	var lastCode int
	for i := 0; i < 6; i++ {
		w := rateLimitRequest(r, "192.168.1.200")
		lastCode = w.Code
	}

	if lastCode != http.StatusTooManyRequests {
		t.Errorf("expected 429 after exceeding rate limit, got %d", lastCode)
	}
}

func TestRateLimit_DifferentIPsIndependent(t *testing.T) {
	r := setupRateLimitRouter(2, time.Minute)

	for i := 0; i < 3; i++ {
		rateLimitRequest(r, "10.0.0.1")
	}

	w := rateLimitRequest(r, "10.0.0.2")

	if w.Code != http.StatusOK {
		t.Errorf("expected 200 for different IP, got %d", w.Code)
	}
}

func TestRateLimit_FallbackWithNilRedis(t *testing.T) {
	r := setupRateLimitRouter(3, time.Minute)

	var codes []int
	for i := 0; i < 4; i++ {
		w := rateLimitRequest(r, "192.168.99.99")
		codes = append(codes, w.Code)
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
