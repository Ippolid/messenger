package storage

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	// ErrChatNotFound — чата нет или пользователь в нём не состоит.
	ErrChatNotFound = errors.New("chat not found")
	ErrNotMember    = errors.New("user is not a member")
)

type Chat struct {
	ID        int64
	Type      string // 'direct' | 'group'
	Title     *string
	CreatedBy int64
	CreatedAt time.Time
}

type ChatListItem struct {
	ChatID          int64
	Type            string
	Title           *string
	PeerLogin       *string // для direct — логин собеседника
	LastMessageID   int64
	LastMessageBody *string
	LastMessageAt   *time.Time
	UnreadCount     int64
}

type Message struct {
	ID          int64
	ChatID      int64
	SenderID    int64
	SenderLogin string
	Body        string
	CreatedAt   time.Time
}

type ChatRepo struct {
	pool *pgxpool.Pool
}

// CreateChat создаёт чат и участников в одной транзакции: creatorID — admin, остальные — member.
func (r *ChatRepo) CreateChat(ctx context.Context, chatType string, title *string, creatorID int64, memberIDs []int64) (int64, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return 0, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck // rollback после commit — no-op

	const insChat = `INSERT INTO chats (type, title, created_by) VALUES ($1, $2, $3) RETURNING id`
	var chatID int64
	if err := tx.QueryRow(ctx, insChat, chatType, title, creatorID).Scan(&chatID); err != nil {
		return 0, fmt.Errorf("insert chat: %w", err)
	}

	const insCreator = `INSERT INTO chat_members (chat_id, user_id, role) VALUES ($1, $2, 'admin')`
	if _, err := tx.Exec(ctx, insCreator, chatID, creatorID); err != nil {
		return 0, fmt.Errorf("insert creator member: %w", err)
	}

	// ON CONFLICT DO NOTHING на случай, если среди memberIDs оказался сам создатель.
	const insMember = `INSERT INTO chat_members (chat_id, user_id, role) VALUES ($1, $2, 'member') ON CONFLICT DO NOTHING`
	for _, uid := range memberIDs {
		if _, err := tx.Exec(ctx, insMember, chatID, uid); err != nil {
			return 0, fmt.Errorf("insert member %d: %w", uid, err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return 0, fmt.Errorf("commit tx: %w", err)
	}
	return chatID, nil
}

// FindDirectChat возвращает id лички между двумя пользователями или 0, если её нет.
func (r *ChatRepo) FindDirectChat(ctx context.Context, userA, userB int64) (int64, error) {
	const q = `
SELECT c.id
FROM chats c
JOIN chat_members m1 ON m1.chat_id = c.id AND m1.user_id = $1
JOIN chat_members m2 ON m2.chat_id = c.id AND m2.user_id = $2
WHERE c.type = 'direct'
LIMIT 1`
	var id int64
	err := r.pool.QueryRow(ctx, q, userA, userB).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("find direct chat: %w", err)
	}
	return id, nil
}

func (r *ChatRepo) GetType(ctx context.Context, chatID int64) (string, error) {
	const q = `SELECT type FROM chats WHERE id = $1`
	var t string
	err := r.pool.QueryRow(ctx, q, chatID).Scan(&t)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrChatNotFound
	}
	if err != nil {
		return "", fmt.Errorf("select chat type: %w", err)
	}
	return t, nil
}

// DeleteChat удаляет чат целиком (сообщения, участники, outbox — через ON DELETE CASCADE).
func (r *ChatRepo) DeleteChat(ctx context.Context, chatID int64) error {
	const q = `DELETE FROM chats WHERE id = $1`
	if _, err := r.pool.Exec(ctx, q, chatID); err != nil {
		return fmt.Errorf("delete chat: %w", err)
	}
	return nil
}

