package chat

import (
	"context"
	"time"

	"connectrpc.com/connect"
	chatv1 "github.com/fraser-isbester/sandbox/chat/gen/chat/v1"
	"github.com/google/uuid"
)

func (h *Handler) SendMessage(ctx context.Context, req *connect.Request[chatv1.SendMessageRequest]) (*connect.Response[chatv1.SendMessageResponse], error) {

	msg := &chatv1.Message{
		Id:             uuid.New().String(),
		ConversationId: req.Msg.ConversationId,
		Content:        req.Msg.Content,
		Role:           chatv1.Role_ROLE_USER,
		Timestamp:      time.Now().UnixNano(),
	}

	// TODO: Get or Create Conversation

	// TODO: Create response

	assistantMsg := &chatv1.Message{
		Id:             uuid.New().String(),
		ConversationId: req.Msg.ConversationId,
		// Content:        resp.Content[0].Text,
		Content:   "test",
		Role:      chatv1.Role_ROLE_ASSISTANT,
		Timestamp: time.Now().UnixNano(),
	}

	return connect.NewResponse(&chatv1.SendMessageResponse{
		Message: assistantMsg,
	}), nil
}
