package auth

import (
	"errors"
	"testing"
)

func TestValidateLogin(t *testing.T) {
	tests := []struct {
		name    string
		login   string
		wantErr bool
	}{
		{"valid simple", "alice", false},
		{"valid with digits and underscore", "bob_123", false},
		{"min length 3", "abc", false},
		{"max length 32", "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", false},
		{"too short", "ab", true},
		{"too long 33", "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", true},
		{"uppercase not allowed", "Alice", true},
		{"space not allowed", "al ice", true},
		{"dash not allowed", "al-ice", true},
		{"empty", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateLogin(tt.login)
			if tt.wantErr && err == nil {
				t.Fatalf("expected error for %q, got nil", tt.login)
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("unexpected error for %q: %v", tt.login, err)
			}
			// Ошибки валидации должны оборачивать ErrValidation.
			if tt.wantErr && !errors.Is(err, ErrValidation) {
				t.Fatalf("error %v does not wrap ErrValidation", err)
			}
		})
	}
}

func TestValidatePassword(t *testing.T) {
	tests := []struct {
		name     string
		password string
		wantErr  bool
	}{
		{"exactly 8", "12345678", false},
		{"long", "verylongpassword", false},
		{"too short 7", "1234567", true},
		{"empty", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidatePassword(tt.password)
			if (err != nil) != tt.wantErr {
				t.Fatalf("ValidatePassword(%q) error = %v, wantErr = %v", tt.password, err, tt.wantErr)
			}
		})
	}
}

func TestHashAndCheckPassword(t *testing.T) {
	const password = "password123"

	hash, err := HashPassword(password)
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	// Хэш не должен совпадать с открытым паролем (bcrypt, не plaintext).
	if hash == password {
		t.Fatal("hash equals plaintext password")
	}

	if err := CheckPassword(hash, password); err != nil {
		t.Fatalf("CheckPassword with correct password: %v", err)
	}
	if err := CheckPassword(hash, "wrongpassword"); err == nil {
		t.Fatal("CheckPassword with wrong password: expected error, got nil")
	}
}
