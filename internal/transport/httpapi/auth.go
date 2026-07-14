package httpapi

import (
	"context"
	"net/http"
	"strings"
)

type ctxKey struct{}

var userIDKey = ctxKey{}

// authMiddleware проверяет JWT и кладёт user_id в контекст запроса.
// Токен берётся из заголовка "Authorization: Bearer <jwt>" или, для SSE
// (EventSource не умеет слать заголовки), из query-параметра ?token=<jwt>.
func (s *Server) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := bearerToken(r)
		if token == "" {
			token = r.URL.Query().Get("token")
		}
		if token == "" {
			writeErr(w, http.StatusUnauthorized, "missing token")
			return
		}
		userID, err := s.tokens.ParseAccess(token)
		if err != nil {
			writeErr(w, http.StatusUnauthorized, "invalid token")
			return
		}
		ctx := context.WithValue(r.Context(), userIDKey, userID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (s *Server) rateLimitMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !s.limiter.Allow(userID(r.Context())) {
			writeErr(w, http.StatusTooManyRequests, "слишком часто, подождите")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func bearerToken(r *http.Request) string {
	h := r.Header.Get("Authorization")
	if token, ok := strings.CutPrefix(h, "Bearer "); ok {
		return strings.TrimSpace(token)
	}
	return ""
}

func userID(ctx context.Context) int64 {
	id, _ := ctx.Value(userIDKey).(int64)
	return id
}
