// internal/handlers/chat/conversation_test.go

package chat

import (
	"context"
	"testing"

	"connectrpc.com/connect"
	chatv1 "github.com/fraser-isbester/sandbox/chat/gen/chat/v1"
	"github.com/stretchr/testify/assert"
)

func TestCreateConversation(t *testing.T) {
	h := NewHandler()
	req := connect.NewRequest(&chatv1.CreateConversationRequest{})

	resp, err := h.CreateConversation(context.Background(), req)

	assert.NoError(t, err)
	assert.NotEmpty(t, resp.Msg.Conversation.Id)
}

func TestGetConversation(t *testing.T) {
	h := NewHandler()
	req := connect.NewRequest(&chatv1.GetConversationRequest{
		Id: "test-conv",
	})

	resp, err := h.GetConversation(context.Background(), req)

	assert.NoError(t, err)
	assert.Equal(t, "test-conv", resp.Msg.Conversation.Id)
}
