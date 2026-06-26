package database

import (
	"log"
	"os"

	"github.com/AshrafAhmed9/assignment-golang/models"
	"github.com/glebarez/sqlite"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func Connect(driver, databaseURL string, bcryptCost int) *gorm.DB {
	var db *gorm.DB
	var err error

	switch driver {
	case "postgres":
		if databaseURL == "" {
			log.Fatal("DATABASE_URL is required when DB_DRIVER is postgres")
		}
		db, err = gorm.Open(postgres.Open(databaseURL), &gorm.Config{})
		if err != nil {
			log.Fatal("failed to connect to postgres:", err)
		}
		log.Println("connected to postgres")

	default:
		if err := os.MkdirAll("data", os.ModePerm); err != nil {
			log.Fatal("failed to create data directory:", err)
		}
		db, err = gorm.Open(sqlite.Open("data/app.db"), &gorm.Config{})
		if err != nil {
			log.Fatal("failed to connect to sqlite:", err)
		}
		db.AutoMigrate(&models.User{}, &models.RefreshToken{}, &models.AuditEvent{})
		log.Println("connected to sqlite, schema auto-migrated")
	}

	seedAdmin(db, bcryptCost)
	return db
}

func seedAdmin(db *gorm.DB, bcryptCost int) {
	var count int64
	db.Model(&models.User{}).Where("role = ?", "admin").Count(&count)
	if count > 0 {
		return
	}

	hash, _ := bcrypt.GenerateFromPassword([]byte("admin123"), bcryptCost)
	db.Create(&models.User{
		Name:     "Admin",
		Email:    "admin@app.com",
		Password: string(hash),
		Role:     "admin",
	})
	log.Println("admin user seeded")
}
