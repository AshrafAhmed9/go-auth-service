package config

import (
	"log"
	"os"
	"strconv"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	JWTSecret   string
	Port        string
	BcryptCost  int
	TokenExpiry time.Duration
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

	expiry := 24 * time.Hour
	if val := os.Getenv("TOKEN_EXPIRY_HOURS"); val != "" {
		if hours, err := strconv.Atoi(val); err == nil {
			expiry = time.Duration(hours) * time.Hour
		}
	}

	return &Config{
		JWTSecret:   secret,
		Port:        os.Getenv("PORT"),
		BcryptCost:  cost,
		TokenExpiry: expiry,
	}
}
