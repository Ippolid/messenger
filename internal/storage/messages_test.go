package storage

import (
	"context"
	"os"
	"testing"
)

// testDSN возвращает DSN тестовой БД. Если TEST_DB_DSN не задан, тест пропускается —
// так `go test ./...` не падает на машинах без поднятого Postgres.
func testDSN(t *testing.T) string {
	t.Helper()
	dsn := os.Getenv("TEST_DB_DSN")
	if dsn == "" {
		t.Skip("TEST_DB_DSN not set — skipping integration test")
	}
	return dsn
}

// setupTestStore открывает Storage и регистрирует очистку тестовых данных.
func setupTestStore(t *testing.T) *Storage {
	t.Helper()
	ctx := context.Background()
	store, err := New(ctx, testDSN(t))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(store.Close)
	return store
}

// seedUserAndChat создаёт пользователя и group-чат, возвращает их id.
func seedUserAndChat(t *testing.T, store *Storage) (userID, chatID int64) {
	t.Helper()
	ctx := context.Background()

	// Уникальный логин, чтобы прогоны не конфликтовали.
	login := "test_" + randSuffix(t)
	uid, err := store.Users.CreateUser(ctx, login, "hash")
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	title := "test chat"
	cid, err := store.Chats.CreateChat(ctx, "group", &title, uid, nil)
	if err != nil {
		t.Fatalf("create chat: %v", err)
	}
	return uid, cid
}

// TestSaveMessage_TransactionMessageAndOutbox проверяет ключевой инвариант ТЗ:
// SaveMessage в одной транзакции создаёт и messages, и outbox-запись.
func TestSaveMessage_TransactionMessageAndOutbox(t *testing.T) {
	store := setupTestStore(t)
	ctx := context.Background()
	uid, cid := seedUserAndChat(t, store)

	payload := func(msgID int64) ([]byte, error) {
		return []byte(`{"type":"message.new"}`), nil
	}
	msg, err := store.Chats.SaveMessage(ctx, cid, uid, "hello", payload)
	if err != nil {
		t.Fatalf("SaveMessage: %v", err)
	}
	if msg.ID == 0 {
		t.Fatal("expected non-zero message id")
	}

	// Проверяем: сообщение есть.
	var bodyInDB string
	if err := store.pool.QueryRow(ctx, `SELECT body FROM messages WHERE id=$1`, msg.ID).Scan(&bodyInDB); err != nil {
		t.Fatalf("select message: %v", err)
	}
	if bodyInDB != "hello" {
		t.Fatalf("message body = %q, want hello", bodyInDB)
	}

	// Проверяем: outbox-запись с этим message_id есть и не опубликована.
	var published *string
	if err := store.pool.QueryRow(ctx,
		`SELECT published_at::text FROM outbox WHERE message_id=$1`, msg.ID).Scan(&published); err != nil {
		t.Fatalf("select outbox: %v", err)
	}
	if published != nil {
		t.Fatalf("expected published_at to be NULL, got %v", *published)
	}
}

// TestSaveMessage_RollbackOnPayloadError проверяет атомарность:
// если формирование payload падает, сообщение НЕ должно сохраниться.
func TestSaveMessage_RollbackOnPayloadError(t *testing.T) {
	store := setupTestStore(t)
	ctx := context.Background()
	uid, cid := seedUserAndChat(t, store)

	var countBefore int
	if err := store.pool.QueryRow(ctx, `SELECT COUNT(*) FROM messages WHERE chat_id=$1`, cid).Scan(&countBefore); err != nil {
		t.Fatalf("count before: %v", err)
	}

	failingPayload := func(msgID int64) ([]byte, error) {
		return nil, errPayload
	}
	if _, err := store.Chats.SaveMessage(ctx, cid, uid, "should rollback", failingPayload); err == nil {
		t.Fatal("expected error from failing payload, got nil")
	}

	var countAfter int
	if err := store.pool.QueryRow(ctx, `SELECT COUNT(*) FROM messages WHERE chat_id=$1`, cid).Scan(&countAfter); err != nil {
		t.Fatalf("count after: %v", err)
	}
	if countAfter != countBefore {
		t.Fatalf("message count changed after rollback: before=%d after=%d", countBefore, countAfter)
	}
}
