package models

import "time"

type RefreshToken struct {
	ID        uint       `gorm:"primaryKey;autoIncrement" json:"id"`
	UserID    uint       `gorm:"not null;index"           json:"user_id"`
	TokenHash string     `gorm:"not null;uniqueIndex"     json:"-"`
	ExpiresAt time.Time  `gorm:"not null"                 json:"expires_at"`
	RevokedAt *time.Time `                                json:"-"`
	CreatedAt time.Time  `                                json:"created_at"`
}
