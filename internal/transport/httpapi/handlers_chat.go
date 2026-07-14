package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/Ippolid/messenger/internal/chat"
	"github.com/Ippolid/messenger/internal/storage"
)

type chatJSON struct {
	ChatID int64  `json:"chat_id"`
	Type   string `json:"type"`
	// Name — title для группы, логин собеседника для лички.
	Name            string `json:"name"`
	LastMessageID   int64  `json:"last_message_id"`
	LastMessageBody string `json:"last_message_body"`
	UnreadCount     int64  `json:"unread_count"`
}

type messageJSON struct {
	ID          int64  `json:"id"`
	ChatID      int64  `json:"chat_id"`
	SenderID    int64  `json:"sender_id"`
	SenderLogin string `json:"sender_login"`
	Body        string `json:"body"`
	At          string `json:"at"` // RFC3339
}

func (s *Server) handleGetChats(w http.ResponseWriter, r *http.Request) {
	items, err := s.chat.GetChats(r.Context(), userID(r.Context()))
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}
	out := make([]chatJSON, 0, len(items))
	for _, it := range items {
		c := chatJSON{
			ChatID:        it.ChatID,
			Type:          it.Type,
			Name:          chatName(it),
			LastMessageID: it.LastMessageID,
			UnreadCount:   it.UnreadCount,
		}
		if it.LastMessageBody != nil {
			c.LastMessageBody = *it.LastMessageBody
		}
		out = append(out, c)
	}
	writeJSON(w, http.StatusOK, out)
}

type createChatRequest struct {
	Type    string   `json:"type"` // "direct" | "group"
	Title   string   `json:"title"`
	Members []string `json:"members"` // логины
}

func (s *Server) handleCreateChat(w http.ResponseWriter, r *http.Request) {
	var req createChatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "bad json")
		return
	}
	id, err := s.chat.CreateChat(r.Context(), userID(r.Context()), req.Type, req.Title, req.Members)
	if err != nil {
		s.writeChatErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]int64{"chat_id": id})
}

func (s *Server) handleHistory(w http.ResponseWriter, r *http.Request) {
	chatID, _ := strconv.ParseInt(r.URL.Query().Get("chat_id"), 10, 64)
	beforeID, _ := strconv.ParseInt(r.URL.Query().Get("before_id"), 10, 64)
	msgs, err := s.chat.GetHistory(r.Context(), userID(r.Context()), chatID, beforeID, 50)
	if err != nil {
		s.writeChatErr(w, err)
		return
	}
	out := make([]messageJSON, 0, len(msgs))
	for _, m := range msgs {
		out = append(out, messageToJSON(m))
	}
	writeJSON(w, http.StatusOK, out)
}

type sendMessageRequest struct {
	ChatID int64  `json:"chat_id"`
	Body   string `json:"body"`
}

func (s *Server) handleSendMessage(w http.ResponseWriter, r *http.Request) {
	var req sendMessageRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "bad json")
		return
	}
	id, err := s.chat.SendMessage(r.Context(), userID(r.Context()), req.ChatID, req.Body)
	if err != nil {
		s.writeChatErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]int64{"message_id": id})
}

type markReadRequest struct {
	ChatID    int64 `json:"chat_id"`
	MessageID int64 `json:"message_id"`
}

func (s *Server) handleMarkRead(w http.ResponseWriter, r *http.Request) {
	var req markReadRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "bad json")
		return
	}
	if err := s.chat.MarkRead(r.Context(), userID(r.Context()), req.ChatID, req.MessageID); err != nil {
		s.writeChatErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

type typingRequest struct {
	ChatID int64 `json:"chat_id"`
}

func (s *Server) handleTyping(w http.ResponseWriter, r *http.Request) {
	var req typingRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "bad json")
		return
	}
	if err := s.chat.SendTyping(r.Context(), userID(r.Context()), req.ChatID); err != nil {
		s.writeChatErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

type addMemberRequest struct {
	ChatID int64  `json:"chat_id"`
	Login  string `json:"login"`
	Role   string `json:"role"`
}

// Добавить участника может только admin.
func (s *Server) handleAddMember(w http.ResponseWriter, r *http.Request) {
	var req addMemberRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "bad json")
		return
	}
	role := req.Role
	if role == "" {
		role = "member"
	}
	if err := s.chat.AddMember(r.Context(), userID(r.Context()), req.ChatID, req.Login, role); err != nil {
		s.writeChatErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

type deleteChatRequest struct {
	ChatID int64 `json:"chat_id"`
}

// Личку удаляет любой участник, группу — только admin.
func (s *Server) handleDeleteChat(w http.ResponseWriter, r *http.Request) {
	var req deleteChatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "bad json")
		return
	}
	if err := s.chat.DeleteChat(r.Context(), userID(r.Context()), req.ChatID); err != nil {
		s.writeChatErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func messageToJSON(m storage.Message) messageJSON {
	return messageJSON{
		ID:          m.ID,
		ChatID:      m.ChatID,
		SenderID:    m.SenderID,
		SenderLogin: m.SenderLogin,
		Body:        m.Body,
		At:          m.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}
}

// chatName: title группы или логин собеседника для лички; иначе «Чат #id».
func chatName(it storage.ChatListItem) string {
	if it.Type == "direct" && it.PeerLogin != nil && *it.PeerLogin != "" {
		return *it.PeerLogin
	}
	if it.Title != nil && *it.Title != "" {
		return *it.Title
	}
	return "Чат #" + strconv.FormatInt(it.ChatID, 10)
}

func (s *Server) writeChatErr(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, chat.ErrPermissionDenied), errors.Is(err, chat.ErrNotMember):
		writeErr(w, http.StatusForbidden, "permission denied")
	case errors.Is(err, chat.ErrUserNotFound):
		writeErr(w, http.StatusNotFound, err.Error())
	case errors.Is(err, chat.ErrValidation):
		writeErr(w, http.StatusBadRequest, err.Error())
	default:
		writeErr(w, http.StatusInternalServerError, "internal error")
	}
}
