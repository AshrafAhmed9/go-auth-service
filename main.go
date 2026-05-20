package main

import (
	"context"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/AshrafAhmed9/assignment-golang/cache"
	"github.com/AshrafAhmed9/assignment-golang/config"
	"github.com/AshrafAhmed9/assignment-golang/database"
	"github.com/AshrafAhmed9/assignment-golang/handlers"
	"github.com/AshrafAhmed9/assignment-golang/middleware"
	"github.com/gin-gonic/gin"
)

func main() {
	cfg := config.Load()
	db := database.Connect(cfg.BcryptCost)

	rdb := cache.NewRedisClient(cfg.RedisAddr)
	middleware.SetRedisClient(rdb)

	authHandler := handlers.NewAuthHandler(db, cfg, rdb)
	userHandler := handlers.NewUserHandler(db)
	healthHandler := handlers.NewHealthHandler(db)

	r := gin.New()
	r.Use(middleware.SecurityHeaders())
	r.Use(middleware.RequestID())
	r.Use(gin.Recovery())
	r.Use(func(c *gin.Context) {
		start := time.Now()
		c.Next()
		slog.Info("request",
			"method", c.Request.Method,
			"path", c.Request.URL.Path,
			"status", c.Writer.Status(),
			"latency_ms", time.Since(start).Milliseconds(),
			"request_id", c.GetString("requestID"),
		)
	})

	r.GET("/health", healthHandler.Health)
	r.POST("/signup", authHandler.Signup)
	r.POST("/login", middleware.RateLimitMiddleware(), authHandler.Login)

	protected := r.Group("/")
	protected.Use(middleware.JWTAuthMiddleware())
	{
		protected.GET("/profile", userHandler.Profile)
		protected.POST("/logout", authHandler.Logout)

		admin := protected.Group("/")
		admin.Use(middleware.AdminOnlyMiddleware())
		{
			admin.GET("/users", userHandler.GetAllUsers)
		}
	}

	srv := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      r,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  30 * time.Second,
	}

	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal("server error:", err)
		}
	}()

	slog.Info("server started", "port", cfg.Port)

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	slog.Info("shutting down server...")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	srv.Shutdown(ctx)
	slog.Info("server stopped")
}
