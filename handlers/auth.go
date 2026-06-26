package handlers

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/AshrafAhmed9/assignment-golang/cache"
	"github.com/AshrafAhmed9/assignment-golang/config"
	"github.com/AshrafAhmed9/assignment-golang/models"
	"github.com/AshrafAhmed9/assignment-golang/utils"
	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

type AuthHandler struct {
	db    *gorm.DB
	cfg   *config.Config
	cache *cache.RedisClient
}

func NewAuthHandler(db *gorm.DB, cfg *config.Config, rdb *cache.RedisClient) *AuthHandler {
	return &AuthHandler{db: db, cfg: cfg, cache: rdb}
}

type SignupRequest struct {
	Name     string `json:"name"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type RefreshRequest struct {
	RefreshToken string `json:"refresh_token"`
}

func (h *AuthHandler) Signup(c *gin.Context) {
	var req SignupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body", "request_id": c.GetString("requestID")})
		return
	}

	if req.Name == "" || req.Email == "" || req.Password == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "name, email and password are required", "request_id": c.GetString("requestID")})
		return
	}

	if !strings.Contains(req.Email, "@") {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid email", "request_id": c.GetString("requestID")})
		return
	}

	if len(req.Password) < 8 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "password must be at least 8 characters", "request_id": c.GetString("requestID")})
		return
	}

	req.Name = strings.TrimSpace(req.Name)
	req.Email = strings.ToLower(strings.TrimSpace(req.Email))

	var existing models.User
	if err := h.db.Where("email = ?", req.Email).First(&existing).Error; err == nil {
		c.JSON(http.StatusConflict, gin.H{"error": "email already registered", "request_id": c.GetString("requestID")})
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), h.cfg.BcryptCost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to hash password", "request_id": c.GetString("requestID")})
		return
	}

	user := models.User{
		Name:     req.Name,
		Email:    req.Email,
		Password: string(hash),
		Role:     "user",
	}

	if err := h.db.Create(&user).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create user", "request_id": c.GetString("requestID")})
		return
	}

	writeAuditEvent(h.db, EventSignup, &user.ID, user.Email, c.ClientIP(), c.GetString("requestID"), "")

	c.JSON(http.StatusCreated, user)
}

func (h *AuthHandler) Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body", "request_id": c.GetString("requestID")})
		return
	}

	if req.Email == "" || req.Password == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "email and password are required", "request_id": c.GetString("requestID")})
		return
	}

	req.Email = strings.ToLower(strings.TrimSpace(req.Email))

	if h.cache != nil && h.cache.IsLocked(req.Email) {
		writeAuditEvent(h.db, EventLoginLocked, nil, req.Email, c.ClientIP(), c.GetString("requestID"), "account locked")
		c.JSON(http.StatusTooManyRequests, gin.H{
			"error":      "account temporarily locked due to too many failed attempts",
			"request_id": c.GetString("requestID"),
		})
		return
	}

	var user models.User
	if err := h.db.Where("email = ?", req.Email).First(&user).Error; err != nil {
		h.recordFailedLogin(req.Email, c)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials", "request_id": c.GetString("requestID")})
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password)); err != nil {
		h.recordFailedLogin(req.Email, c)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials", "request_id": c.GetString("requestID")})
		return
	}

	if h.cache != nil {
		h.cache.ClearFailedAttempts(req.Email)
	}

	accessToken, err := utils.GenerateToken(user.ID, user.Email, user.Role, h.cfg.JWTSecret, h.cfg.TokenExpiry)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate token", "request_id": c.GetString("requestID")})
		return
	}

	rawRefresh, hashRefresh, err := utils.GenerateRefreshToken()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate refresh token", "request_id": c.GetString("requestID")})
		return
	}

	rt := models.RefreshToken{
		UserID:    user.ID,
		TokenHash: hashRefresh,
		ExpiresAt: time.Now().Add(h.cfg.RefreshTokenTTL),
	}
	if err := h.db.Create(&rt).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to store refresh token", "request_id": c.GetString("requestID")})
		return
	}

	writeAuditEvent(h.db, EventLoginSuccess, &user.ID, user.Email, c.ClientIP(), c.GetString("requestID"), "")

	c.JSON(http.StatusOK, gin.H{
		"access_token":  accessToken,
		"refresh_token": rawRefresh,
		"token_type":    "Bearer",
		"expires_in":    int(h.cfg.TokenExpiry.Seconds()),
	})
}

func (h *AuthHandler) recordFailedLogin(email string, c *gin.Context) {
	writeAuditEvent(h.db, EventLoginFailure, nil, email, c.ClientIP(), c.GetString("requestID"), "")

	if h.cache == nil {
		return
	}

	count, err := h.cache.IncrementFailedAttempts(email, h.cfg.LockoutDuration)
	if err != nil {
		return
	}

	if count >= int64(h.cfg.LockoutMaxAttempts) {
		h.cache.LockAccount(email, h.cfg.LockoutDuration)
		writeAuditEvent(h.db, EventLoginLocked, nil, email, c.ClientIP(), c.GetString("requestID"),
			fmt.Sprintf("locked after %d failed attempts", count))
	}
}

func (h *AuthHandler) Refresh(c *gin.Context) {
	var req RefreshRequest
	if err := c.ShouldBindJSON(&req); err != nil || req.RefreshToken == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "refresh_token is required", "request_id": c.GetString("requestID")})
		return
	}

	tokenHash := utils.HashToken(req.RefreshToken)

	var rt models.RefreshToken
	if err := h.db.Where("token_hash = ?", tokenHash).First(&rt).Error; err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid refresh token", "request_id": c.GetString("requestID")})
		return
	}

	if rt.RevokedAt != nil {
		h.db.Model(&models.RefreshToken{}).
			Where("user_id = ? AND revoked_at IS NULL", rt.UserID).
			Update("revoked_at", time.Now())
		c.JSON(http.StatusUnauthorized, gin.H{"error": "refresh token reuse detected, all sessions revoked", "request_id": c.GetString("requestID")})
		return
	}

	if time.Now().After(rt.ExpiresAt) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "refresh token expired", "request_id": c.GetString("requestID")})
		return
	}

	now := time.Now()
	h.db.Model(&rt).Update("revoked_at", now)

	var user models.User
	if err := h.db.First(&user, rt.UserID).Error; err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "user not found", "request_id": c.GetString("requestID")})
		return
	}

	accessToken, err := utils.GenerateToken(user.ID, user.Email, user.Role, h.cfg.JWTSecret, h.cfg.TokenExpiry)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate token", "request_id": c.GetString("requestID")})
		return
	}

	rawRefresh, hashRefresh, err := utils.GenerateRefreshToken()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate refresh token", "request_id": c.GetString("requestID")})
		return
	}

	newRT := models.RefreshToken{
		UserID:    user.ID,
		TokenHash: hashRefresh,
		ExpiresAt: time.Now().Add(h.cfg.RefreshTokenTTL),
	}
	if err := h.db.Create(&newRT).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to store refresh token", "request_id": c.GetString("requestID")})
		return
	}

	writeAuditEvent(h.db, EventTokenRefresh, &user.ID, user.Email, c.ClientIP(), c.GetString("requestID"), "")

	c.JSON(http.StatusOK, gin.H{
		"access_token":  accessToken,
		"refresh_token": rawRefresh,
		"token_type":    "Bearer",
		"expires_in":    int(h.cfg.TokenExpiry.Seconds()),
	})
}

func (h *AuthHandler) Logout(c *gin.Context) {
	tokenString := c.GetString("token")
	if tokenString == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "no token found",
			"request_id": c.GetString("requestID"),
		})
		return
	}

	claims, err := utils.ParseToken(tokenString, h.cfg.JWTSecret)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error":      "invalid token",
			"request_id": c.GetString("requestID"),
		})
		return
	}

	ttl := time.Until(claims.ExpiresAt.Time)
	if ttl > 0 && h.cache != nil {
		if err := h.cache.Blacklist(claims.ID, ttl); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error":      "failed to revoke token",
				"request_id": c.GetString("requestID"),
			})
			return
		}
	}

	h.db.Model(&models.RefreshToken{}).
		Where("user_id = ? AND revoked_at IS NULL", claims.UserID).
		Update("revoked_at", time.Now())

	writeAuditEvent(h.db, EventLogout, &claims.UserID, claims.Email, c.ClientIP(), c.GetString("requestID"), "")

	c.JSON(http.StatusOK, gin.H{"message": "logged out successfully"})
}
