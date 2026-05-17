package handlers

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

var serverStart = time.Now()

type HealthHandler struct {
	db *gorm.DB
}

func NewHealthHandler(db *gorm.DB) *HealthHandler {
	return &HealthHandler{db: db}
}

func (h *HealthHandler) Health(c *gin.Context) {
	start := time.Now()
	err := h.db.Exec("SELECT 1").Error
	latency := time.Since(start).Milliseconds()
	uptime := time.Since(serverStart).Seconds()

	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"status":   "degraded",
			"database": "error",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":        "ok",
		"database":      "ok",
		"db_latency_ms": latency,
		"uptime_seconds": uptime,
	})
}
