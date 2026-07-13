package storage

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"testing"
)

// errPayload — искусственная ошибка для проверки отката транзакции.
var errPayload = errors.New("payload build failed")

// randSuffix возвращает случайный hex-суффикс для уникальных логинов в тестах.
func randSuffix(t *testing.T) string {
	t.Helper()
	b := make([]byte, 6)
	if _, err := rand.Read(b); err != nil {
		t.Fatalf("rand: %v", err)
	}
	return hex.EncodeToString(b)
}
