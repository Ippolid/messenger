package chat

import (
	"context"
	"errors"
	"fmt"
	"time"

	chatv1 "github.com/Ippolid/messenger/gen/chat/v1"
	"github.com/Ippolid/messenger/internal/storage"
)

// SendMessage сохраняет сообщение (транзакция message+outbox) и возвращает его id.
// Писать могут admin/member, но не reader.
func (s *Service) SendMessage(ctx context.Context, senderID, chatID int64, body string) (int64, error) {
	if body == "" {
		return 0, fmt.Errorf("%w: message body must not be empty", ErrValidation)
	}

	role, err := s.store.Chats.GetRole(ctx, chatID, senderID)
	if err != nil {
		if errors.Is(err, storage.ErrNotMember) {
			return 0, ErrPermissionDenied // не участник — писать нельзя (изоляция)
		}
		return 0, fmt.Errorf("get role: %w", err)
	}
	if role == "reader" {
		return 0, ErrPermissionDenied // reader — только чтение
	}

	// Логин нужен уже в payload (fanout не ходит в БД) — резолвим до сохранения.
	senderLogin, _ := s.store.Users.GetLoginByID(ctx, senderID)

	payload := newMessagePayload(chatID, senderID, senderLogin, body, time.Now())
	msg, err := s.store.Chats.SaveMessage(ctx, chatID, senderID, body, payload)
	if err != nil {
		return 0, fmt.Errorf("save message: %w", err)
	}

	_ = s.store.InsertAudit(ctx, senderID, "send_message", "message",
		map[string]any{"chat_id": chatID, "message_id": msg.ID})

	// Больше не пушим в hub напрямую: событие едет через outbox → Redis → fanout.
	return msg.ID, nil
}

// GetHistory возвращает историю чата keyset-пагинацией с проверкой изоляции.
func (s *Service) GetHistory(ctx context.Context, userID, chatID, beforeID int64, limit int) ([]storage.Message, error) {
	// Изоляция: пользователь должен состоять в чате.
	if _, err := s.store.Chats.GetRole(ctx, chatID, userID); err != nil {
		if errors.Is(err, storage.ErrNotMember) {
			return nil, ErrPermissionDenied
		}
		return nil, fmt.Errorf("get role: %w", err)
	}
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	return s.store.Chats.GetHistory(ctx, chatID, beforeID, limit)
}

// MarkRead сдвигает read-курсор и рассылает событие message.read участникам.
func (s *Service) MarkRead(ctx context.Context, userID, chatID, messageID int64) error {
	// Изоляция.
	if _, err := s.store.Chats.GetRole(ctx, chatID, userID); err != nil {
		if errors.Is(err, storage.ErrNotMember) {
			return ErrPermissionDenied
		}
		return fmt.Errorf("get role: %w", err)
	}
	if err := s.store.Chats.MarkRead(ctx, chatID, userID, messageID); err != nil {
		return fmt.Errorf("mark read: %w", err)
	}

	_ = s.store.InsertAudit(ctx, userID, "mark_read", "chat",
		map[string]any{"chat_id": chatID, "message_id": messageID})

	// message.read едет тем же конвейером через outbox (message_id = NULL).
	if payload, perr := readEventPayload(chatID, userID, messageID); perr == nil {
		_ = s.store.Chats.InsertOutboxEvent(ctx, payload)
	}
	return nil
}

// SendTyping рассылает эфемерное событие «печатает» участникам чата.
// Мимо outbox — терять такое событие не жалко.
func (s *Service) SendTyping(ctx context.Context, userID, chatID int64) error {
	// Изоляция.
	if _, err := s.store.Chats.GetRole(ctx, chatID, userID); err != nil {
		if errors.Is(err, storage.ErrNotMember) {
			return ErrPermissionDenied
		}
		return fmt.Errorf("get role: %w", err)
	}
	ev := &chatv1.ServerEvent{
		Type: chatv1.ServerEventType_SERVER_EVENT_TYPE_TYPING,
		Payload: &chatv1.ServerEvent_Typing{Typing: &chatv1.TypingEvent{
			ChatId: chatID,
			UserId: userID,
		}},
	}
	s.publishToChat(ctx, chatID, ev)
	return nil
}

func (s *Service) publishToChat(ctx context.Context, chatID int64, ev *chatv1.ServerEvent) {
	ids, err := s.store.Chats.MemberIDs(ctx, chatID)
	if err != nil {
		return // realtime best-effort: ошибка получения участников не роняет запрос
	}
	s.hub.Publish(ids, ev)
}

// publishChatEvent рассылает событие изменения списка чатов заранее известному списку
// участников — при удалении чата их уже не получить из БД.
func (s *Service) publishChatEvent(memberIDs []int64, chatID int64, evType chatv1.ServerEventType) {
	ev := &chatv1.ServerEvent{
		Type:    evType,
		Payload: &chatv1.ServerEvent_Chat{Chat: &chatv1.ChatEvent{ChatId: chatID}},
	}
	s.hub.Publish(memberIDs, ev)
}
