package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func testDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open test db: %v", err)
	}
	if err := db.AutoMigrate(&User{}, &RefreshToken{}); err != nil {
		t.Fatalf("failed to migrate test db: %v", err)
	}

	// SQLite's :memory: database is per-connection, so a pool of more than
	// one connection would give concurrent requests separate, empty
	// databases. One connection is enough for tests and keeps this real.
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("failed to get underlying sql.DB: %v", err)
	}
	sqlDB.SetMaxOpenConns(1)

	return db
}

func testConfig() *Config {
	return &Config{
		JWTSecret:       testSecret,
		BcryptCost:      4,
		TokenExpiry:     15 * time.Minute,
		RefreshTokenTTL: 7 * 24 * time.Hour,
	}
}

func testRouter(t *testing.T) (*gin.Engine, *AuthHandler) {
	gin.SetMode(gin.TestMode)
	h := newAuthHandler(testDB(t), testConfig(), nil)
	r := gin.New()
	r.POST("/signup", h.Signup)
	r.POST("/login", h.Login)
	r.POST("/refresh", h.Refresh)
	return r, h
}

func doJSON(r *gin.Engine, method, path string, body string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(method, path, bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	return w
}

func field(w *httptest.ResponseRecorder, key string) interface{} {
	var body map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &body)
	return body[key]
}

func signupAndLogin(r *gin.Engine, t *testing.T) map[string]interface{} {
	t.Helper()
	doJSON(r, "POST", "/signup", `{"name":"Alice","email":"alice@test.com","password":"secret123"}`)
	w := doJSON(r, "POST", "/login", `{"email":"alice@test.com","password":"secret123"}`)
	var body map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &body)
	return body
}

func TestSignup_Success(t *testing.T) {
	r, _ := testRouter(t)
	w := doJSON(r, "POST", "/signup", `{"name":"Alice","email":"alice@test.com","password":"secret123"}`)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", w.Code)
	}
	if field(w, "role") != "user" {
		t.Errorf("expected role 'user', got %v", field(w, "role"))
	}
	if field(w, "password") != nil {
		t.Error("password should never appear in the response")
	}
}

func TestSignup_DuplicateEmail(t *testing.T) {
	r, _ := testRouter(t)
	doJSON(r, "POST", "/signup", `{"name":"Alice","email":"alice@test.com","password":"secret123"}`)
	w := doJSON(r, "POST", "/signup", `{"name":"Alice","email":"alice@test.com","password":"secret123"}`)

	if w.Code != http.StatusConflict {
		t.Errorf("expected 409, got %d", w.Code)
	}
}

func TestSignup_InvalidEmail(t *testing.T) {
	r, _ := testRouter(t)
	w := doJSON(r, "POST", "/signup", `{"name":"Alice","email":"notanemail","password":"secret123"}`)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestSignup_ShortPassword(t *testing.T) {
	r, _ := testRouter(t)
	w := doJSON(r, "POST", "/signup", `{"name":"Alice","email":"alice@test.com","password":"abc"}`)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestSignup_RoleEscalationBlocked(t *testing.T) {
	r, _ := testRouter(t)
	w := doJSON(r, "POST", "/signup", `{"name":"Alice","email":"alice@test.com","password":"secret123","role":"admin"}`)

	if field(w, "role") != "user" {
		t.Errorf("expected role forced to 'user' even when 'admin' was requested, got %v", field(w, "role"))
	}
}

func TestLogin_Success(t *testing.T) {
	r, _ := testRouter(t)
	doJSON(r, "POST", "/signup", `{"name":"Alice","email":"alice@test.com","password":"secret123"}`)
	w := doJSON(r, "POST", "/login", `{"email":"alice@test.com","password":"secret123"}`)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if field(w, "access_token") == nil || field(w, "refresh_token") == nil {
		t.Error("expected both access_token and refresh_token")
	}
}

func TestLogin_WrongPassword(t *testing.T) {
	r, _ := testRouter(t)
	doJSON(r, "POST", "/signup", `{"name":"Alice","email":"alice@test.com","password":"secret123"}`)
	w := doJSON(r, "POST", "/login", `{"email":"alice@test.com","password":"wrongpassword"}`)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

func TestLogin_UserNotFound(t *testing.T) {
	r, _ := testRouter(t)
	w := doJSON(r, "POST", "/login", `{"email":"nobody@test.com","password":"secret123"}`)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

func TestRefresh_RotatesToken(t *testing.T) {
	r, _ := testRouter(t)
	login := signupAndLogin(r, t)
	oldRefresh := login["refresh_token"].(string)

	body, _ := json.Marshal(map[string]string{"refresh_token": oldRefresh})
	w := doJSON(r, "POST", "/refresh", string(body))

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if field(w, "refresh_token") == oldRefresh {
		t.Error("expected a rotated refresh_token, got the same one back")
	}
}

func TestRefresh_ReplayIsRejected(t *testing.T) {
	r, _ := testRouter(t)
	login := signupAndLogin(r, t)
	refreshToken := login["refresh_token"].(string)

	body, _ := json.Marshal(map[string]string{"refresh_token": refreshToken})
	doJSON(r, "POST", "/refresh", string(body))
	w := doJSON(r, "POST", "/refresh", string(body))

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 replaying a used refresh token, got %d", w.Code)
	}
}

func TestRefresh_ReplayRevokesEverySession(t *testing.T) {
	r, _ := testRouter(t)
	login := signupAndLogin(r, t)
	oldRefresh := login["refresh_token"].(string)

	body, _ := json.Marshal(map[string]string{"refresh_token": oldRefresh})
	firstRefresh := doJSON(r, "POST", "/refresh", string(body))
	newRefresh := field(firstRefresh, "refresh_token").(string)

	// Replay the already-used token — this is the theft signal.
	doJSON(r, "POST", "/refresh", string(body))

	// The legitimate follow-up token must now be dead too.
	newBody, _ := json.Marshal(map[string]string{"refresh_token": newRefresh})
	w := doJSON(r, "POST", "/refresh", string(newBody))

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected the newer refresh token to be revoked after reuse was detected, got %d", w.Code)
	}
}

func TestRefresh_InvalidToken(t *testing.T) {
	r, _ := testRouter(t)
	w := doJSON(r, "POST", "/refresh", `{"refresh_token":"totally-fake-token"}`)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

// TestRefresh_ConcurrentReplayOnlySucceedsOnce is the test that would have
// failed on the old read-then-write rotation: two goroutines race to
// refresh the same token, and exactly one may win.
func TestRefresh_ConcurrentReplayOnlySucceedsOnce(t *testing.T) {
	r, _ := testRouter(t)
	login := signupAndLogin(r, t)
	refreshToken := login["refresh_token"].(string)
	body, _ := json.Marshal(map[string]string{"refresh_token": refreshToken})

	var wg sync.WaitGroup
	codes := make([]int, 10)
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			codes[i] = doJSON(r, "POST", "/refresh", string(body)).Code
		}(i)
	}
	wg.Wait()

	successes := 0
	for _, code := range codes {
		if code == http.StatusOK {
			successes++
		}
	}
	if successes != 1 {
		t.Errorf("expected exactly 1 of 10 concurrent replays to succeed, got %d", successes)
	}
}
