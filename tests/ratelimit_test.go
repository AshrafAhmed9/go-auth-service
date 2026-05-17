package tests

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/AshrafAhmed9/assignment-golang/middleware"
	"github.com/gin-gonic/gin"
)

func TestRateLimit_LoginExceeded(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/login", middleware.RateLimitMiddleware(), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	var lastCode int
	for i := 0; i < 6; i++ {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", "/login", nil)
		req.Header.Set("X-Forwarded-For", "192.168.1.100")
		r.ServeHTTP(w, req)
		lastCode = w.Code
	}

	if lastCode != http.StatusTooManyRequests {
		t.Errorf("expected 429 after exceeding rate limit, got %d", lastCode)
	}
}
