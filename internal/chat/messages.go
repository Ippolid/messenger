package chat

import (
	"context"
	"errors"
	"fmt"

	"google.golang.org/protobuf/types/known/timestamppb"

	chatv1 "github.com/Ippolid/messenger/gen/chat/v1"
	"github.com/Ippolid/messenger/internal/storage"
)

// SendMessage сохраняет сообщение (транзакция message+outbox) и возвращает его id
// Отправитель должен быть участником с правом писать (admin/member, не reader)
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

	// Транзакция: message + outbox. Ack клиенту — после успешного коммита
	msg, err := s.store.Chats.SaveMessage(ctx, chatID, senderID, body, marshalMessagePayload(chatID, senderID, body))
	if err != nil {
		return 0, fmt.Errorf("save message: %w", err)
	}

	// Аудит: send_message
	_ = s.store.InsertAudit(ctx, senderID, "send_message", "message",
		map[string]any{"chat_id": chatID, "message_id": msg.ID})

	s.fanoutNewMessage(ctx, msg)
	return msg.ID, nil
}

// GetHistory возвращает историю чата keyset-пагинацией с проверкой изоляции
func (s *Service) GetHistory(ctx context.Context, userID, chatID, beforeID int64, limit int) ([]storage.Message, error) {
	// Изоляция: пользователь должен состоять в чате.
	if _, err := s.store.Chats.GetRole(ctx, chatID, userID); err != nil {
		if errors.Is(err, storage.ErrNotMember) {
			return nil, ErrPermissionDenied
		}
		return nil, fmt.Errorf("get role: %w", err)
	}
	if limit <= 0 || limit > 100 {
		limit = 50 // разумный дефолт/потолок
	}
	return s.store.Chats.GetHistory(ctx, chatID, beforeID, limit)
}

// MarkRead сдвигает read-курсор и рассылает событие message.read участникам
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

	// Аудит: mark_read
	_ = s.store.InsertAudit(ctx, userID, "mark_read", "chat",
		map[string]any{"chat_id": chatID, "message_id": messageID})

	// Событие message.read участникам
	ev := &chatv1.ServerEvent{
		Type: chatv1.ServerEventType_SERVER_EVENT_TYPE_MESSAGE_READ,
		Payload: &chatv1.ServerEvent_Read{Read: &chatv1.MessageReadEvent{
			ChatId:            chatID,
			UserId:            userID,
			LastReadMessageId: messageID,
		}},
	}
	s.publishToChat(ctx, chatID, ev)
	return nil
}

// SendTyping рассылает эфемерное событие «печатает» участникам чата
// Мимо outbox — терять такое событие не жалко
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

// fanoutNewMessage рассылает событие message.new участникам чата
func (s *Service) fanoutNewMessage(ctx context.Context, msg storage.Message) {
	ev := &chatv1.ServerEvent{
		Type: chatv1.ServerEventType_SERVER_EVENT_TYPE_MESSAGE_NEW,
		Payload: &chatv1.ServerEvent_Message{Message: &chatv1.Message{
			Id:        msg.ID,
			ChatId:    msg.ChatID,
			SenderId:  msg.SenderID,
			Body:      msg.Body,
			CreatedAt: timestamppb.New(msg.CreatedAt),
		}},
	}
	s.publishToChat(ctx, msg.ChatID, ev)
}

// publishToChat получает участников чата и доставляет им событие через hub
func (s *Service) publishToChat(ctx context.Context, chatID int64, ev *chatv1.ServerEvent) {
	ids, err := s.store.Chats.MemberIDs(ctx, chatID)
	if err != nil {
		return // realtime best-effort: ошибка получения участников не роняет запрос
	}
	s.hub.Publish(ids, ev)
}
