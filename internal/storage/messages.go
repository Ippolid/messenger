package storage

import (
	"context"
	"fmt"
)

// SaveMessage сохраняет сообщение и запись в outbox в ОДНОЙ транзакции.
func (r *ChatRepo) SaveMessage(ctx context.Context, chatID, senderID int64, body string, outboxPayload func(msgID int64) ([]byte, error)) (Message, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return Message{}, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck // rollback после commit — no-op

	const insMsg = `INSERT INTO messages (chat_id, sender_id, body) VALUES ($1, $2, $3) RETURNING id, created_at`
	var m Message
	m.ChatID = chatID
	m.SenderID = senderID
	m.Body = body
	if err := tx.QueryRow(ctx, insMsg, chatID, senderID, body).Scan(&m.ID, &m.CreatedAt); err != nil {
		return Message{}, fmt.Errorf("insert message: %w", err)
	}

	// payload формируем после вставки — нужен уже известный m.ID.
	payload, err := outboxPayload(m.ID)
	if err != nil {
		return Message{}, fmt.Errorf("build outbox payload: %w", err)
	}
	const insOutbox = `INSERT INTO outbox (message_id, payload) VALUES ($1, $2)`
	if _, err := tx.Exec(ctx, insOutbox, m.ID, payload); err != nil {
		return Message{}, fmt.Errorf("insert outbox: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return Message{}, fmt.Errorf("commit tx: %w", err)
	}
	return m, nil
}

// GetHistory возвращает историю keyset-пагинацией (WHERE id < beforeID), НЕ OFFSET.
// beforeID == 0 означает «с конца», без верхней границы.
func (r *ChatRepo) GetHistory(ctx context.Context, chatID int64, beforeID int64, limit int) ([]Message, error) {
	const q = `
SELECT m.id, m.chat_id, m.sender_id, u.login, m.body, m.created_at
FROM messages m
JOIN users u ON u.id = m.sender_id
WHERE m.chat_id = $1 AND ($2 = 0 OR m.id < $2)
ORDER BY m.id DESC
LIMIT $3`
	rows, err := r.pool.Query(ctx, q, chatID, beforeID, limit)
	if err != nil {
		return nil, fmt.Errorf("query history: %w", err)
	}
	defer rows.Close()

	var msgs []Message
	for rows.Next() {
		var m Message
		if err := rows.Scan(&m.ID, &m.ChatID, &m.SenderID, &m.SenderLogin, &m.Body, &m.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan message: %w", err)
		}
		msgs = append(msgs, m)
	}
	return msgs, rows.Err()
}

// SearchMessages выполняет полнотекстовый поиск по сообщениям одного чата.
func (r *ChatRepo) SearchMessages(ctx context.Context, chatID int64, query string, limit int) ([]Message, error) {
	const q = `
SELECT m.id, m.chat_id, m.sender_id, u.login, m.body, m.created_at
FROM messages m
JOIN users u ON u.id = m.sender_id
WHERE m.chat_id = $1 AND m.body_tsv @@ plainto_tsquery('russian', $2)
ORDER BY m.id DESC
LIMIT $3`
	rows, err := r.pool.Query(ctx, q, chatID, query, limit)
	if err != nil {
		return nil, fmt.Errorf("search messages: %w", err)
	}
	defer rows.Close()

	var msgs []Message
	for rows.Next() {
		var m Message
		if err := rows.Scan(&m.ID, &m.ChatID, &m.SenderID, &m.SenderLogin, &m.Body, &m.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan message: %w", err)
		}
		msgs = append(msgs, m)
	}
	return msgs, rows.Err()
}

// MarkRead сдвигает read-курсор пользователя в чате.
// Курсор только растёт: GREATEST не даёт откатить его назад.
func (r *ChatRepo) MarkRead(ctx context.Context, chatID, userID, messageID int64) error {
	const q = `
UPDATE chat_members
SET last_read_message_id = GREATEST(last_read_message_id, $3)
WHERE chat_id = $1 AND user_id = $2`
	if _, err := r.pool.Exec(ctx, q, chatID, userID, messageID); err != nil {
		return fmt.Errorf("update read cursor: %w", err)
	}
	return nil
}
