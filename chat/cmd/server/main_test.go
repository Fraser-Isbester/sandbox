package main

import (
	"context"
	"crypto/tls"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"

	chatv1 "github.com/fraser-isbester/sandbox/chat/gen/chat/v1"
	"github.com/fraser-isbester/sandbox/chat/gen/chat/v1/chatv1connect"
)

func setupTestServer(t *testing.T) (*httptest.Server, *connect.BidiStreamForClient[chatv1.ChatRequest, chatv1.ChatResponse]) {
	t.Helper()
	handler := &ChatServiceHandler{}
	mux := http.NewServeMux()
	path, h := chatv1connect.NewChatServiceHandler(handler)
	mux.Handle(path, h)

	server := httptest.NewUnstartedServer(
		h2c.NewHandler(mux, &http2.Server{}),
	)
	server.EnableHTTP2 = true
	server.Start()

	client := &http.Client{
		Transport: &http2.Transport{
			AllowHTTP: true,
			DialTLS: func(network, addr string, cfg *tls.Config) (net.Conn, error) {
				return net.Dial(network, addr)
			},
		},
	}

	streamClient := chatv1connect.NewChatServiceClient(
		client,
		server.URL,
	)

	stream := streamClient.Chat(context.Background())
	return server, stream
}

func TestChat_SingleMessage(t *testing.T) {
	server, stream := setupTestServer(t)
	defer server.Close()

	// Send a single message
	err := stream.Send(&chatv1.ChatRequest{Message: "test"})
	require.NoError(t, err)

	// Receive the response
	resp, err := stream.Receive()
	require.NoError(t, err)
	require.Equal(t, "hello: test", resp.Message)

	// Close the stream
	err = stream.CloseRequest()
	require.NoError(t, err)

	// Should get stream closed error
	_, err = stream.Receive()
	require.Error(t, err)
	require.Equal(t, "unknown: EOF", err.Error())
}

func TestChat_MultipleMessages(t *testing.T) {
	server, stream := setupTestServer(t)
	defer server.Close()

	messages := []string{"hello", "world", "test"}

	// Send all messages
	for _, msg := range messages {
		err := stream.Send(&chatv1.ChatRequest{Message: msg})
		require.NoError(t, err)
	}

	// Close send side
	err := stream.CloseRequest()
	require.NoError(t, err)

	// Receive all responses
	for i := 0; i < len(messages); i++ {
		resp, err := stream.Receive()
		require.NoError(t, err)
		require.Contains(t, resp.Message, "hello: ")
	}

	// Should get stream closed error
	_, err = stream.Receive()
	require.Error(t, err)
	require.Equal(t, "unknown: EOF", err.Error())
}

func TestChat_ContextCancellation(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	handler := &ChatServiceHandler{}
	mux := http.NewServeMux()
	path, h := chatv1connect.NewChatServiceHandler(handler)
	mux.Handle(path, h)

	server := httptest.NewUnstartedServer(
		h2c.NewHandler(mux, &http2.Server{}),
	)
	server.EnableHTTP2 = true
	server.Start()
	defer server.Close()

	client := &http.Client{
		Transport: &http2.Transport{
			AllowHTTP: true,
			DialTLS: func(network, addr string, cfg *tls.Config) (net.Conn, error) {
				return net.Dial(network, addr)
			},
		},
	}

	streamClient := chatv1connect.NewChatServiceClient(
		client,
		server.URL,
	)

	stream := streamClient.Chat(ctx)

	// First message should work
	err := stream.Send(&chatv1.ChatRequest{Message: "test"})
	require.NoError(t, err)

	// Wait for context to expire
	time.Sleep(200 * time.Millisecond)

	// Next message should fail with deadline exceeded
	err = stream.Send(&chatv1.ChatRequest{Message: "test2"})
	require.Error(t, err)
	require.Contains(t, err.Error(), "context deadline exceeded")
}
