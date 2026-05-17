package models

import (
	"time"

	"gorm.io/gorm"
)

type User struct {
	ID        uint           `gorm:"primaryKey;autoIncrement" json:"id"`
	Name      string         `gorm:"not null"                 json:"name"`
	Email     string         `gorm:"uniqueIndex;not null"     json:"email"`
	Password  string         `gorm:"not null"                 json:"-"`
	Role      string         `gorm:"default:'user';not null"  json:"role"`
	CreatedAt time.Time      `                                json:"created_at"`
	UpdatedAt time.Time      `                                json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index"                    json:"-"`
}
