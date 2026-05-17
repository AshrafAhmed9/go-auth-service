package database

import (
	"log"
	"os"

	"github.com/AshrafAhmed9/assignment-golang/models"
	"github.com/glebarez/sqlite"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

func Connect(bcryptCost int) *gorm.DB {
	if err := os.MkdirAll("data", os.ModePerm); err != nil {
		log.Fatal("failed to create data directory:", err)
	}

	db, err := gorm.Open(sqlite.Open("data/app.db"), &gorm.Config{})
	if err != nil {
		log.Fatal("Failed to connect to database:", err)
	}

	db.AutoMigrate(&models.User{})
	log.Println("database migrated successfully")

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
