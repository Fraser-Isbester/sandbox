package main

import (
	"fmt"
	"log"
	"net/http"
	"time"
)

func sseHandler(w http.ResponseWriter, r *http.Request) {

	// Set headers for SSE
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	// Create channel for client disconnect detection
	notify := r.Context().Done()
	go func() {
		<-notify
		log.Println("Client disconnected")
	}()

	// Send events every 2 seconds
	messageChan := make(chan string)
	go func() {
		id := 1
		for {
			// Simulate some event data
			message := fmt.Sprintf("id: %d\ndata: Server time is %s\n\n",
				id,
				time.Now().Format(time.RFC3339))
			messageChan <- message
			id++
			time.Sleep(2 * time.Second)
		}
	}()

	// Write events to response
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "SSE not supported", http.StatusInternalServerError)
		return
	}

	for {
		select {
		case <-notify:
			return
		case msg := <-messageChan:
			fmt.Fprint(w, msg)
			flusher.Flush()
		}
	}
}

func main() {
	// Serve the HTML file
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "index.html")
	})

	// SSE endpoint
	http.HandleFunc("/events", sseHandler)

	fmt.Println("Server starting on :8080...")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatal(err)
	}
}
