// Package ratelimit содержит общий ограничитель частоты запросов транспорта.
package ratelimit

import (
	"sync"

	"golang.org/x/time/rate"
)

const (
	requestsPerSecond = rate.Limit(5)
	burst             = 10
)

// Limiter ограничивает отправку сообщений отдельно для каждого пользователя.
type Limiter struct {
	mu    sync.Mutex
	users map[int64]*rate.Limiter
}

func New() *Limiter {
	return &Limiter{users: make(map[int64]*rate.Limiter)}
}

// Allow сообщает, может ли пользователь отправить сообщение прямо сейчас.
func (l *Limiter) Allow(userID int64) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	limiter := l.users[userID]
	if limiter == nil {
		limiter = rate.NewLimiter(requestsPerSecond, burst)
		l.users[userID] = limiter
	}
	return limiter.Allow()
}
