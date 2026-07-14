package grpc

import (
	"context"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/Ippolid/messenger/internal/ratelimit"
)

func TestRateLimitInterceptorRejectsBurstOverflow(t *testing.T) {
	t.Parallel()

	interceptor := rateLimitInterceptor(ratelimit.New())
	info := &grpc.UnaryServerInfo{FullMethod: sendMessageMethod}
	ctx := withUserID(context.Background(), 42)
	handler := func(context.Context, any) (any, error) { return nil, nil }

	for range 10 {
		if _, err := interceptor(ctx, nil, info, handler); err != nil {
			t.Fatalf("request inside burst: %v", err)
		}
	}
	_, err := interceptor(ctx, nil, info, handler)
	if status.Code(err) != codes.ResourceExhausted {
		t.Fatalf("code = %s, want %s", status.Code(err), codes.ResourceExhausted)
	}
}
