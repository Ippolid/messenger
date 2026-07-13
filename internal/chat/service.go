// Package chat реализует доменную логику чатов, сообщений и realtime-доставки.
package chat

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/Ippolid/messenger/internal/storage"
)

// Доменные ошибки (транспорт маппит их в gRPC-коды)
var (
	ErrValidation       = errors.New("validation")
	ErrPermissionDenied = errors.New("permission denied")
	ErrNotMember        = errors.New("not a member")
	ErrUserNotFound     = errors.New("user not found")
)

// Service — доменная логика чатов поверх хранилища и in-memory хаба
type Service struct {
	store *storage.Storage
	hub   *Hub
}

// NewService создаёт сервис чатов
func NewService(store *storage.Storage, hub *Hub) *Service {
	return &Service{store: store, hub: hub}
}

// Hub возвращает хаб (транспорт использует его для Subscribe-стримов)
func (s *Service) Hub() *Hub { return s.hub }

// CreateChat создаёт чат по логинам участников
// direct: ровно 2 участника (создатель + 1), без title
// group: title обязателен, создатель становится admin
func (s *Service) CreateChat(ctx context.Context, creatorID int64, chatType, title string, memberLogins []string) (int64, error) {
	// Резолвим логины участников в id.
	memberIDs, err := s.resolveLogins(ctx, memberLogins)
	if err != nil {
		return 0, err
	}

	var titlePtr *string
	switch chatType {
	case "direct":
		// Ровно один «другой» участник, без title
		if len(memberIDs) != 1 {
			return 0, fmt.Errorf("%w: direct chat requires exactly one other member", ErrValidation)
		}
		if title != "" {
			return 0, fmt.Errorf("%w: direct chat must not have a title", ErrValidation)
		}
	case "group":
		if title == "" {
			return 0, fmt.Errorf("%w: group chat requires a title", ErrValidation)
		}
		titlePtr = &title
	default:
		return 0, fmt.Errorf("%w: unknown chat type %q", ErrValidation, chatType)
	}

	return s.store.Chats.CreateChat(ctx, chatType, titlePtr, creatorID, memberIDs)
}

// GetChats возвращает чаты пользователя (изоляция обеспечена SQL)
func (s *Service) GetChats(ctx context.Context, userID int64) ([]storage.ChatListItem, error) {
	return s.store.Chats.GetChats(ctx, userID)
}

// AddMember добавляет участника по логину. Только admin чата (иначе ErrPermissionDenied)
func (s *Service) AddMember(ctx context.Context, actorID, chatID int64, login, role string) error {
	if err := s.requireAdmin(ctx, chatID, actorID); err != nil {
		return err
	}
	if role != "member" && role != "reader" {
		return fmt.Errorf("%w: role must be 'member' or 'reader'", ErrValidation)
	}
	uid, err := s.store.Users.GetIDByLogin(ctx, login)
	if err != nil {
		if errors.Is(err, storage.ErrUserNotFound) {
			return ErrUserNotFound
		}
		return fmt.Errorf("resolve login: %w", err)
	}
	if err := s.store.Chats.AddMember(ctx, chatID, uid, role); err != nil {
		return fmt.Errorf("add member: %w", err)
	}
	// Аудит: add_member.
	_ = s.store.InsertAudit(ctx, actorID, "add_member", "chat",
		map[string]any{"chat_id": chatID, "login": login, "role": role})
	return nil
}

// RemoveMember удаляет участника по логину. Только admin чата
func (s *Service) RemoveMember(ctx context.Context, actorID, chatID int64, login string) error {
	if err := s.requireAdmin(ctx, chatID, actorID); err != nil {
		return err
	}
	uid, err := s.store.Users.GetIDByLogin(ctx, login)
	if err != nil {
		if errors.Is(err, storage.ErrUserNotFound) {
			return ErrUserNotFound
		}
		return fmt.Errorf("resolve login: %w", err)
	}
	if err := s.store.Chats.RemoveMember(ctx, chatID, uid); err != nil {
		return fmt.Errorf("remove member: %w", err)
	}
	// Аудит: remove_member.
	_ = s.store.InsertAudit(ctx, actorID, "remove_member", "chat",
		map[string]any{"chat_id": chatID, "login": login})
	return nil
}

// requireAdmin проверяет, что пользователь — admin чата. Иначе ErrPermissionDenied
// (или ErrNotMember, если он вообще не участник — тоже недостаточно прав).
func (s *Service) requireAdmin(ctx context.Context, chatID, userID int64) error {
	role, err := s.store.Chats.GetRole(ctx, chatID, userID)
	if err != nil {
		if errors.Is(err, storage.ErrNotMember) {
			return ErrPermissionDenied
		}
		return fmt.Errorf("get role: %w", err)
	}
	if role != "admin" {
		return ErrPermissionDenied
	}
	return nil
}

// resolveLogins переводит логины в id, возвращая ErrUserNotFound при отсутствии любого
func (s *Service) resolveLogins(ctx context.Context, logins []string) ([]int64, error) {
	ids := make([]int64, 0, len(logins))
	for _, login := range logins {
		id, err := s.store.Users.GetIDByLogin(ctx, login)
		if err != nil {
			if errors.Is(err, storage.ErrUserNotFound) {
				return nil, fmt.Errorf("%w: %s", ErrUserNotFound, login)
			}
			return nil, fmt.Errorf("resolve login %s: %w", login, err)
		}
		ids = append(ids, id)
	}
	return ids, nil
}

// outboxEvent — форма payload для outbox (JSON события message.new)
type outboxEvent struct {
	Type      string `json:"type"`
	MessageID int64  `json:"message_id"`
	ChatID    int64  `json:"chat_id"`
	SenderID  int64  `json:"sender_id"`
	Body      string `json:"body"`
}

// marshalMessagePayload формирует JSON события message.new для outbox
func marshalMessagePayload(chatID, senderID int64, body string) func(int64) ([]byte, error) {
	return func(msgID int64) ([]byte, error) {
		return json.Marshal(outboxEvent{
			Type:      "message.new",
			MessageID: msgID,
			ChatID:    chatID,
			SenderID:  senderID,
			Body:      body,
		})
	}
}
