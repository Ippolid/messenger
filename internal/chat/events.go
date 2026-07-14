package chat

import (
	"encoding/json"
	"time"

	"google.golang.org/protobuf/types/known/timestamppb"

	chatv1 "github.com/Ippolid/messenger/gen/chat/v1"
)

// outboxEvent — единый JSON-формат события в outbox (message.new и message.read).
// Едет через Redis Streams; fanout восстанавливает из него ServerEvent.
type outboxEvent struct {
	Type              string `json:"type"` // "message.new" | "message.read"
	ChatID            int64  `json:"chat_id"`
	MessageID         int64  `json:"message_id,omitempty"`
	SenderID          int64  `json:"sender_id,omitempty"`
	SenderLogin       string `json:"sender_login,omitempty"`
	Body              string `json:"body,omitempty"`
	CreatedAt         string `json:"created_at,omitempty"` // RFC3339
	UserID            int64  `json:"user_id,omitempty"`
	LastReadMessageID int64  `json:"last_read_message_id,omitempty"`
}

// newMessagePayload формирует payload события message.new для транзакции SaveMessage.
// Логин и время подставляются после вставки (msgID уже известен).
func newMessagePayload(chatID, senderID int64, senderLogin, body string, createdAt time.Time) func(int64) ([]byte, error) {
	return func(msgID int64) ([]byte, error) {
		return json.Marshal(outboxEvent{
			Type:        "message.new",
			ChatID:      chatID,
			MessageID:   msgID,
			SenderID:    senderID,
			SenderLogin: senderLogin,
			Body:        body,
			CreatedAt:   createdAt.Format(time.RFC3339Nano),
		})
	}
}

func readEventPayload(chatID, userID, lastReadID int64) ([]byte, error) {
	return json.Marshal(outboxEvent{
		Type:              "message.read",
		ChatID:            chatID,
		UserID:            userID,
		LastReadMessageID: lastReadID,
	})
}

// serverEventFromPayload восстанавливает ServerEvent из JSON outbox-записи.
func serverEventFromPayload(payload []byte) (*chatv1.ServerEvent, error) {
	var e outboxEvent
	if err := json.Unmarshal(payload, &e); err != nil {
		return nil, err
	}
	switch e.Type {
	case "message.new":
		ts := timestamppb.Now()
		if t, err := time.Parse(time.RFC3339Nano, e.CreatedAt); err == nil {
			ts = timestamppb.New(t)
		}
		return &chatv1.ServerEvent{
			Type: chatv1.ServerEventType_SERVER_EVENT_TYPE_MESSAGE_NEW,
			Payload: &chatv1.ServerEvent_Message{Message: &chatv1.Message{
				Id:          e.MessageID,
				ChatId:      e.ChatID,
				SenderId:    e.SenderID,
				SenderLogin: e.SenderLogin,
				Body:        e.Body,
				CreatedAt:   ts,
			}},
		}, nil
	case "message.read":
		return &chatv1.ServerEvent{
			Type: chatv1.ServerEventType_SERVER_EVENT_TYPE_MESSAGE_READ,
			Payload: &chatv1.ServerEvent_Read{Read: &chatv1.MessageReadEvent{
				ChatId:            e.ChatID,
				UserId:            e.UserID,
				LastReadMessageId: e.LastReadMessageID,
			}},
		}, nil
	default:
		return nil, nil
	}
}

// chatIDFromPayload достаёт chat_id, чтобы fanout знал, каким участникам слать.
func chatIDFromPayload(payload []byte) (int64, error) {
	var e outboxEvent
	if err := json.Unmarshal(payload, &e); err != nil {
		return 0, err
	}
	return e.ChatID, nil
}
