package grpc

import (
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	chatv1 "github.com/Ippolid/messenger/gen/chat/v1"
	"github.com/Ippolid/messenger/internal/auth"
)

// New собирает *grpc.Server с auth-интерсепторами и зарегистрированным ChatService.
func New(authSvc *auth.Service, tokens *auth.TokenManager) *grpc.Server {
	ai := &authInterceptor{tokens: tokens}
	srv := grpc.NewServer(
		grpc.ChainUnaryInterceptor(ai.Unary()),
		grpc.ChainStreamInterceptor(ai.Stream()),
	)
	chatv1.RegisterChatServiceServer(srv, NewServer(authSvc))
	// Reflection — чтобы grpcurl и отладочные клиенты видели схему без proto-файла.
	reflection.Register(srv)
	return srv
}
