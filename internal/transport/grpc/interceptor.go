package grpc

import (
	"context"
	"strings"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"github.com/Ippolid/messenger/internal/auth"
)

// publicMethods — методы, не требующие JWT (полные имена gRPC)
var publicMethods = map[string]bool{
	"/chat.v1.ChatService/Register": true,
	"/chat.v1.ChatService/Login":    true,
}

// authInterceptor проверяет JWT для всех методов кроме whitelisted и кладёт user_id в контекст.
type authInterceptor struct {
	tokens *auth.TokenManager
}

// authenticate извлекает и проверяет токен из metadata, возвращая обогащённый контекст
func (a *authInterceptor) authenticate(ctx context.Context, method string) (context.Context, error) {
	// Служебный сервис reflection (используется grpcurl) не требует авторизации.
	if publicMethods[method] || strings.HasPrefix(method, "/grpc.reflection.") {
		return ctx, nil
	}

	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing metadata")
	}
	vals := md.Get("authorization")
	if len(vals) == 0 {
		return nil, status.Error(codes.Unauthenticated, "missing authorization token")
	}
	// Ожидаем формат "Bearer <jwt>".
	token, ok := strings.CutPrefix(vals[0], "Bearer ")
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "authorization must be 'Bearer <jwt>'")
	}

	userID, err := a.tokens.ParseAccess(strings.TrimSpace(token))
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, "invalid token")
	}
	return withUserID(ctx, userID), nil
}

// Unary возвращает unary-интерсептор аутентификации.
func (a *authInterceptor) Unary() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		ctx, err := a.authenticate(ctx, info.FullMethod)
		if err != nil {
			return nil, err
		}
		return handler(ctx, req)
	}
}

// Stream возвращает stream-интерсептор аутентификации.
func (a *authInterceptor) Stream() grpc.StreamServerInterceptor {
	return func(srv any, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		ctx, err := a.authenticate(ss.Context(), info.FullMethod)
		if err != nil {
			return err
		}
		// Оборачиваем поток, чтобы handler видел обогащённый контекст.
		return handler(srv, &wrappedStream{ServerStream: ss, ctx: ctx})
	}
}

// wrappedStream подменяет Context() у ServerStream на обогащённый.
type wrappedStream struct {
	grpc.ServerStream
	ctx context.Context
}

func (w *wrappedStream) Context() context.Context { return w.ctx }
