package tests

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/AshrafAhmed9/assignment-golang/config"
	"github.com/AshrafAhmed9/assignment-golang/handlers"
	"github.com/AshrafAhmed9/assignment-golang/models"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func setupTestDB() *gorm.DB {
	db, _ := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	db.AutoMigrate(&models.User{}, &models.RefreshToken{}, &models.AuditEvent{})
	return db
}

func setupTestConfig() *config.Config {
	return &config.Config{
		JWTSecret:            "test-secret-key-that-is-32-chars!!",
		Port:                 "8080",
		BcryptCost:           4,
		TokenExpiry:          15 * time.Minute,
		RefreshTokenTTL:      7 * 24 * time.Hour,
		LockoutMaxAttempts:   5,
		LockoutDuration:      15 * time.Minute,
		RateLimitRequests:    5,
		RateLimitWindow:      time.Minute,
		PerUserLimitRequests: 30,
		PerUserLimitWindow:   time.Minute,
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
	h := handlers.NewAuthHandler(db, cfg, nil)

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
	h := handlers.NewAuthHandler(db, cfg, nil)

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
	h := handlers.NewAuthHandler(db, cfg, nil)

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
	h := handlers.NewAuthHandler(db, cfg, nil)

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
	h := handlers.NewAuthHandler(db, cfg, nil)

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
	h := handlers.NewAuthHandler(db, cfg, nil)

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

	if resp["access_token"] == nil {
		t.Error("expected access_token in response")
	}
	if resp["refresh_token"] == nil {
		t.Error("expected refresh_token in response")
	}
	if resp["token_type"] != "Bearer" {
		t.Errorf("expected token_type 'Bearer', got %v", resp["token_type"])
	}
	if resp["expires_in"] == nil {
		t.Error("expected expires_in in response")
	}
}

func TestLogin_WrongPassword(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupTestDB()
	cfg := setupTestConfig()
	h := handlers.NewAuthHandler(db, cfg, nil)

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
	h := handlers.NewAuthHandler(db, cfg, nil)

	r := gin.New()
	r.POST("/login", h.Login)

	w := makeRequest(r, "POST", "/login", `{"email":"nobody@test.com","password":"secret123"}`)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

func TestRefresh_Success(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupTestDB()
	cfg := setupTestConfig()
	h := handlers.NewAuthHandler(db, cfg, nil)

	r := gin.New()
	r.POST("/signup", h.Signup)
	r.POST("/login", h.Login)
	r.POST("/refresh", h.Refresh)

	makeRequest(r, "POST", "/signup", `{"name":"Alice","email":"alice@test.com","password":"secret123"}`)
	loginResp := makeRequest(r, "POST", "/login", `{"email":"alice@test.com","password":"secret123"}`)

	var loginBody map[string]interface{}
	json.Unmarshal(loginResp.Body.Bytes(), &loginBody)
	refreshToken := loginBody["refresh_token"].(string)

	body, _ := json.Marshal(map[string]string{"refresh_token": refreshToken})
	w := makeRequest(r, "POST", "/refresh", string(body))

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)

	if resp["access_token"] == nil {
		t.Error("expected new access_token")
	}
	if resp["refresh_token"] == nil {
		t.Error("expected new refresh_token")
	}
	if resp["refresh_token"] == refreshToken {
		t.Error("expected rotated refresh_token, got same one")
	}
}

func TestRefresh_OldTokenRejected(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupTestDB()
	cfg := setupTestConfig()
	h := handlers.NewAuthHandler(db, cfg, nil)

	r := gin.New()
	r.POST("/signup", h.Signup)
	r.POST("/login", h.Login)
	r.POST("/refresh", h.Refresh)

	makeRequest(r, "POST", "/signup", `{"name":"Alice","email":"alice@test.com","password":"secret123"}`)
	loginResp := makeRequest(r, "POST", "/login", `{"email":"alice@test.com","password":"secret123"}`)

	var loginBody map[string]interface{}
	json.Unmarshal(loginResp.Body.Bytes(), &loginBody)
	refreshToken := loginBody["refresh_token"].(string)

	body, _ := json.Marshal(map[string]string{"refresh_token": refreshToken})
	makeRequest(r, "POST", "/refresh", string(body))

	w := makeRequest(r, "POST", "/refresh", string(body))

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 for reused refresh token, got %d", w.Code)
	}
}

