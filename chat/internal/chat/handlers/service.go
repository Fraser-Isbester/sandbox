package chat

import (
	"github.com/anthropics/anthropic-sdk-go"
	"github.com/fraser-isbester/sandbox/chat/internal/storage"
)

type Handler struct {
	client *anthropic.Client
	store  *storage.ConversationStore
}

func NewHandler() *Handler {
	client := anthropic.NewClient()
	return &Handler{
		client: client,
		store:  storage.NewConversationStore("./data"),
	}
}
