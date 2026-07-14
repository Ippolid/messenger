package grpc

import (
	chatv1 "github.com/Ippolid/messenger/gen/chat/v1"
)

// Subscribe держит стрим до отмены контекста и пушит события из hub.
func (s *Server) Subscribe(_ *chatv1.SubscribeRequest, stream chatv1.ChatService_SubscribeServer) error {
	uid, _ := UserIDFromContext(stream.Context())

	events, unsub := s.chatSvc.Hub().Subscribe(uid)
	defer unsub()

	ctx := stream.Context()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case ev, ok := <-events:
			if !ok {
				return nil // канал закрыт (отписка)
			}
			if err := stream.Send(ev); err != nil {
				return err
			}
		}
	}
}
