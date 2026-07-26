package main

import "time"

// User and RefreshToken are the only two tables this service needs — one
// row per account, one row per outstanding refresh token.
type User struct {
	ID        uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	Name      string    `gorm:"not null"                 json:"name"`
	Email     string    `gorm:"uniqueIndex;not null"     json:"email"`
	Password  string    `gorm:"not null"                 json:"-"`
	Role      string    `gorm:"default:'user';not null"  json:"role"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// RefreshToken is stored by hash only — the raw token that goes to the
// client is never written to the database.
type RefreshToken struct {
	ID        uint       `gorm:"primaryKey;autoIncrement" json:"id"`
	UserID    uint       `gorm:"not null;index"           json:"user_id"`
	TokenHash string     `gorm:"not null;uniqueIndex"     json:"-"`
	ExpiresAt time.Time  `gorm:"not null"                 json:"expires_at"`
	RevokedAt *time.Time `json:"-"`
	CreatedAt time.Time  `json:"created_at"`
}
