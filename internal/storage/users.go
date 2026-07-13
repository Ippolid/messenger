package storage

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ErrUserExists возвращается, когда логин уже занят (нарушение UNIQUE users.login)
var ErrUserExists = errors.New("user already exists")

// ErrUserNotFound возвращается, когда пользователь не найден
var ErrUserNotFound = errors.New("user not found")

// User — строка таблицы users.
type User struct {
	ID           int64
	Login        string
	PasswordHash string
	CreatedAt    time.Time
}

// RefreshToken — строка таблицы refresh_tokens
type RefreshToken struct {
	ID        uuid.UUID
	UserID    int64
	TokenHash string
	ExpiresAt time.Time
	RevokedAt *time.Time
}

// UserRepo инкапсулирует SQL по пользователям и refresh-токенам.
type UserRepo struct {
	pool *pgxpool.Pool
}

// CreateUser вставляет пользователя и возвращает его id.
// При занятом логине возвращает ErrUserExists.
func (r *UserRepo) CreateUser(ctx context.Context, login, passwordHash string) (int64, error) {
	const q = `INSERT INTO users (login, password_hash) VALUES ($1, $2) RETURNING id`
	var id int64
	if err := r.pool.QueryRow(ctx, q, login, passwordHash).Scan(&id); err != nil {
		// 23505 — unique_violation: логин уже занят.
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return 0, ErrUserExists
		}
		return 0, fmt.Errorf("insert user: %w", err)
	}
	return id, nil
}

// GetByLogin возвращает пользователя по логину или ErrUserNotFound.
func (r *UserRepo) GetByLogin(ctx context.Context, login string) (User, error) {
	const q = `SELECT id, login, password_hash, created_at FROM users WHERE login = $1`
	var u User
	err := r.pool.QueryRow(ctx, q, login).Scan(&u.ID, &u.Login, &u.PasswordHash, &u.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return User{}, ErrUserNotFound
	}
	if err != nil {
		return User{}, fmt.Errorf("select user by login: %w", err)
	}
	return u, nil
}

// SaveRefreshToken сохраняет хэш refresh-токена.
func (r *UserRepo) SaveRefreshToken(ctx context.Context, t RefreshToken) error {
	const q = `INSERT INTO refresh_tokens (id, user_id, token_hash, expires_at) VALUES ($1, $2, $3, $4)`
	if _, err := r.pool.Exec(ctx, q, t.ID, t.UserID, t.TokenHash, t.ExpiresAt); err != nil {
		return fmt.Errorf("insert refresh token: %w", err)
	}
	return nil
}
