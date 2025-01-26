package chat

import chatv1connect "github.com/fraser-isbester/sandbox/chat/gen/chat/v1/chatv1connect"

type Handler struct {
	chatv1connect.UnimplementedChatServiceHandler
}

func NewHandler() *Handler {
	return &Handler{}
}
