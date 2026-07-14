package grpc

import (
	"context"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/Ippolid/messenger/internal/ratelimit"
)

const sendMessageMethod = "/chat.v1.ChatService/SendMessage"

func rateLimitInterceptor(limiter *ratelimit.Limiter) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		if info.FullMethod == sendMessageMethod {
			userID, ok := UserIDFromContext(ctx)
			if !ok || !limiter.Allow(userID) {
				return nil, status.Error(codes.ResourceExhausted, "слишком часто, подождите")
			}
		}
		return handler(ctx, req)
	}
}
