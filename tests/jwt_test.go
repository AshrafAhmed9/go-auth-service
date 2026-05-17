package tests

import (
	"testing"
	"time"

	"github.com/AshrafAhmed9/assignment-golang/utils"
)

func TestGenerateToken_Valid(t *testing.T) {
	secret := "test-secret-key-that-is-32-chars!!"
	token, err := utils.GenerateToken(1, "alice@test.com", "user", secret, 24*time.Hour)

	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if token == "" {
		t.Error("expected token, got empty string")
	}

	claims, err := utils.ParseToken(token, secret)
	if err != nil {
		t.Errorf("expected valid claims, got %v", err)
	}
	if claims.UserID != 1 {
		t.Errorf("expected userID 1, got %d", claims.UserID)
	}
	if claims.Role != "user" {
		t.Errorf("expected role 'user', got %s", claims.Role)
	}
	if claims.Issuer != "go-auth-service" {
		t.Errorf("expected issuer 'go-auth-service', got %s", claims.Issuer)
	}
}

func TestParseToken_Expired(t *testing.T) {
	secret := "test-secret-key-that-is-32-chars!!"
	token, _ := utils.GenerateToken(1, "alice@test.com", "user", secret, -1*time.Second)

	_, err := utils.ParseToken(token, secret)
	if err == nil {
		t.Error("expected error for expired token, got nil")
	}
}

func TestParseToken_Malformed(t *testing.T) {
	secret := "test-secret-key-that-is-32-chars!!"
	_, err := utils.ParseToken("this.is.garbage", secret)
	if err == nil {
		t.Error("expected error for malformed token, got nil")
	}
}
