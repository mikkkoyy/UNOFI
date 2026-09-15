package auth

import (
	"testing"
	"time"
)

func TestHashPassword(t *testing.T) {
	hash, err := HashPassword("securepassword123")
	if err != nil {
		t.Fatalf("HashPassword error: %v", err)
	}
	if hash == "" {
		t.Error("hash should not be empty")
	}
	if hash == "securepassword123" {
		t.Error("hash should not equal plaintext")
	}
}

func TestVerifyPassword(t *testing.T) {
	hash, _ := HashPassword("securepassword123")

	if !VerifyPassword("securepassword123", hash) {
		t.Error("correct password should verify")
	}
	if VerifyPassword("wrongpassword", hash) {
		t.Error("wrong password should not verify")
	}
}

func TestVerifyPasswordDifferentHashes(t *testing.T) {
	// Same password should produce different hashes (due to salt)
	hash1, _ := HashPassword("password")
	hash2, _ := HashPassword("password")

	if hash1 == hash2 {
		t.Error("same password should produce different hashes")
	}

	// But both should verify
	if !VerifyPassword("password", hash1) {
		t.Error("hash1 should verify")
	}
	if !VerifyPassword("password", hash2) {
		t.Error("hash2 should verify")
	}
}

func TestSecureCompare(t *testing.T) {
	if !SecureCompare("abc", "abc") {
		t.Error("equal strings should compare true")
	}
	if SecureCompare("abc", "def") {
		t.Error("different strings should compare false")
	}
}

func TestGenerateToken(t *testing.T) {
	token, err := GenerateToken(32)
	if err != nil {
		t.Fatalf("GenerateToken error: %v", err)
	}
	if len(token) != 64 { // 32 bytes = 64 hex chars
		t.Errorf("expected token length 64, got %d", len(token))
	}

	// Different calls should produce different tokens
	token2, _ := GenerateToken(32)
	if token == token2 {
		t.Error("tokens should be unique")
	}
}

func TestSessionStore(t *testing.T) {
	store := NewStore(1 * time.Hour)

	// Create session
	sess, err := store.CreateSession(1, "admin")
	if err != nil {
		t.Fatalf("CreateSession error: %v", err)
	}
	if sess.Token == "" {
		t.Error("token should not be empty")
	}
	if sess.UserID != 1 {
		t.Errorf("expected userID 1, got %d", sess.UserID)
	}

	// Validate session
	valid := store.ValidateSession(sess.Token)
	if valid == nil {
		t.Error("valid session should be returned")
	}
	if valid.Username != "admin" {
		t.Errorf("expected username admin, got %s", valid.Username)
	}

	// Invalid token
	if store.ValidateSession("invalid-token") != nil {
		t.Error("invalid token should return nil")
	}

	// Destroy session
	store.DestroySession(sess.Token)
	if store.ValidateSession(sess.Token) != nil {
		t.Error("destroyed session should return nil")
	}
}

func TestSessionExpiry(t *testing.T) {
	store := NewStore(50 * time.Millisecond)

	sess, _ := store.CreateSession(1, "admin")

	// Session should be valid immediately
	if store.ValidateSession(sess.Token) == nil {
		t.Error("session should be valid before expiry")
	}

	// Wait for expiry
	time.Sleep(100 * time.Millisecond)

	if store.ValidateSession(sess.Token) != nil {
		t.Error("session should be invalid after expiry")
	}
}
