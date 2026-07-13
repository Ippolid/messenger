// Хранит pgxpool и репозитории поверх него.
package storage

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Storage владеет пулом подключений к PostgreSQL и репозиториями
type Storage struct {
	pool  *pgxpool.Pool
	Users *UserRepo
}

// New открывает пул подключений по DSN и проверяет доступность БД
func New(ctx context.Context, dsn string) (*Storage, error) {
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, fmt.Errorf("create pgx pool: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping db: %w", err)
	}
	return &Storage{
		pool:  pool,
		Users: &UserRepo{pool: pool},
	}, nil
}

// Close закрывает пул подключений
func (s *Storage) Close() {
	s.pool.Close()
}
