package handlers

import (
	"github.com/AshrafAhmed9/assignment-golang/models"
	"gorm.io/gorm"
)

const (
	EventSignup       = "signup"
	EventLoginSuccess = "login_success"
	EventLoginFailure = "login_failure"
	EventLoginLocked  = "login_locked"
	EventTokenRefresh = "token_refresh"
	EventLogout       = "logout"
)

func writeAuditEvent(db *gorm.DB, eventType string, userID *uint, email, ip, requestID, metadata string) {
	db.Create(&models.AuditEvent{
		EventType: eventType,
		UserID:    userID,
		Email:     email,
		IP:        ip,
		RequestID: requestID,
		Metadata:  metadata,
	})
}
