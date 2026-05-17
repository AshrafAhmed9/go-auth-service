package main

import (
	"github.com/AshrafAhmed9/assignment-golang/config"
	"github.com/AshrafAhmed9/assignment-golang/database"
	"github.com/AshrafAhmed9/assignment-golang/handlers"
	"github.com/AshrafAhmed9/assignment-golang/middleware"
	"github.com/gin-gonic/gin"
)


func main() {
	cfg := config.Load()
	db := database.Connect()

	authHandler := handlers.NewAuthHandler(db)
	userHandler := handlers.NewUserHandler(db)

	r := gin.Default()

	r.POST("/signup", authHandler.Signup)
	r.POST("/login", authHandler.Login)

	protected := r.Group("/")
	protected.Use(middleware.JWTAuthMiddleware())
	{
		protected.GET("/profile", userHandler.Profile)

		admin := protected.Group("/")
		admin.Use(middleware.AdminOnlyMiddleware())
		{
			admin.GET("/users", userHandler.GetAllUsers)
		}
	}

	r.Run(":" + cfg.Port)
}
