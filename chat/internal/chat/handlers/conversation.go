package chat

import (
	"context"
	"fmt"

	"connectrpc.com/connect"
	chatv1 "github.com/fraser-isbester/sandbox/chat/gen/chat/v1"
	"github.com/google/uuid"
)

func (h *Handler) GetConversation(ctx context.Context, req *connect.Request[chatv1.GetConversationRequest]) (*connect.Response[chatv1.GetConversationResponse], error) {
	conv, err := h.store.Load(req.Msg.Id)
	if err != nil {
		return nil, fmt.Errorf("loading conversation: %w", err)
	}
	return connect.NewResponse(&chatv1.GetConversationResponse{
		Conversation: conv,
	}), nil
}

func (h *Handler) CreateConversation(ctx context.Context, req *connect.Request[chatv1.CreateConversationRequest]) (*connect.Response[chatv1.CreateConversationResponse], error) {
	conv := &chatv1.Conversation{
		Id: uuid.New().String(),
	}
	return connect.NewResponse(&chatv1.CreateConversationResponse{
		Conversation: conv,
	}), nil
}
