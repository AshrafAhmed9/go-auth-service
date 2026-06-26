package models

import "time"

type AuditEvent struct {
	ID        uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	EventType string    `gorm:"not null;index"           json:"event_type"`
	UserID    *uint     `gorm:"index"                    json:"user_id"`
	Email     string    `gorm:"not null"                 json:"email"`
	IP        string    `gorm:"not null"                 json:"ip"`
	RequestID string    `gorm:"not null"                 json:"request_id"`
	Metadata  string    `gorm:"type:text"                json:"metadata"`
	CreatedAt time.Time `                              json:"created_at"`
}
