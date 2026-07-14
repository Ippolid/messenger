package storage

import (
	"context"
	"fmt"
)

// OutboxRow — неопубликованная запись outbox.
type OutboxRow struct {
	ID      int64
	Payload []byte
}

// FetchUnpublished возвращает до limit неопубликованных записей по возрастанию id.
// Использует частичный индекс WHERE published_at IS NULL.
func (r *ChatRepo) FetchUnpublished(ctx context.Context, limit int) ([]OutboxRow, error) {
	const q = `
SELECT id, payload FROM outbox
WHERE published_at IS NULL
ORDER BY id
LIMIT $1`
	rows, err := r.pool.Query(ctx, q, limit)
	if err != nil {
		return nil, fmt.Errorf("query outbox: %w", err)
	}
	defer rows.Close()

	var out []OutboxRow
	for rows.Next() {
		var row OutboxRow
		if err := rows.Scan(&row.ID, &row.Payload); err != nil {
			return nil, fmt.Errorf("scan outbox: %w", err)
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

// MarkPublished проставляет published_at для переданных id.
func (r *ChatRepo) MarkPublished(ctx context.Context, ids []int64) error {
	if len(ids) == 0 {
		return nil
	}
	const q = `UPDATE outbox SET published_at = now() WHERE id = ANY($1)`
	if _, err := r.pool.Exec(ctx, q, ids); err != nil {
		return fmt.Errorf("mark published: %w", err)
	}
	return nil
}

// InsertOutboxEvent пишет эфемерное событие без привязки к сообщению (message_id = NULL).
// Используется для message.read, который едет тем же конвейером.
func (r *ChatRepo) InsertOutboxEvent(ctx context.Context, payload []byte) error {
	const q = `INSERT INTO outbox (message_id, payload) VALUES (NULL, $1)`
	if _, err := r.pool.Exec(ctx, q, payload); err != nil {
		return fmt.Errorf("insert outbox event: %w", err)
	}
	return nil
}
