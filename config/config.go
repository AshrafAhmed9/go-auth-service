package config

import (
	"log"
	"os"
	"strconv"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	JWTSecret            string
	Port                 string
	BcryptCost           int
	TokenExpiry          time.Duration
	RefreshTokenTTL      time.Duration
	RedisAddr            string
	DBDriver             string
	DatabaseURL          string
	RateLimitRequests    int
	RateLimitWindow      time.Duration
	PerUserLimitRequests int
	PerUserLimitWindow   time.Duration
	LockoutMaxAttempts   int
	LockoutDuration      time.Duration
	GRPCPort             string
}

func Load() *Config {
	if err := godotenv.Load(); err != nil {
		log.Fatal("Error loading .env file")
	}

	secret := os.Getenv("JWT_SECRET")
	if secret == "" {
		log.Fatal("JWT_SECRET is required")
	}
	if len(secret) < 32 {
		log.Fatal("JWT_SECRET must be at least 32 characters")
	}

	cost := 12
	if val := os.Getenv("BCRYPT_COST"); val != "" {
		if parsed, err := strconv.Atoi(val); err == nil {
			cost = parsed
		}
	}

	expiry := 15 * time.Minute
	if val := os.Getenv("ACCESS_TOKEN_MINUTES"); val != "" {
		if minutes, err := strconv.Atoi(val); err == nil {
			expiry = time.Duration(minutes) * time.Minute
		}
	}

	refreshTTL := 7 * 24 * time.Hour
	if val := os.Getenv("REFRESH_TOKEN_HOURS"); val != "" {
		if hours, err := strconv.Atoi(val); err == nil {
			refreshTTL = time.Duration(hours) * time.Hour
		}
	}

	redisAddr := os.Getenv("REDIS_ADDR")
	if redisAddr == "" {
		redisAddr = "localhost:6379"
	}

	dbDriver := os.Getenv("DB_DRIVER")
	if dbDriver == "" {
		dbDriver = "sqlite"
	}

	databaseURL := os.Getenv("DATABASE_URL")

	rateLimitRequests := 5
	if val := os.Getenv("RATE_LIMIT_REQUESTS"); val != "" {
		if parsed, err := strconv.Atoi(val); err == nil {
			rateLimitRequests = parsed
		}
	}

	rateLimitWindowSec := 60
	if val := os.Getenv("RATE_LIMIT_WINDOW_SECONDS"); val != "" {
		if parsed, err := strconv.Atoi(val); err == nil {
			rateLimitWindowSec = parsed
		}
	}

	perUserLimitRequests := 30
	if val := os.Getenv("PER_USER_LIMIT_REQUESTS"); val != "" {
		if parsed, err := strconv.Atoi(val); err == nil {
			perUserLimitRequests = parsed
		}
	}

	perUserLimitWindowSec := 60
	if val := os.Getenv("PER_USER_LIMIT_WINDOW_SECONDS"); val != "" {
		if parsed, err := strconv.Atoi(val); err == nil {
			perUserLimitWindowSec = parsed
		}
	}

	lockoutMaxAttempts := 5
	if val := os.Getenv("LOCKOUT_MAX_ATTEMPTS"); val != "" {
		if parsed, err := strconv.Atoi(val); err == nil {
			lockoutMaxAttempts = parsed
		}
	}

	lockoutDurationMin := 15
	if val := os.Getenv("LOCKOUT_DURATION_MINUTES"); val != "" {
		if parsed, err := strconv.Atoi(val); err == nil {
			lockoutDurationMin = parsed
		}
	}

	grpcPort := os.Getenv("GRPC_PORT")
	if grpcPort == "" {
		grpcPort = "9090"
	}

	return &Config{
		JWTSecret:            secret,
		Port:                 os.Getenv("PORT"),
		BcryptCost:           cost,
		TokenExpiry:          expiry,
		RefreshTokenTTL:      refreshTTL,
		RedisAddr:            redisAddr,
		DBDriver:             dbDriver,
		DatabaseURL:          databaseURL,
		RateLimitRequests:    rateLimitRequests,
		RateLimitWindow:      time.Duration(rateLimitWindowSec) * time.Second,
		PerUserLimitRequests: perUserLimitRequests,
		PerUserLimitWindow:   time.Duration(perUserLimitWindowSec) * time.Second,
		LockoutMaxAttempts:   lockoutMaxAttempts,
		LockoutDuration:      time.Duration(lockoutDurationMin) * time.Minute,
		GRPCPort:             grpcPort,
	}
}
