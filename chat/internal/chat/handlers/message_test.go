// internal/handlers/chat/message_test.go

package chat

import (
	"context"
	"testing"

	"connectrpc.com/connect"
	chatv1 "github.com/fraser-isbester/sandbox/chat/gen/chat/v1"
	"github.com/stretchr/testify/assert"
)

func TestSendMessage(t *testing.T) {
	h := NewHandler()
	req := connect.NewRequest(&chatv1.SendMessageRequest{
		ConversationId: "test-conv",
		Content:        "test message",
	})

	resp, err := h.SendMessage(context.Background(), req)

	assert.NoError(t, err)
	assert.Equal(t, "Echo: test message", resp.Msg.Message.Content)
	assert.Equal(t, chatv1.Role_ROLE_ASSISTANT, resp.Msg.Message.Role)
}
