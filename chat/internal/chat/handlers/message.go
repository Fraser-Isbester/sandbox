package chat

import (
	"context"

	"connectrpc.com/connect"
	chatv1 "github.com/fraser-isbester/sandbox/chat/gen/chat/v1"
)

// SendMessage handler accepts a message
func (h *Handler) SendMessage(ctx context.Context, req *connect.Request[chatv1.SendMessageRequest]) (*connect.Response[chatv1.SendMessageResponse], error) {

	msg := &chatv1.Message{
		Content: "Echo: " + req.Msg.Content,
		Role:    chatv1.Role_ROLE_ASSISTANT,
	}

	return connect.NewResponse(&chatv1.SendMessageResponse{Message: msg}), nil
}
