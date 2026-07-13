// Package grpc содержит gRPC-транспорт: сервер, интерсепторы, маппинг ошибок.
package grpc

import "context"

// ctxKey — приватный тип ключа контекста, чтобы избежать коллизий.
type ctxKey struct{}

var userIDKey = ctxKey{}

// withUserID кладёт id аутентифицированного пользователя в контекст.
func withUserID(ctx context.Context, userID int64) context.Context {
	return context.WithValue(ctx, userIDKey, userID)
}

// UserIDFromContext возвращает id пользователя, установленный auth-интерсептором.
// Второй результат false, если пользователь не аутентифицирован.
func UserIDFromContext(ctx context.Context) (int64, bool) {
	id, ok := ctx.Value(userIDKey).(int64)
	return id, ok
}
