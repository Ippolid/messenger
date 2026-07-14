package httpapi

import (
	"encoding/json"
	"fmt"
	"net/http"

	chatv1 "github.com/Ippolid/messenger/gen/chat/v1"
)

type eventJSON struct {
	Type              string `json:"type"` // message.new | message.read | typing
	ChatID            int64  `json:"chat_id"`
	MessageID         int64  `json:"message_id,omitempty"`
	SenderID          int64  `json:"sender_id,omitempty"`
	SenderLogin       string `json:"sender_login,omitempty"`
	Body              string `json:"body,omitempty"`
	UserID            int64  `json:"user_id,omitempty"`
	LastReadMessageID int64  `json:"last_read_message_id,omitempty"`
	At                string `json:"at,omitempty"`
}

// handleEvents — SSE-стрим: каждое событие hub отдаётся как "data: <json>\n\n".
func (s *Server) handleEvents(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeErr(w, http.StatusInternalServerError, "streaming unsupported")
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	events, unsub := s.chat.Hub().Subscribe(userID(r.Context()))
	defer unsub()

	ctx := r.Context()
	for {
		select {
		case <-ctx.Done():
			return // клиент отключился
		case ev, ok := <-events:
			if !ok {
				return
			}
			data, err := json.Marshal(toEventJSON(ev))
			if err != nil {
				continue
			}
			// Ошибку записи игнорируем: при обрыве клиента следующий цикл
			// поймает ctx.Done() и завершит стрим.
			_, _ = fmt.Fprintf(w, "data: %s\n\n", data)
			flusher.Flush()
		}
	}
}

func toEventJSON(ev *chatv1.ServerEvent) eventJSON {
	switch ev.GetType() {
	case chatv1.ServerEventType_SERVER_EVENT_TYPE_MESSAGE_NEW:
		m := ev.GetMessage()
		return eventJSON{
			Type:        "message.new",
			ChatID:      m.GetChatId(),
			MessageID:   m.GetId(),
			SenderID:    m.GetSenderId(),
			SenderLogin: m.GetSenderLogin(),
			Body:        m.GetBody(),
			At:          m.GetCreatedAt().AsTime().Format("2006-01-02T15:04:05Z07:00"),
		}
	case chatv1.ServerEventType_SERVER_EVENT_TYPE_MESSAGE_READ:
		rd := ev.GetRead()
		return eventJSON{
			Type:              "message.read",
			ChatID:            rd.GetChatId(),
			UserID:            rd.GetUserId(),
			LastReadMessageID: rd.GetLastReadMessageId(),
		}
	case chatv1.ServerEventType_SERVER_EVENT_TYPE_TYPING:
		tp := ev.GetTyping()
		return eventJSON{
			Type:   "typing",
			ChatID: tp.GetChatId(),
			UserID: tp.GetUserId(),
		}
	case chatv1.ServerEventType_SERVER_EVENT_TYPE_CHAT_CREATED:
		return eventJSON{Type: "chat.created", ChatID: ev.GetChat().GetChatId()}
	case chatv1.ServerEventType_SERVER_EVENT_TYPE_CHAT_DELETED:
		return eventJSON{Type: "chat.deleted", ChatID: ev.GetChat().GetChatId()}
	default:
		return eventJSON{Type: "unknown"}
	}
}
