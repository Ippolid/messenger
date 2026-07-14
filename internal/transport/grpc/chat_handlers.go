package grpc

import (
	"context"
	"errors"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	chatv1 "github.com/Ippolid/messenger/gen/chat/v1"
	"github.com/Ippolid/messenger/internal/chat"
	"github.com/Ippolid/messenger/internal/storage"
)

func (s *Server) CreateChat(ctx context.Context, req *chatv1.CreateChatRequest) (*chatv1.CreateChatResponse, error) {
	uid, _ := UserIDFromContext(ctx)
	chatID, err := s.chatSvc.CreateChat(ctx, uid, chatTypeToString(req.GetType()), req.GetTitle(), req.GetMemberLogins())
	if err != nil {
		return nil, chatError(err)
	}
	return &chatv1.CreateChatResponse{ChatId: chatID}, nil
}

func (s *Server) GetChats(ctx context.Context, _ *chatv1.GetChatsRequest) (*chatv1.GetChatsResponse, error) {
	uid, _ := UserIDFromContext(ctx)
	items, err := s.chatSvc.GetChats(ctx, uid)
	if err != nil {
		return nil, chatError(err)
	}
	resp := &chatv1.GetChatsResponse{Chats: make([]*chatv1.ChatInfo, 0, len(items))}
	for _, it := range items {
		info := &chatv1.ChatInfo{
			ChatId:        it.ChatID,
			Type:          chatTypeFromString(it.Type),
			LastMessageId: it.LastMessageID,
			UnreadCount:   it.UnreadCount,
		}
		if it.Title != nil {
			info.Title = *it.Title
		}
		if it.LastMessageBody != nil {
			info.LastMessageBody = *it.LastMessageBody
		}
		if it.LastMessageAt != nil {
			info.LastMessageAt = timestamppb.New(*it.LastMessageAt)
		}
		resp.Chats = append(resp.Chats, info)
	}
	return resp, nil
}

func (s *Server) AddMember(ctx context.Context, req *chatv1.AddMemberRequest) (*chatv1.AddMemberResponse, error) {
	uid, _ := UserIDFromContext(ctx)
	if err := s.chatSvc.AddMember(ctx, uid, req.GetChatId(), req.GetLogin(), req.GetRole()); err != nil {
		return nil, chatError(err)
	}
	return &chatv1.AddMemberResponse{}, nil
}

func (s *Server) RemoveMember(ctx context.Context, req *chatv1.RemoveMemberRequest) (*chatv1.RemoveMemberResponse, error) {
	uid, _ := UserIDFromContext(ctx)
	if err := s.chatSvc.RemoveMember(ctx, uid, req.GetChatId(), req.GetLogin()); err != nil {
		return nil, chatError(err)
	}
	return &chatv1.RemoveMemberResponse{}, nil
}

func (s *Server) SendMessage(ctx context.Context, req *chatv1.SendMessageRequest) (*chatv1.SendMessageResponse, error) {
	uid, _ := UserIDFromContext(ctx)
	msgID, err := s.chatSvc.SendMessage(ctx, uid, req.GetChatId(), req.GetBody())
	if err != nil {
		return nil, chatError(err)
	}
	return &chatv1.SendMessageResponse{MessageId: msgID}, nil
}

func (s *Server) GetHistory(ctx context.Context, req *chatv1.GetHistoryRequest) (*chatv1.GetHistoryResponse, error) {
	uid, _ := UserIDFromContext(ctx)
	msgs, err := s.chatSvc.GetHistory(ctx, uid, req.GetChatId(), req.GetBeforeId(), int(req.GetLimit()))
	if err != nil {
		return nil, chatError(err)
	}
	resp := &chatv1.GetHistoryResponse{Messages: make([]*chatv1.Message, 0, len(msgs))}
	for _, m := range msgs {
		resp.Messages = append(resp.Messages, messageToProto(m))
	}
	return resp, nil
}

func (s *Server) MarkRead(ctx context.Context, req *chatv1.MarkReadRequest) (*chatv1.MarkReadResponse, error) {
	uid, _ := UserIDFromContext(ctx)
	if err := s.chatSvc.MarkRead(ctx, uid, req.GetChatId(), req.GetMessageId()); err != nil {
		return nil, chatError(err)
	}
	return &chatv1.MarkReadResponse{}, nil
}

func (s *Server) SendTyping(ctx context.Context, req *chatv1.SendTypingRequest) (*chatv1.SendTypingResponse, error) {
	uid, _ := UserIDFromContext(ctx)
	if err := s.chatSvc.SendTyping(ctx, uid, req.GetChatId()); err != nil {
		return nil, chatError(err)
	}
	return &chatv1.SendTypingResponse{}, nil
}

func messageToProto(m storage.Message) *chatv1.Message {
	return &chatv1.Message{
		Id:        m.ID,
		ChatId:    m.ChatID,
		SenderId:  m.SenderID,
		Body:      m.Body,
		CreatedAt: timestamppb.New(m.CreatedAt),
	}
}

func chatTypeToString(t chatv1.ChatType) string {
	switch t {
	case chatv1.ChatType_CHAT_TYPE_DIRECT:
		return "direct"
	case chatv1.ChatType_CHAT_TYPE_GROUP:
		return "group"
	default:
		return ""
	}
}

func chatTypeFromString(s string) chatv1.ChatType {
	switch s {
	case "direct":
		return chatv1.ChatType_CHAT_TYPE_DIRECT
	case "group":
		return chatv1.ChatType_CHAT_TYPE_GROUP
	default:
		return chatv1.ChatType_CHAT_TYPE_UNSPECIFIED
	}
}

func chatError(err error) error {
	switch {
	case errors.Is(err, chat.ErrPermissionDenied):
		return status.Error(codes.PermissionDenied, "permission denied")
	case errors.Is(err, chat.ErrNotMember):
		return status.Error(codes.PermissionDenied, "not a member of the chat")
	case errors.Is(err, chat.ErrUserNotFound):
		return status.Error(codes.NotFound, err.Error())
	case errors.Is(err, chat.ErrValidation):
		return status.Error(codes.InvalidArgument, err.Error())
	default:
		return status.Error(codes.Internal, "internal error")
	}
}
