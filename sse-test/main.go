package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"
)

type Client struct {
	ID        string
	Channel   chan []byte
	LastSeen  time.Time
	UserAgent string
}

type SSEServer struct {
	clients sync.Map
	stats   struct {
		connections int
		messages    int
	}
}

func NewSSEServer() *SSEServer {
	server := &SSEServer{}

	// Cleanup routine - remove stale connections
	go func() {
		for {
			now := time.Now()
			server.clients.Range(func(key, value interface{}) bool {
				client := value.(*Client)
				if now.Sub(client.LastSeen) > 30*time.Second {
					server.clients.Delete(key)
					close(client.Channel)
				}
				return true
			})
			time.Sleep(10 * time.Second)
		}
	}()

	return server
}

func (s *SSEServer) handleSSE(w http.ResponseWriter, r *http.Request) {
	// SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	// Create client
	client := &Client{
		ID:        r.RemoteAddr,
		Channel:   make(chan []byte, 10),
		LastSeen:  time.Now(),
		UserAgent: r.UserAgent(),
	}
	s.clients.Store(client.ID, client)

	// Cleanup on disconnect
	notify := r.Context().Done()
	go func() {
		<-notify
		s.clients.Delete(client.ID)
		close(client.Channel)
	}()

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "SSE not supported", http.StatusInternalServerError)
		return
	}

	// Send initial retry timeout (15 seconds)
	fmt.Fprintf(w, "retry: 15000\n\n")
	flusher.Flush()

	// Keep-alive ticker
	keepAliveTicker := time.NewTicker(15 * time.Second)
	defer keepAliveTicker.Stop()

	for {
		select {
		case <-notify:
			return
		case msg := <-client.Channel:
			fmt.Fprintf(w, "data: %s\n\n", msg)
			flusher.Flush()
			client.LastSeen = time.Now()
		case <-keepAliveTicker.C:
			// Send comment as keepalive
			fmt.Fprintf(w, ": keepalive %v\n\n", time.Now().Unix())
			flusher.Flush()
		}
	}
}

func (s *SSEServer) broadcast(eventType string, data interface{}) {
	payload, _ := json.Marshal(data)
	s.clients.Range(func(key, value interface{}) bool {
		client := value.(*Client)
		select {
		case client.Channel <- payload:
		default:
			// Channel full, client too slow
			s.clients.Delete(key)
			close(client.Channel)
		}
		return true
	})
}

func main() {
	server := NewSSEServer()

	// Send events every 2 seconds
	go func() {
		for {
			time.Sleep(2 * time.Second)
			server.broadcast("message", map[string]interface{}{
				"time":    time.Now().Format(time.RFC3339),
				"message": "Server time update",
			})
		}
	}()

	// Log client count every 5 seconds
	go func() {
		for {
			time.Sleep(5 * time.Second)
			count := 0
			server.clients.Range(func(_, _ interface{}) bool {
				count++
				return true
			})
			fmt.Printf("Active clients: %d\n", count)
		}
	}()

	// Serve client
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "index.html")
	})

	// SSE endpoint
	http.HandleFunc("/events", server.handleSSE)

	// Stats endpoint
	http.HandleFunc("/stats", func(w http.ResponseWriter, r *http.Request) {
		activeClients := 0
		server.clients.Range(func(_, _ interface{}) bool {
			activeClients++
			return true
		})
		json.NewEncoder(w).Encode(map[string]int{
			"active_connections": activeClients,
		})
	})

	fmt.Println("Server starting on :8080...")
	http.ListenAndServe(":8080", nil)
}
