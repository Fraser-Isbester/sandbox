package chat

import (
	"context"

	"connectrpc.com/connect"
	chatv1 "github.com/fraser-isbester/sandbox/chat/gen/chat/v1"
)

func (h *Handler) CreateConversation(ctx context.Context, req *connect.Request[chatv1.CreateConversationRequest]) (*connect.Response[chatv1.CreateConversationResponse], error) {

	// Set's the conversation ID
	conv := &chatv1.Conversation{Id: "new-conversation"}

	// TODO: Create conversation

	// Returns the conversation
	return connect.NewResponse(&chatv1.CreateConversationResponse{Conversation: conv}), nil
}

func (h *Handler) GetConversation(ctx context.Context, req *connect.Request[chatv1.GetConversationRequest]) (*connect.Response[chatv1.GetConversationResponse], error) {

	// Fetch conversation
	conv := &chatv1.Conversation{Id: req.Msg.Id}

	// Return fetched conversation
	return connect.NewResponse(&chatv1.GetConversationResponse{Conversation: conv}), nil
}
