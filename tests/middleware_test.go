package tests

import (
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/AshrafAhmed9/assignment-golang/middleware"
	"github.com/AshrafAhmed9/assignment-golang/utils"
	"github.com/gin-gonic/gin"
)

func setupAuthRouter(secret string) *gin.Engine {
	os.Setenv("JWT_SECRET", secret)
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(middleware.RequestID())
	r.Use(middleware.JWTAuthMiddleware())
	r.GET("/protected", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"role": c.GetString("role")})
	})
	return r
}

func TestJWTMiddleware_NoHeader(t *testing.T) {
	r := setupAuthRouter("test-secret-key-that-is-32-chars!!")
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/protected", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

func TestJWTMiddleware_InvalidToken(t *testing.T) {
	r := setupAuthRouter("test-secret-key-that-is-32-chars!!")
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/protected", nil)
	req.Header.Set("Authorization", "Bearer invalidtoken")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

func TestJWTMiddleware_ExpiredToken(t *testing.T) {
	secret := "test-secret-key-that-is-32-chars!!"
	token, _ := utils.GenerateToken(1, "alice@test.com", "user", secret, -1*time.Second)

	r := setupAuthRouter(secret)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 for expired token, got %d", w.Code)
	}
}

func TestJWTMiddleware_Valid(t *testing.T) {
	secret := "test-secret-key-that-is-32-chars!!"
	token, _ := utils.GenerateToken(1, "alice@test.com", "user", secret, 24*time.Hour)

	r := setupAuthRouter(secret)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestAdminMiddleware_UserRole(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(middleware.RequestID())
	r.Use(func(c *gin.Context) {
		c.Set("role", "user")
		c.Next()
	})
	r.GET("/admin", middleware.AdminOnlyMiddleware(), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/admin", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", w.Code)
	}
}

func TestAdminMiddleware_AdminRole(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(middleware.RequestID())
	r.Use(func(c *gin.Context) {
		c.Set("role", "admin")
		c.Next()
	})
	r.GET("/admin", middleware.AdminOnlyMiddleware(), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/admin", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}
