package handlers

import (
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

	var user models.User
	if err := h.db.Where("email = ?", req.Email).First(&user).Error; err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials", "request_id": c.GetString("requestID")})
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password)); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials", "request_id": c.GetString("requestID")})
		return
	}

	token, err := utils.GenerateToken(user.ID, user.Email, user.Role, h.cfg.JWTSecret, h.cfg.TokenExpiry)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate token", "request_id": c.GetString("requestID")})
		return
	}

	c.JSON(http.StatusOK, gin.H{"token": token})
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
	if ttl <= 0 {
		c.JSON(http.StatusOK, gin.H{"message": "token already expired"})
		return
	}

	if h.cache != nil {
		if err := h.cache.Blacklist(tokenString, ttl); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error":      "failed to revoke token",
				"request_id": c.GetString("requestID"),
			})
			return
		}
	}

	c.JSON(http.StatusOK, gin.H{"message": "logged out successfully"})
}
