package storage

import (
	"context"
	"testing"
)

// TestOutboxRelayCycle проверяет цикл relay: запись видна через FetchUnpublished,
// после MarkPublished исчезает из выборки.
func TestOutboxRelayCycle(t *testing.T) {
	store := setupTestStore(t)
	ctx := context.Background()
	uid, cid := seedUserAndChat(t, store)

	payload := func(msgID int64) ([]byte, error) {
		return []byte(`{"type":"message.new","chat_id":1}`), nil
	}
	if _, err := store.Chats.SaveMessage(ctx, cid, uid, "hi", payload); err != nil {
		t.Fatalf("SaveMessage: %v", err)
	}

	rows, err := store.Chats.FetchUnpublished(ctx, 100)
	if err != nil {
		t.Fatalf("FetchUnpublished: %v", err)
	}
	if len(rows) == 0 {
		t.Fatal("expected at least one unpublished row")
	}

	ids := make([]int64, len(rows))
	for i, r := range rows {
		ids[i] = r.ID
	}
	if err := store.Chats.MarkPublished(ctx, ids); err != nil {
		t.Fatalf("MarkPublished: %v", err)
	}

	after, err := store.Chats.FetchUnpublished(ctx, 100)
	if err != nil {
		t.Fatalf("FetchUnpublished after: %v", err)
	}
	if len(after) != 0 {
		t.Fatalf("expected no unpublished rows after MarkPublished, got %d", len(after))
	}
}