// GetRole возвращает роль пользователя в чате или ErrNotMember.
func (r *ChatRepo) GetRole(ctx context.Context, chatID, userID int64) (string, error) {
	const q = `SELECT role FROM chat_members WHERE chat_id = $1 AND user_id = $2`
	var role string
	err := r.pool.QueryRow(ctx, q, chatID, userID).Scan(&role)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrNotMember
	}
	if err != nil {
		return "", fmt.Errorf("select role: %w", err)
	}
	return role, nil
}

// GetChats возвращает чаты пользователя с последним сообщением и числом непрочитанных.
func (r *ChatRepo) GetChats(ctx context.Context, userID int64) ([]ChatListItem, error) {
	const q = `
WITH my_chats AS (
    -- Изоляция: только чаты, где состоит текущий пользователь.
    SELECT cm.chat_id, cm.last_read_message_id
    FROM chat_members cm
    WHERE cm.user_id = $1
),
last_msg AS (
    -- Последнее сообщение каждого чата через оконную функцию.
    SELECT m.chat_id, m.id, m.body, m.created_at,
           ROW_NUMBER() OVER (PARTITION BY m.chat_id ORDER BY m.id DESC) AS rn
    FROM messages m
    JOIN my_chats mc ON mc.chat_id = m.chat_id
)
SELECT c.id, c.type, c.title,
       COALESCE(lm.id, 0)        AS last_message_id,
       lm.body                   AS last_message_body,
       lm.created_at             AS last_message_at,
       -- Непрочитанные: сообщения с id больше курсора пользователя.
       (SELECT COUNT(*) FROM messages m2
        WHERE m2.chat_id = c.id AND m2.id > mc.last_read_message_id) AS unread_count,
       -- Для direct-чата — логин собеседника (единственный другой участник).
       (SELECT u.login FROM chat_members cm2
        JOIN users u ON u.id = cm2.user_id
        WHERE cm2.chat_id = c.id AND cm2.user_id <> $1
        LIMIT 1)                 AS peer_login
FROM my_chats mc
JOIN chats c ON c.id = mc.chat_id
LEFT JOIN last_msg lm ON lm.chat_id = c.id AND lm.rn = 1
ORDER BY last_message_id DESC`

	rows, err := r.pool.Query(ctx, q, userID)
	if err != nil {
		return nil, fmt.Errorf("query chats: %w", err)
	}
	defer rows.Close()

	var items []ChatListItem
	for rows.Next() {
		var it ChatListItem
		if err := rows.Scan(&it.ChatID, &it.Type, &it.Title,
			&it.LastMessageID, &it.LastMessageBody, &it.LastMessageAt, &it.UnreadCount, &it.PeerLogin); err != nil {
			return nil, fmt.Errorf("scan chat: %w", err)
		}
		items = append(items, it)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate chats: %w", err)
	}
	return items, nil
}

func (r *ChatRepo) AddMember(ctx context.Context, chatID, userID int64, role string) error {
	const q = `INSERT INTO chat_members (chat_id, user_id, role) VALUES ($1, $2, $3) ON CONFLICT DO NOTHING`
	if _, err := r.pool.Exec(ctx, q, chatID, userID, role); err != nil {
		return fmt.Errorf("insert member: %w", err)
	}
	return nil
}

func (r *ChatRepo) RemoveMember(ctx context.Context, chatID, userID int64) error {
	const q = `DELETE FROM chat_members WHERE chat_id = $1 AND user_id = $2`
	if _, err := r.pool.Exec(ctx, q, chatID, userID); err != nil {
		return fmt.Errorf("delete member: %w", err)
	}
	return nil
}

func (r *ChatRepo) MemberIDs(ctx context.Context, chatID int64) ([]int64, error) {
	const q = `SELECT user_id FROM chat_members WHERE chat_id = $1`
	rows, err := r.pool.Query(ctx, q, chatID)
	if err != nil {
		return nil, fmt.Errorf("query members: %w", err)
	}
	defer rows.Close()

	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan member: %w", err)
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}
