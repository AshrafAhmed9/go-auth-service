package main

// Signup, login, refresh and logout. The interesting design decision is in
// refreshTokens: rotation is a single conditional UPDATE (WHERE
// revoked_at IS NULL), not a read-then-write. That closes a race where two
// requests replaying the same refresh token at the same instant could both
// read "not yet revoked" and both succeed — the whole point of reuse
// detection is that a replay must always lose.
import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

type AuthHandler struct {
	db    *gorm.DB
	cfg   *Config
	cache *RedisClient
}

func newAuthHandler(db *gorm.DB, cfg *Config, cache *RedisClient) *AuthHandler {
	return &AuthHandler{db: db, cfg: cfg, cache: cache}
}

type signupRequest struct {
	Name     string `json:"name"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type refreshRequest struct {
	RefreshToken string `json:"refresh_token"`
}

func (h *AuthHandler) Signup(c *gin.Context) {
	var req signupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, http.StatusBadRequest, "invalid request body")
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	req.Email = strings.ToLower(strings.TrimSpace(req.Email))

	if req.Name == "" || req.Email == "" || req.Password == "" {
		fail(c, http.StatusBadRequest, "name, email and password are required")
		return
	}
	if !strings.Contains(req.Email, "@") {
		fail(c, http.StatusBadRequest, "invalid email")
		return
	}
	if len(req.Password) < 8 {
		fail(c, http.StatusBadRequest, "password must be at least 8 characters")
		return
	}

	var existing User
	if err := h.db.Where("email = ?", req.Email).First(&existing).Error; err == nil {
		fail(c, http.StatusConflict, "email already registered")
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), h.cfg.BcryptCost)
	if err != nil {
		fail(c, http.StatusInternalServerError, "failed to hash password")
		return
	}

	// Role is always "user" here — never taken from the request — so a
	// signup call can never grant itself admin.
	user := User{Name: req.Name, Email: req.Email, Password: string(hash), Role: "user"}
	if err := h.db.Create(&user).Error; err != nil {
		fail(c, http.StatusInternalServerError, "failed to create user")
		return
	}

	c.JSON(http.StatusCreated, user)
}

func (h *AuthHandler) Login(c *gin.Context) {
	var req loginRequest
	if err := c.ShouldBindJSON(&req); err != nil || req.Email == "" || req.Password == "" {
		fail(c, http.StatusBadRequest, "email and password are required")
		return
	}
	req.Email = strings.ToLower(strings.TrimSpace(req.Email))

	var user User
	if err := h.db.Where("email = ?", req.Email).First(&user).Error; err != nil {
		fail(c, http.StatusUnauthorized, "invalid credentials")
		return
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password)); err != nil {
		fail(c, http.StatusUnauthorized, "invalid credentials")
		return
	}

	h.issueTokenPair(c, &user)
}

func (h *AuthHandler) Refresh(c *gin.Context) {
	var req refreshRequest
	if err := c.ShouldBindJSON(&req); err != nil || req.RefreshToken == "" {
		fail(c, http.StatusBadRequest, "refresh_token is required")
		return
	}

	tokenHash := hashToken(req.RefreshToken)

	var rt RefreshToken
	if err := h.db.Where("token_hash = ?", tokenHash).First(&rt).Error; err != nil {
		fail(c, http.StatusUnauthorized, "invalid refresh token")
		return
	}

	// Atomic claim: only the first caller to reach this row while it's
	// still unrevoked wins. A concurrent or later replay affects zero rows.
	result := h.db.Model(&RefreshToken{}).
		Where("id = ? AND revoked_at IS NULL", rt.ID).
		Update("revoked_at", time.Now())

	if result.RowsAffected == 0 {
		h.db.Model(&RefreshToken{}).
			Where("user_id = ? AND revoked_at IS NULL", rt.UserID).
			Update("revoked_at", time.Now())
		fail(c, http.StatusUnauthorized, "refresh token reuse detected, all sessions revoked")
		return
	}

	if time.Now().After(rt.ExpiresAt) {
		fail(c, http.StatusUnauthorized, "refresh token expired")
		return
	}

	var user User
	if err := h.db.First(&user, rt.UserID).Error; err != nil {
		fail(c, http.StatusUnauthorized, "user not found")
		return
	}

	h.issueTokenPair(c, &user)
}

func (h *AuthHandler) Logout(c *gin.Context) {
	claims := c.MustGet("claims").(*Claims)

	ttl := time.Until(claims.ExpiresAt.Time)
	if ttl > 0 && h.cache != nil {
		if err := h.cache.Blacklist(claims.ID, ttl); err != nil {
			fail(c, http.StatusInternalServerError, "failed to revoke token")
			return
		}
	}

	h.db.Model(&RefreshToken{}).
		Where("user_id = ? AND revoked_at IS NULL", claims.UserID).
		Update("revoked_at", time.Now())

	c.JSON(http.StatusOK, gin.H{"message": "logged out successfully"})
}

// issueTokenPair mints a fresh access token plus a fresh, rotated refresh
// token, and stores only the refresh token's hash.
func (h *AuthHandler) issueTokenPair(c *gin.Context, user *User) {
	accessToken, err := generateToken(user.ID, user.Email, user.Role, h.cfg.JWTSecret, h.cfg.TokenExpiry)
	if err != nil {
		fail(c, http.StatusInternalServerError, "failed to generate token")
		return
	}

	rawRefresh, hash, err := newRefreshToken()
	if err != nil {
		fail(c, http.StatusInternalServerError, "failed to generate refresh token")
		return
	}

	rt := RefreshToken{UserID: user.ID, TokenHash: hash, ExpiresAt: time.Now().Add(h.cfg.RefreshTokenTTL)}
	if err := h.db.Create(&rt).Error; err != nil {
		fail(c, http.StatusInternalServerError, "failed to store refresh token")
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"access_token":  accessToken,
		"refresh_token": rawRefresh,
		"token_type":    "Bearer",
		"expires_in":    int(h.cfg.TokenExpiry.Seconds()),
	})
}

// requireAuth protects a route with a valid, non-blacklisted access token.
func requireAuth(cfg *Config, cache *RedisClient) gin.HandlerFunc {
	return func(c *gin.Context) {
		header := c.GetHeader("Authorization")
		if !strings.HasPrefix(header, "Bearer ") {
			fail(c, http.StatusUnauthorized, "authorization header required")
			c.Abort()
			return
		}

		claims, err := parseToken(strings.TrimPrefix(header, "Bearer "), cfg.JWTSecret)
		if err != nil {
			fail(c, http.StatusUnauthorized, "invalid or expired token")
			c.Abort()
			return
		}
		if cache != nil && cache.IsBlacklisted(claims.ID) {
			fail(c, http.StatusUnauthorized, "token has been revoked")
			c.Abort()
			return
		}

		c.Set("claims", claims)
		c.Next()
	}
}

func fail(c *gin.Context, status int, message string) {
	c.JSON(status, gin.H{"error": message})
}

func newRefreshToken() (raw string, hash string, err error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", "", err
	}
	raw = hex.EncodeToString(b)
	return raw, hashToken(raw), nil
}

func hashToken(token string) string {
	h := sha256.Sum256([]byte(token))
	return hex.EncodeToString(h[:])
}
