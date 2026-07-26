package main

// Config holds every setting the service needs, read once at startup.
// Loading fails loudly (log.Fatal) rather than silently running with an
// insecure default — a service that can't start safely shouldn't start.
import (
	"log"
	"os"
	"strconv"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	JWTSecret         string
	Port              string
	GRPCPort          string
	BcryptCost        int
	TokenExpiry       time.Duration
	RefreshTokenTTL   time.Duration
	RedisAddr         string
	DBDriver          string
	DatabaseURL       string
	RateLimitRequests int
	RateLimitWindow   time.Duration
}

func loadConfig() *Config {
	_ = godotenv.Load()

	secret := os.Getenv("JWT_SECRET")
	if len(secret) < 32 {
		log.Fatal("JWT_SECRET must be set and at least 32 characters")
	}

	return &Config{
		JWTSecret:         secret,
		Port:              envOr("PORT", "8080"),
		GRPCPort:          envOr("GRPC_PORT", "9090"),
		BcryptCost:        envInt("BCRYPT_COST", 12),
		TokenExpiry:       time.Duration(envInt("ACCESS_TOKEN_MINUTES", 15)) * time.Minute,
		RefreshTokenTTL:   time.Duration(envInt("REFRESH_TOKEN_HOURS", 168)) * time.Hour,
		RedisAddr:         envOr("REDIS_ADDR", "localhost:6379"),
		DBDriver:          envOr("DB_DRIVER", "sqlite"),
		DatabaseURL:       os.Getenv("DATABASE_URL"),
		RateLimitRequests: envInt("RATE_LIMIT_REQUESTS", 5),
		RateLimitWindow:   time.Duration(envInt("RATE_LIMIT_WINDOW_SECONDS", 60)) * time.Second,
	}
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func envInt(key string, fallback int) int {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return fallback
	}
	return n
}
