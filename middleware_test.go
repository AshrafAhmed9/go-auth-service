package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func protectedRouter() *gin.Engine {
	return protectedRouterWithCache(nil)
}

func protectedRouterWithCache(cache *RedisClient) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/protected", requireAuth(testConfig(), cache), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})
	return r
}

func withBearer(req *http.Request, token string) {
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
}

func TestRequireAuth_NoHeader(t *testing.T) {
	r := protectedRouter()
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/protected", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

func TestRequireAuth_InvalidToken(t *testing.T) {
	r := protectedRouter()
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/protected", nil)
	withBearer(req, "not-a-real-token")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

func TestRequireAuth_ExpiredToken(t *testing.T) {
	token, _ := generateToken(1, "alice@test.com", "user", testSecret, -time.Second)

	r := protectedRouter()
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/protected", nil)
	withBearer(req, token)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 for expired token, got %d", w.Code)
	}
}

func TestRequireAuth_ValidToken(t *testing.T) {
	token, _ := generateToken(1, "alice@test.com", "user", testSecret, time.Hour)

	r := protectedRouter()
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/protected", nil)
	withBearer(req, token)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestRequireAuth_BlacklistedToken(t *testing.T) {
	cache := testRedisClient(t)
	token, _ := generateToken(1, "alice@test.com", "user", testSecret, time.Hour)
	claims, err := parseToken(token, testSecret)
	if err != nil {
		t.Fatalf("failed to parse the token we just generated: %v", err)
	}
	if err := cache.Blacklist(claims.ID, time.Minute); err != nil {
		t.Fatalf("failed to blacklist: %v", err)
	}

	r := protectedRouterWithCache(cache)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/protected", nil)
	withBearer(req, token)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 for a blacklisted token, got %d", w.Code)
	}
}
