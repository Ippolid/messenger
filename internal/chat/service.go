// Package chat реализует доменную логику чатов, сообщений и realtime-доставки.
package chat

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	chatv1 "github.com/Ippolid/messenger/gen/chat/v1"
	"github.com/Ippolid/messenger/internal/storage"
)

// Доменные ошибки — транспорт маппит их в gRPC-коды.
var (
	ErrValidation       = errors.New("validation")
	ErrPermissionDenied = errors.New("permission denied")
	ErrNotMember        = errors.New("not a member")
	ErrUserNotFound     = errors.New("user not found")
)

type Service struct {
	store *storage.Storage
	hub   *Hub
}

func NewService(store *storage.Storage, hub *Hub) *Service {
	return &Service{store: store, hub: hub}
}

func (s *Service) Hub() *Hub { return s.hub }

// CreateChat создаёт чат по логинам участников.
// direct: ровно 2 участника, без title; group: title обязателен, создатель — admin.
func (s *Service) CreateChat(ctx context.Context, creatorID int64, chatType, title string, memberLogins []string) (int64, error) {
	memberIDs, err := s.resolveLogins(ctx, memberLogins)
	if err != nil {
		return 0, err
	}

	var titlePtr *string
	switch chatType {
	case "direct":
		if len(memberIDs) != 1 {
			return 0, fmt.Errorf("%w: direct chat requires exactly one other member", ErrValidation)
		}
		if title != "" {
			return 0, fmt.Errorf("%w: direct chat must not have a title", ErrValidation)
		}
		peer := memberIDs[0]
		if peer == creatorID {
			return 0, fmt.Errorf("%w: cannot create a direct chat with yourself", ErrValidation)
		}
		// Личка уникальна: если уже есть — возвращаем существующую, не плодим дубли.
		if existing, err := s.store.Chats.FindDirectChat(ctx, creatorID, peer); err != nil {
			return 0, fmt.Errorf("find direct chat: %w", err)
		} else if existing != 0 {
			return existing, nil
		}
	case "group":
		if title == "" {
			return 0, fmt.Errorf("%w: group chat requires a title", ErrValidation)
		}
		titlePtr = &title
	default:
		return 0, fmt.Errorf("%w: unknown chat type %q", ErrValidation, chatType)
	}

	chatID, err := s.store.Chats.CreateChat(ctx, chatType, titlePtr, creatorID, memberIDs)
	if err != nil {
		return 0, err
	}

	// Уведомляем участников, чтобы новый чат появился у них в realtime.
	s.publishChatEvent(memberIDs, chatID, chatv1.ServerEventType_SERVER_EVENT_TYPE_CHAT_CREATED)
	return chatID, nil
}

func (s *Service) GetChats(ctx context.Context, userID int64) ([]storage.ChatListItem, error) {
	return s.store.Chats.GetChats(ctx, userID)
}

// AddMember добавляет участника по логину; только admin чата.
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
	_ = s.store.InsertAudit(ctx, actorID, "add_member", "chat",
		map[string]any{"chat_id": chatID, "login": login, "role": role})
	// Добавленному пользователю чат должен появиться в realtime.
	s.publishChatEvent([]int64{uid}, chatID, chatv1.ServerEventType_SERVER_EVENT_TYPE_CHAT_CREATED)
	return nil
}

// RemoveMember удаляет участника по логину; только admin чата.
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
	_ = s.store.InsertAudit(ctx, actorID, "remove_member", "chat",
		map[string]any{"chat_id": chatID, "login": login})
	return nil
}

// DeleteChat удаляет чат. Для группы — только admin; для лички — любой её участник.
func (s *Service) DeleteChat(ctx context.Context, actorID, chatID int64) error {
	role, err := s.store.Chats.GetRole(ctx, chatID, actorID)
	if err != nil {
		if errors.Is(err, storage.ErrNotMember) {
			return ErrPermissionDenied
		}
		return fmt.Errorf("get role: %w", err)
	}
	// В группе удалить может только admin; в личке — любой участник.
	chatType, err := s.store.Chats.GetType(ctx, chatID)
	if err != nil {
		return fmt.Errorf("get chat type: %w", err)
	}
	if chatType == "group" && role != "admin" {
		return ErrPermissionDenied
	}

	// Список участников получаем ДО удаления — после DeleteChat их уже не достать.
	memberIDs, _ := s.store.Chats.MemberIDs(ctx, chatID)

	if err := s.store.Chats.DeleteChat(ctx, chatID); err != nil {
		return fmt.Errorf("delete chat: %w", err)
	}
	_ = s.store.InsertAudit(ctx, actorID, "delete_chat", "chat", map[string]any{"chat_id": chatID})

	// Уведомляем всех участников, чтобы чат исчез у них в realtime.
	s.publishChatEvent(memberIDs, chatID, chatv1.ServerEventType_SERVER_EVENT_TYPE_CHAT_DELETED)
	return nil
}

// requireAdmin проверяет, что пользователь — admin чата, иначе ErrPermissionDenied.
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

// resolveLogins переводит логины в id, возвращая ErrUserNotFound при отсутствии любого.
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

// outboxEvent — JSON-форма события message.new для outbox.
type outboxEvent struct {
	Type      string `json:"type"`
	MessageID int64  `json:"message_id"`
	ChatID    int64  `json:"chat_id"`
	SenderID  int64  `json:"sender_id"`
	Body      string `json:"body"`
}

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
