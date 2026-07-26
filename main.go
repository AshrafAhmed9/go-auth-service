package main

// Every route this service exposes is listed right here, in one place, so
// "what can this service do" is answerable by reading one function.
import (
	"log"
	"net"
	"net/http"

	"github.com/gin-gonic/gin"
	"google.golang.org/grpc"
)

func main() {
	cfg := loadConfig()
	db := openStore(cfg)
	redis := newRedisClient(cfg.RedisAddr)
	limiter := newRateLimiter(redis, cfg.RateLimitRequests, cfg.RateLimitWindow)
	auth := newAuthHandler(db, cfg, redis)

	router := gin.New()
	router.Use(gin.Recovery())

	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})
	router.POST("/signup", auth.Signup)
	router.POST("/login", limiter.Middleware(), auth.Login)
	router.POST("/refresh", auth.Refresh)
	router.POST("/logout", requireAuth(cfg, redis), auth.Logout)

	go serveGRPC(cfg, redis)

	log.Println("server started on port", cfg.Port)
	if err := router.Run(":" + cfg.Port); err != nil {
		log.Fatal("server error: ", err)
	}
}

func serveGRPC(cfg *Config, redis *RedisClient) {
	lis, err := net.Listen("tcp", ":"+cfg.GRPCPort)
	if err != nil {
		log.Fatal("failed to listen for gRPC: ", err)
	}

	srv := grpc.NewServer()
	registerGRPCServer(srv, cfg.JWTSecret, redis)

	log.Println("gRPC server started on port", cfg.GRPCPort)
	if err := srv.Serve(lis); err != nil {
		log.Fatal("gRPC server error: ", err)
	}
}
