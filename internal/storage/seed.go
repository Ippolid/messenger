package storage

import (
	"context"
	"fmt"
	"time"
)

type SeedUser struct {
	Login        string
	PasswordHash string
}

type SeedChat struct {
	Type      string
	Title     *string
	CreatedBy string
	Members   []string
	Messages  []SeedMessage
}

type SeedMessage struct {
	SenderLogin string
	Body        string
	CreatedAt   time.Time
}

// SeedDemo заменяет содержимое БД согласованным набором демонстрационных данных.
func (s *Storage) SeedDemo(ctx context.Context, users []SeedUser, chats []SeedChat) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin seed transaction: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck // rollback после commit — no-op

	if _, err := tx.Exec(ctx, "TRUNCATE TABLE users RESTART IDENTITY CASCADE"); err != nil {
		return fmt.Errorf("truncate demo tables: %w", err)
	}

	userIDs := make(map[string]int64, len(users))
	for _, user := range users {
		var id int64
		if err := tx.QueryRow(ctx, `INSERT INTO users (login, password_hash) VALUES ($1, $2) RETURNING id`, user.Login, user.PasswordHash).Scan(&id); err != nil {
			return fmt.Errorf("insert user %q: %w", user.Login, err)
		}
		userIDs[user.Login] = id
	}

	for _, chat := range chats {
		creatorID, ok := userIDs[chat.CreatedBy]
		if !ok {
			return fmt.Errorf("unknown chat creator %q", chat.CreatedBy)
		}
		var chatID int64
		if err := tx.QueryRow(ctx, `INSERT INTO chats (type, title, created_by) VALUES ($1, $2, $3) RETURNING id`, chat.Type, chat.Title, creatorID).Scan(&chatID); err != nil {
			return fmt.Errorf("insert chat: %w", err)
		}
		for _, login := range chat.Members {
			memberID, ok := userIDs[login]
			if !ok {
				return fmt.Errorf("unknown member %q", login)
			}
			role := "member"
			if login == chat.CreatedBy {
				role = "admin"
			}
			if _, err := tx.Exec(ctx, `INSERT INTO chat_members (chat_id, user_id, role) VALUES ($1, $2, $3)`, chatID, memberID, role); err != nil {
				return fmt.Errorf("insert chat member: %w", err)
			}
		}
		for _, message := range chat.Messages {
			senderID, ok := userIDs[message.SenderLogin]
			if !ok {
				return fmt.Errorf("unknown sender %q", message.SenderLogin)
			}
			if _, err := tx.Exec(ctx, `INSERT INTO messages (chat_id, sender_id, body, created_at) VALUES ($1, $2, $3, $4)`, chatID, senderID, message.Body, message.CreatedAt); err != nil {
				return fmt.Errorf("insert message: %w", err)
			}
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit seed transaction: %w", err)
	}
	return nil
}
