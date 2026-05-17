package middleware

import (
	"net/http"
	"os"
	"strings"

	"github.com/AshrafAhmed9/assignment-golang/utils"
	"github.com/gin-gonic/gin"
)

func JWTAuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error":      "authorization header required",
				"request_id": c.GetString("requestID"),
			})
			c.Abort()
			return
		}

		tokenString := strings.TrimPrefix(authHeader, "Bearer ")

		claims, err := utils.ParseToken(tokenString, os.Getenv("JWT_SECRET"))
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error":      "invalid or expired token",
				"request_id": c.GetString("requestID"),
			})
			c.Abort()
			return
		}

		c.Set("userID", claims.UserID)
		c.Set("email", claims.Email)
		c.Set("role", claims.Role)

		c.Next()
	}
}

func AdminOnlyMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		role := c.GetString("role")
		if role != "admin" {
			c.JSON(http.StatusForbidden, gin.H{
				"error":      "admin access required",
				"request_id": c.GetString("requestID"),
			})
			c.Abort()
			return
		}
		c.Next()
	}
}
