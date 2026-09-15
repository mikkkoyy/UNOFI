package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"sync"
	"time"

	"golang.org/x/crypto/bcrypt"
)

// Session represents an authenticated admin session.
type Session struct {
	Token     string    `json:"token"`
	UserID    int64     `json:"user_id"`
	Username  string    `json:"username"`
	ExpiresAt time.Time `json:"expires_at"`
	CreatedAt time.Time `json:"created_at"`
}

// Store manages admin authentication sessions.
type Store struct {
	mu       sync.RWMutex
	sessions map[string]*Session
	ttl      time.Duration
}

// NewStore creates a new session store with the specified TTL.
func NewStore(ttl time.Duration) *Store {
	s := &Store{
		sessions: make(map[string]*Session),
		ttl:      ttl,
	}
	go s.cleanupLoop()
	return s
}

// cleanupLoop periodically removes expired sessions.
func (s *Store) cleanupLoop() {
	ticker := time.NewTicker(5 * time.Minute)
	for range ticker.C {
		s.mu.Lock()
		now := time.Now()
		for tok, sess := range s.sessions {
			if now.After(sess.ExpiresAt) {
				delete(s.sessions, tok)
			}
		}
		s.mu.Unlock()
	}
}

// CreateSession generates a new session for a user.
func (s *Store) CreateSession(userID int64, username string) (*Session, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return nil, fmt.Errorf("generate token: %w", err)
	}
	token := hex.EncodeToString(b)

	sess := &Session{
		Token:     token,
		UserID:    userID,
		Username:  username,
		ExpiresAt: time.Now().Add(s.ttl),
		CreatedAt: time.Now(),
	}

	s.mu.Lock()
	s.sessions[token] = sess
	s.mu.Unlock()

	return sess, nil
}

// ValidateSession checks if a session token is valid.
func (s *Store) ValidateSession(token string) *Session {
	s.mu.RLock()
	defer s.mu.RUnlock()

	sess, ok := s.sessions[token]
	if !ok {
		return nil
	}
	if time.Now().After(sess.ExpiresAt) {
		return nil
	}
	return sess
}

// TTL returns the session time-to-live.
func (s *Store) TTL() time.Duration {
	return s.ttl
}

// DestroySession removes a session.
func (s *Store) DestroySession(token string) {
	s.mu.Lock()
	delete(s.sessions, token)
	s.mu.Unlock()
}

// HashPassword creates a bcrypt hash of a password.
func HashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", fmt.Errorf("hash password: %w", err)
	}
	return string(hash), nil
}

// VerifyPassword checks a password against a bcrypt hash.
func VerifyPassword(password, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}

// SecureCompare performs constant-time comparison of two strings.
func SecureCompare(a, b string) bool {
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}

// GenerateToken creates a random hex token.
func GenerateToken(length int) (string, error) {
	b := make([]byte, length)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate token: %w", err)
	}
	return hex.EncodeToString(b), nil
}
