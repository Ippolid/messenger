// Package httpapi — HTTP/JSON-шлюз для веб-клиента, realtime через SSE.
package httpapi

import (
	"encoding/json"
	"net/http"

	"github.com/Ippolid/messenger/internal/auth"
	"github.com/Ippolid/messenger/internal/chat"
	"github.com/Ippolid/messenger/internal/ratelimit"
)

type Server struct {
	auth    *auth.Service
	chat    *chat.Service
	tokens  *auth.TokenManager
	mux     *http.ServeMux
	limiter *ratelimit.Limiter
}

func NewServer(authSvc *auth.Service, chatSvc *chat.Service, tokens *auth.TokenManager, limiter *ratelimit.Limiter) *Server {
	s := &Server{auth: authSvc, chat: chatSvc, tokens: tokens, mux: http.NewServeMux(), limiter: limiter}
	s.routes()
	return s
}

func (s *Server) Handler() http.Handler { return s.mux }

func (s *Server) routes() {
	s.mux.HandleFunc("GET /", s.handleIndex)

	// Публичные.
	s.mux.HandleFunc("POST /api/register", s.handleRegister)
	s.mux.HandleFunc("POST /api/login", s.handleLogin)

	// Защищённые JWT.
	s.mux.Handle("GET /api/chats", s.authMiddleware(http.HandlerFunc(s.handleGetChats)))
	s.mux.Handle("POST /api/chats", s.authMiddleware(http.HandlerFunc(s.handleCreateChat)))
	s.mux.Handle("GET /api/history", s.authMiddleware(http.HandlerFunc(s.handleHistory)))
	s.mux.Handle("POST /api/messages", s.authMiddleware(s.rateLimitMiddleware(http.HandlerFunc(s.handleSendMessage))))
	s.mux.Handle("GET /api/search", s.authMiddleware(http.HandlerFunc(s.handleSearch)))
	s.mux.Handle("POST /api/read", s.authMiddleware(http.HandlerFunc(s.handleMarkRead)))
	s.mux.Handle("POST /api/typing", s.authMiddleware(http.HandlerFunc(s.handleTyping)))
	s.mux.Handle("GET /api/events", s.authMiddleware(http.HandlerFunc(s.handleEvents)))
	s.mux.Handle("POST /api/members", s.authMiddleware(http.HandlerFunc(s.handleAddMember)))
	s.mux.Handle("POST /api/chats/delete", s.authMiddleware(http.HandlerFunc(s.handleDeleteChat)))
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
