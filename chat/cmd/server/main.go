package main

import (
	"log"
	"net/http"

	"github.com/rs/cors"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"

	chatv1connect "github.com/fraser-isbester/sandbox/chat/gen/chat/v1/chatv1connect"
	chat "github.com/fraser-isbester/sandbox/chat/internal/chat/handlers"
)

func main() {
	mux := http.NewServeMux()
	chatService := chat.NewHandler()
	path, handler := chatv1connect.NewChatServiceHandler(chatService)
	mux.Handle(path, handler)

	corsHandler := cors.New(cors.Options{
		AllowedMethods: []string{http.MethodGet, http.MethodPost, http.MethodPatch,
			http.MethodPut, http.MethodDelete, http.MethodOptions},
		AllowedHeaders: []string{"Accept", "Content-Type", "Connect-Protocol-Version"},
	})

	server := &http.Server{
		Addr:    ":50051",
		Handler: h2c.NewHandler(corsHandler.Handler(mux), &http2.Server{}),
	}

	log.Printf("server listening on %s", server.Addr)
	log.Fatal(server.ListenAndServe())
}
