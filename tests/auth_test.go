package tests

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/AshrafAhmed9/assignment-golang/config"
	"github.com/AshrafAhmed9/assignment-golang/handlers"
	"github.com/AshrafAhmed9/assignment-golang/models"
	"github.com/glebarez/sqlite"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func setupTestDB() *gorm.DB {
	db, _ := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	db.AutoMigrate(&models.User{})
	return db
}

func setupTestConfig() *config.Config {
	return &config.Config{
		JWTSecret:   "test-secret-key-that-is-32-chars!!",
		Port:        "8080",
		BcryptCost:  4,
		TokenExpiry: 24 * 1e9 * 3600,
	}
}

func makeRequest(router *gin.Engine, method, path, body string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	var req *http.Request
	if body != "" {
		req, _ = http.NewRequest(method, path, bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
	} else {
		req, _ = http.NewRequest(method, path, nil)
	}
	router.ServeHTTP(w, req)
	return w
}

func TestSignup_Success(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupTestDB()
	cfg := setupTestConfig()
	h := handlers.NewAuthHandler(db, cfg)

	r := gin.New()
	r.POST("/signup", h.Signup)

	w := makeRequest(r, "POST", "/signup", `{"name":"Alice","email":"alice@test.com","password":"secret123"}`)

	if w.Code != http.StatusCreated {
		t.Errorf("expected 201, got %d", w.Code)
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)

	if resp["role"] != "user" {
		t.Errorf("expected role 'user', got %v", resp["role"])
	}
	if resp["password"] != nil {
		t.Error("password should not be in response")
	}
}

func TestSignup_DuplicateEmail(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupTestDB()
	cfg := setupTestConfig()
	h := handlers.NewAuthHandler(db, cfg)

	r := gin.New()
	r.POST("/signup", h.Signup)

	makeRequest(r, "POST", "/signup", `{"name":"Alice","email":"alice@test.com","password":"secret123"}`)
	w := makeRequest(r, "POST", "/signup", `{"name":"Alice","email":"alice@test.com","password":"secret123"}`)

	if w.Code != http.StatusConflict {
		t.Errorf("expected 409, got %d", w.Code)
	}
}

func TestSignup_InvalidEmail(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupTestDB()
	cfg := setupTestConfig()
	h := handlers.NewAuthHandler(db, cfg)

	r := gin.New()
	r.POST("/signup", h.Signup)

	w := makeRequest(r, "POST", "/signup", `{"name":"Alice","email":"notanemail","password":"secret123"}`)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestSignup_ShortPassword(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupTestDB()
	cfg := setupTestConfig()
	h := handlers.NewAuthHandler(db, cfg)

	r := gin.New()
	r.POST("/signup", h.Signup)

	w := makeRequest(r, "POST", "/signup", `{"name":"Alice","email":"alice@test.com","password":"abc"}`)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestSignup_RoleEscalationBlocked(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupTestDB()
	cfg := setupTestConfig()
	h := handlers.NewAuthHandler(db, cfg)

	r := gin.New()
	r.POST("/signup", h.Signup)

	w := makeRequest(r, "POST", "/signup", `{"name":"Alice","email":"alice@test.com","password":"secret123","role":"admin"}`)

	if w.Code != http.StatusCreated {
		t.Errorf("expected 201, got %d", w.Code)
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)

	if resp["role"] != "user" {
		t.Errorf("expected role to be 'user' even if admin was requested, got %v", resp["role"])
	}
}

func TestLogin_Success(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupTestDB()
	cfg := setupTestConfig()
	h := handlers.NewAuthHandler(db, cfg)

	r := gin.New()
	r.POST("/signup", h.Signup)
	r.POST("/login", h.Login)

	makeRequest(r, "POST", "/signup", `{"name":"Alice","email":"alice@test.com","password":"secret123"}`)
	w := makeRequest(r, "POST", "/login", `{"email":"alice@test.com","password":"secret123"}`)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)

	if resp["token"] == nil {
		t.Error("expected token in response")
	}
}

func TestLogin_WrongPassword(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupTestDB()
	cfg := setupTestConfig()
	h := handlers.NewAuthHandler(db, cfg)

	r := gin.New()
	r.POST("/signup", h.Signup)
	r.POST("/login", h.Login)

	makeRequest(r, "POST", "/signup", `{"name":"Alice","email":"alice@test.com","password":"secret123"}`)
	w := makeRequest(r, "POST", "/login", `{"email":"alice@test.com","password":"wrongpassword"}`)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

func TestLogin_UserNotFound(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupTestDB()
	cfg := setupTestConfig()
	h := handlers.NewAuthHandler(db, cfg)

	r := gin.New()
	r.POST("/login", h.Login)

	w := makeRequest(r, "POST", "/login", `{"email":"nobody@test.com","password":"secret123"}`)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}
