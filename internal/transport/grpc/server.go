package grpc

import (
	"context"
	"errors"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	chatv1 "github.com/Ippolid/messenger/gen/chat/v1"
	"github.com/Ippolid/messenger/internal/auth"
	"github.com/Ippolid/messenger/internal/chat"
)

// Server реализует chatv1.ChatServiceServer.
// Незавершённые методы наследуются от UnimplementedChatServiceServer
type Server struct {
	chatv1.UnimplementedChatServiceServer
	authSvc *auth.Service
	chatSvc *chat.Service
}

// NewServer собирает gRPC-сервер поверх доменных сервисов.
func NewServer(authSvc *auth.Service, chatSvc *chat.Service) *Server {
	return &Server{authSvc: authSvc, chatSvc: chatSvc}
}

// Register регистрирует нового пользователя.
func (s *Server) Register(ctx context.Context, req *chatv1.RegisterRequest) (*chatv1.RegisterResponse, error) {
	id, err := s.authSvc.Register(ctx, req.GetLogin(), req.GetPassword())
	if err != nil {
		return nil, authError(err)
	}
	return &chatv1.RegisterResponse{UserId: id}, nil
}

// Login проверяет пароль и выдаёт токены.
func (s *Server) Login(ctx context.Context, req *chatv1.LoginRequest) (*chatv1.LoginResponse, error) {
	access, refresh, err := s.authSvc.Login(ctx, req.GetLogin(), req.GetPassword())
	if err != nil {
		return nil, authError(err)
	}
	return &chatv1.LoginResponse{AccessJwt: access, Refresh: refresh}, nil
}

// authError маппит доменные ошибки auth в gRPC-статусы.
func authError(err error) error {
	switch {
	case errors.Is(err, auth.ErrLoginTaken):
		return status.Error(codes.AlreadyExists, "login already taken")
	case errors.Is(err, auth.ErrInvalidCredentials):
		return status.Error(codes.Unauthenticated, "invalid login or password")
	case errors.Is(err, auth.ErrValidation):
		return status.Error(codes.InvalidArgument, err.Error())
	default:
		return status.Error(codes.Internal, "internal error")
	}
}
