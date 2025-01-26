package main

import (
	"context"
	"io"
	"log"
	"net/http"

	"connectrpc.com/connect"
	"github.com/rs/cors"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"

	chatv1 "github.com/fraser-isbester/sandbox/chat/gen/chat/v1"
	"github.com/fraser-isbester/sandbox/chat/gen/chat/v1/chatv1connect"
)

// ChatServiceHandler implements the ChatService interface
type ChatServiceHandler struct {
	chatv1connect.UnimplementedChatServiceHandler
}

func (s *ChatServiceHandler) Chat(
	ctx context.Context,
	stream *connect.BidiStream[chatv1.ChatRequest, chatv1.ChatResponse],
) error {
	for {
		req, err := stream.Receive()
		if err == io.EOF {
			log.Println("client closed stream")
			return nil
		}
		if err != nil {
			log.Printf("receive error: %v", err)
			return err
		}

		log.Printf("received message: %s", req.Message)
		resp := &chatv1.ChatResponse{
			Message: "hello: " + req.Message,
		}

		if err := stream.Send(resp); err != nil {
			log.Printf("send error: %v", err)
			return err
		}
		log.Printf("sent response: %s", resp.Message)
	}
}

func main() {
	mux := http.NewServeMux()

	// Create a new ChatServiceHandler
	chatService := &ChatServiceHandler{}

	// Register the service
	path, handler := chatv1connect.NewChatServiceHandler(chatService)
	mux.Handle(path, handler)

	corsHandler := cors.New(cors.Options{
		AllowedMethods: []string{
			http.MethodGet,
			http.MethodPost,
			http.MethodPatch,
			http.MethodPut,
			http.MethodDelete,
			http.MethodOptions,
		},
		AllowedHeaders: []string{
			"Accept",
			"Content-Type",
			"Connect-Protocol-Version",
		},
	})

	server := &http.Server{
		Addr: ":50051",
		Handler: h2c.NewHandler(
			corsHandler.Handler(mux),
			&http2.Server{},
		),
	}

	log.Printf("server listening on %s", server.Addr)
	log.Fatal(server.ListenAndServe())
}
