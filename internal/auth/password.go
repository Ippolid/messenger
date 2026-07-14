package auth

import (
	"errors"
	"fmt"
	"regexp"

	"golang.org/x/crypto/bcrypt"
)

// ErrValidation оборачивает ошибки валидации входных данных
var ErrValidation = errors.New("validation")

// loginRe — допустимый логин: 3–32 символа из [a-z0-9_]
var loginRe = regexp.MustCompile(`^[a-z0-9_]{3,32}$`)

const minPasswordLen = 8

func ValidateLogin(login string) error {
	if !loginRe.MatchString(login) {
		return fmt.Errorf("%w: login must be 3-32 chars of [a-z0-9_]", ErrValidation)
	}
	return nil
}

func ValidatePassword(password string) error {
	if len(password) < minPasswordLen {
		return fmt.Errorf("%w: password must be at least %d characters", ErrValidation, minPasswordLen)
	}
	return nil
}

// bcrypt: адаптивный хэш с солью — правильный выбор для паролей.
func HashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", fmt.Errorf("hash password: %w", err)
	}
	return string(hash), nil
}

func CheckPassword(hash, password string) error {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
}
