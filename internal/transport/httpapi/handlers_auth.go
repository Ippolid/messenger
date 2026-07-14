package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/Ippolid/messenger/internal/auth"
)

// credsRequest — тело запросов login/register.
type credsRequest struct {
	Login    string `json:"login"`
	Password string `json:"password"`
}

func (s *Server) handleRegister(w http.ResponseWriter, r *http.Request) {
	var req credsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "bad json")
		return
	}
	id, err := s.auth.Register(r.Context(), req.Login, req.Password)
	if err != nil {
		if errors.Is(err, auth.ErrLoginTaken) {
			writeErr(w, http.StatusConflict, "login already taken")
			return
		}
		if errors.Is(err, auth.ErrValidation) {
			writeErr(w, http.StatusBadRequest, err.Error())
			return
		}
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]int64{"user_id": id})
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	var req credsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "bad json")
		return
	}
	access, refresh, err := s.auth.Login(r.Context(), req.Login, req.Password)
	if err != nil {
		if errors.Is(err, auth.ErrInvalidCredentials) {
			writeErr(w, http.StatusUnauthorized, "invalid login or password")
			return
		}
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}
	// user_id раскодируем для клиента, чтобы он отличал свои сообщения.
	uid, _ := s.tokens.ParseAccess(access)
	writeJSON(w, http.StatusOK, map[string]any{
		"access":  access,
		"refresh": refresh,
		"user_id": uid,
	})
}