func TestRefresh_ReuseRevokesAllSessions(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupTestDB()
	cfg := setupTestConfig()
	h := handlers.NewAuthHandler(db, cfg, nil)

	r := gin.New()
	r.POST("/signup", h.Signup)
	r.POST("/login", h.Login)
	r.POST("/refresh", h.Refresh)

	makeRequest(r, "POST", "/signup", `{"name":"Alice","email":"alice@test.com","password":"secret123"}`)
	loginResp := makeRequest(r, "POST", "/login", `{"email":"alice@test.com","password":"secret123"}`)

	var loginBody map[string]interface{}
	json.Unmarshal(loginResp.Body.Bytes(), &loginBody)
	oldRefresh := loginBody["refresh_token"].(string)

	body, _ := json.Marshal(map[string]string{"refresh_token": oldRefresh})
	refreshResp := makeRequest(r, "POST", "/refresh", string(body))

	var refreshBody map[string]interface{}
	json.Unmarshal(refreshResp.Body.Bytes(), &refreshBody)
	newRefresh := refreshBody["refresh_token"].(string)

	makeRequest(r, "POST", "/refresh", string(body))

	newBody, _ := json.Marshal(map[string]string{"refresh_token": newRefresh})
	w := makeRequest(r, "POST", "/refresh", string(newBody))

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 after reuse detection revoked all sessions, got %d", w.Code)
	}
}

func TestRefresh_InvalidToken(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupTestDB()
	cfg := setupTestConfig()
	h := handlers.NewAuthHandler(db, cfg, nil)

	r := gin.New()
	r.POST("/refresh", h.Refresh)

	w := makeRequest(r, "POST", "/refresh", `{"refresh_token":"totally-fake-token"}`)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

func TestAudit_LoginCreatesEvent(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupTestDB()
	cfg := setupTestConfig()
	h := handlers.NewAuthHandler(db, cfg, nil)

	r := gin.New()
	r.POST("/signup", h.Signup)
	r.POST("/login", h.Login)

	makeRequest(r, "POST", "/signup", `{"name":"Alice","email":"alice@test.com","password":"secret123"}`)
	makeRequest(r, "POST", "/login", `{"email":"alice@test.com","password":"secret123"}`)

	var events []models.AuditEvent
	db.Where("event_type = ?", "login_success").Find(&events)

	if len(events) != 1 {
		t.Errorf("expected 1 login_success audit event, got %d", len(events))
	}
	if events[0].Email != "alice@test.com" {
		t.Errorf("expected email alice@test.com, got %s", events[0].Email)
	}
}

func TestAudit_FailedLoginCreatesEvent(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupTestDB()
	cfg := setupTestConfig()
	h := handlers.NewAuthHandler(db, cfg, nil)

	r := gin.New()
	r.POST("/signup", h.Signup)
	r.POST("/login", h.Login)

	makeRequest(r, "POST", "/signup", `{"name":"Alice","email":"alice@test.com","password":"secret123"}`)
	makeRequest(r, "POST", "/login", `{"email":"alice@test.com","password":"wrongpassword"}`)

	var events []models.AuditEvent
	db.Where("event_type = ?", "login_failure").Find(&events)

	if len(events) != 1 {
		t.Errorf("expected 1 login_failure audit event, got %d", len(events))
	}
}

func TestAudit_SignupCreatesEvent(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupTestDB()
	cfg := setupTestConfig()
	h := handlers.NewAuthHandler(db, cfg, nil)

	r := gin.New()
	r.POST("/signup", h.Signup)

	makeRequest(r, "POST", "/signup", `{"name":"Alice","email":"alice@test.com","password":"secret123"}`)

	var events []models.AuditEvent
	db.Where("event_type = ?", "signup").Find(&events)

	if len(events) != 1 {
		t.Errorf("expected 1 signup audit event, got %d", len(events))
	}
}
