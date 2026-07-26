package main

// openStore connects to Postgres in production or a local SQLite file in
// development, using the same GORM models either way. A single admin
// account is seeded on first run so there's always a way to exercise
// admin-only behaviour in the Spring Boot service that consumes this API.
import (
	"log"
	"os"

	"github.com/glebarez/sqlite"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func openStore(cfg *Config) *gorm.DB {
	db := connect(cfg.DBDriver, cfg.DatabaseURL)
	seedAdmin(db, cfg.BcryptCost)
	return db
}

func connect(driver, databaseURL string) *gorm.DB {
	if driver == "postgres" {
		if databaseURL == "" {
			log.Fatal("DATABASE_URL is required when DB_DRIVER is postgres")
		}
		db, err := gorm.Open(postgres.Open(databaseURL), &gorm.Config{})
		if err != nil {
			log.Fatal("failed to connect to postgres: ", err)
		}
		return db
	}

	if err := os.MkdirAll("data", os.ModePerm); err != nil {
		log.Fatal("failed to create data directory: ", err)
	}
	db, err := gorm.Open(sqlite.Open("data/app.db"), &gorm.Config{})
	if err != nil {
		log.Fatal("failed to connect to sqlite: ", err)
	}
	db.AutoMigrate(&User{}, &RefreshToken{})
	return db
}

func seedAdmin(db *gorm.DB, bcryptCost int) {
	var count int64
	db.Model(&User{}).Where("role = ?", "admin").Count(&count)
	if count > 0 {
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte("admin123"), bcryptCost)
	if err != nil {
		log.Fatal("failed to hash admin password: ", err)
	}
	db.Create(&User{Name: "Admin", Email: "admin@app.com", Password: string(hash), Role: "admin"})
	log.Println("admin user seeded: admin@app.com / admin123")
}
