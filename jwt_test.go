package main

import (
	"testing"
	"time"
)

const testSecret = "test-secret-key-that-is-32-chars!!"

func TestGenerateToken_RoundTrips(t *testing.T) {
	token, err := generateToken(1, "alice@test.com", "user", testSecret, time.Hour)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	claims, err := parseToken(token, testSecret)
	if err != nil {
		t.Fatalf("expected valid claims, got %v", err)
	}
	if claims.UserID != 1 || claims.Role != "user" || claims.Issuer != "go-auth-service" {
		t.Errorf("unexpected claims: %+v", claims)
	}
}

func TestParseToken_Expired(t *testing.T) {
	token, _ := generateToken(1, "alice@test.com", "user", testSecret, -time.Second)
	if _, err := parseToken(token, testSecret); err == nil {
		t.Error("expected error for expired token")
	}
}

func TestParseToken_Malformed(t *testing.T) {
	if _, err := parseToken("this.is.garbage", testSecret); err == nil {
		t.Error("expected error for malformed token")
	}
}

func TestParseToken_WrongSecret(t *testing.T) {
	token, _ := generateToken(1, "alice@test.com", "user", testSecret, time.Hour)
	if _, err := parseToken(token, "a-completely-different-secret-32!"); err == nil {
		t.Error("expected error for token signed with a different secret")
	}
}
