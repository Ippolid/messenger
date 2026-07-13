package auth

import (
	"errors"
	"testing"
	"time"
)

func TestGenerateAndParseAccess(t *testing.T) {
	tm := NewTokenManager("test-secret", 15*time.Minute)
	// Выпускаем от текущего времени: ParseAccess проверяет exp против системных часов.
	now := time.Now()

	token, err := tm.GenerateAccess(42, now)
	if err != nil {
		t.Fatalf("GenerateAccess: %v", err)
	}

	userID, err := tm.ParseAccess(token)
	if err != nil {
		t.Fatalf("ParseAccess: %v", err)
	}
	if userID != 42 {
		t.Fatalf("userID = %d, want 42", userID)
	}
}

func TestParseAccess_Expired(t *testing.T) {
	tm := NewTokenManager("test-secret", 15*time.Minute)
	// Токен выпущен в прошлом так, что истёк 30 минут назад.
	past := time.Now().Add(-45 * time.Minute)

	token, err := tm.GenerateAccess(7, past)
	if err != nil {
		t.Fatalf("GenerateAccess: %v", err)
	}

	if _, err := tm.ParseAccess(token); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("expected ErrInvalidToken for expired token, got %v", err)
	}
}

func TestParseAccess_WrongSecret(t *testing.T) {
	issuer := NewTokenManager("secret-a", 15*time.Minute)
	verifier := NewTokenManager("secret-b", 15*time.Minute)

	token, err := issuer.GenerateAccess(1, time.Now())
	if err != nil {
		t.Fatalf("GenerateAccess: %v", err)
	}

	// Токен подписан другим секретом — подпись не пройдёт проверку.
	if _, err := verifier.ParseAccess(token); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("expected ErrInvalidToken for wrong secret, got %v", err)
	}
}

func TestParseAccess_Garbage(t *testing.T) {
	tm := NewTokenManager("test-secret", 15*time.Minute)
	if _, err := tm.ParseAccess("not-a-jwt"); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("expected ErrInvalidToken for garbage, got %v", err)
	}
}
