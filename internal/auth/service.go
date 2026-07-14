// Package auth реализует регистрацию, вход и проверку JWT.
package auth

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/Ippolid/messenger/internal/storage"
)

// Длительности токенов из ТЗ.
const (
	AccessTTL  = 15 * time.Minute
	refreshTTL = 7 * 24 * time.Hour
)

// Ошибки уровня сервиса (транспорт маппит их в gRPC-коды)
var (
	ErrLoginTaken         = errors.New("login already taken")
	ErrInvalidCredentials = errors.New("invalid login or password")
)

// clock вынесен для подмены времени в тестах.
type clock func() time.Time

type Service struct {
	store  *storage.Storage
	tokens *TokenManager
	now    clock
}

func NewService(store *storage.Storage, tokens *TokenManager) *Service {
	return &Service{store: store, tokens: tokens, now: time.Now}
}

func (s *Service) Register(ctx context.Context, login, password string) (int64, error) {
	if err := ValidateLogin(login); err != nil {
		return 0, err
	}
	if err := ValidatePassword(password); err != nil {
		return 0, err
	}

	hash, err := HashPassword(password)
	if err != nil {
		return 0, err
	}

	id, err := s.store.Users.CreateUser(ctx, login, hash)
	if err != nil {
		if errors.Is(err, storage.ErrUserExists) {
			return 0, ErrLoginTaken
		}
		return 0, fmt.Errorf("create user: %w", err)
	}

	// В details пароль/хэш не кладём.
	if aerr := s.store.InsertAudit(ctx, id, "register", "user", map[string]any{"login": login}); aerr != nil {
		return 0, fmt.Errorf("audit register: %w", aerr)
	}
	return id, nil
}

// Сырой refresh возвращается клиенту, в БД сохраняется только его хэш.
func (s *Service) Login(ctx context.Context, login, password string) (access, refresh string, err error) {
	user, err := s.store.Users.GetByLogin(ctx, login)
	if err != nil {
		if errors.Is(err, storage.ErrUserNotFound) {
			// Не раскрываем, что именно неверно — логин или пароль.
			return "", "", ErrInvalidCredentials
		}
		return "", "", fmt.Errorf("get user: %w", err)
	}

	if err := CheckPassword(user.PasswordHash, password); err != nil {
		return "", "", ErrInvalidCredentials
	}

	now := s.now()
	access, err = s.tokens.GenerateAccess(user.ID, now)
	if err != nil {
		return "", "", err
	}

	refresh, err = generateRefreshToken()
	if err != nil {
		return "", "", err
	}
	rt := storage.RefreshToken{
		ID:        uuid.New(),
		UserID:    user.ID,
		TokenHash: hashRefreshToken(refresh),
		ExpiresAt: now.Add(refreshTTL),
	}
	if err := s.store.Users.SaveRefreshToken(ctx, rt); err != nil {
		return "", "", fmt.Errorf("save refresh: %w", err)
	}

	if aerr := s.store.InsertAudit(ctx, user.ID, "login", "user", map[string]any{"login": login}); aerr != nil {
		return "", "", fmt.Errorf("audit login: %w", aerr)
	}
	return access, refresh, nil
}
