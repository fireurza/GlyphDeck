// Package auth provides first-run admin setup, password-based login,
// cookie-based sessions, and HTTP middleware for GlyphDeck.
package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"fmt"

	"golang.org/x/crypto/bcrypt"
)

// Sentinel errors.
var (
	ErrAdminExists     = errors.New("admin already exists")
	ErrNoAdmin         = errors.New("no admin configured")
	ErrInvalidPassword = errors.New("invalid password")
	ErrSessionNotFound = errors.New("session not found")
	ErrSessionExpired  = errors.New("session expired")
)

// AdminHash is the bcrypt-hashed admin password.
type AdminHash []byte

// HashPassword returns a bcrypt hash of the given password.
func HashPassword(password string) (AdminHash, error) {
	if len(password) < 8 {
		return nil, fmt.Errorf("password must be at least 8 characters")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("hash password: %w", err)
	}
	return hash, nil
}

// VerifyPassword compares a password against a bcrypt hash.
func VerifyPassword(hash AdminHash, password string) bool {
	if len(hash) == 0 {
		return false
	}
	err := bcrypt.CompareHashAndPassword(hash, []byte(password))
	return err == nil
}

// SessionToken is a random 32-byte hex-encoded string.
func NewSessionToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate session token: %w", err)
	}
	return hex.EncodeToString(b), nil
}

// Constant-time comparison for session tokens.
func TokensEqual(a, b string) bool {
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}

// AuthStatus reports whether setup or login is required.
type AuthStatus struct {
	SetupRequired bool   `json:"setupRequired"`
	LoginRequired bool   `json:"loginRequired"`
	AdminExists   bool   `json:"adminExists"`
	Username      string `json:"username,omitempty"`
}

// LoginRequest is the body for POST /api/auth/login.
type LoginRequest struct {
	Password string `json:"password"`
}

// SetupRequest is the body for POST /api/auth/setup.
type SetupRequest struct {
	Password string `json:"password"`
}
