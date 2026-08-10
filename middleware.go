package main

// requireAuth is the guard that wraps a protected route. It runs before the
// handler and answers one question: is this a real, still-valid token? The
// blacklist check is what makes logout mean something — a signature stays
// valid until the token expires, so the only way to reject a revoked token
// is to look it up.
import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

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

		// Handlers read the caller's identity from here rather than
		// re-parsing the token themselves.
		c.Set("claims", claims)
		c.Next()
	}
}
