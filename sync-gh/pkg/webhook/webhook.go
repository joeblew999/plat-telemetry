package webhook

import (
	"context"
	"fmt"
	"log"
	"net/http"

	"github.com/cbrgm/githubevents/v2/githubevents"
)

// Server handles webhook events
type Server struct {
	handler *githubevents.EventHandler
}

// NewServer creates a new webhook server with githubevents
func NewServer() *Server {
	handler := githubevents.New("")

	// Log ALL events - we'll decide what to do with them later
	handler.OnBeforeAny(func(ctx context.Context, deliveryID string, eventName string, event interface{}) error {
		log.Printf("Event: %s [delivery: %s]", eventName, deliveryID)
		return nil
	})

	return &Server{
		handler: handler,
	}
}

// HandleWebhook processes incoming webhook requests
func (s *Server) HandleWebhook(w http.ResponseWriter, r *http.Request) {
	err := s.handler.HandleEventRequest(r)
	if err != nil {
		log.Printf("Webhook error: %v", err)
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, "OK")
}

// Run starts a standalone webhook server on the specified port
func Run(port string) {
	if port == "" {
		port = "8080"
	}

	server := NewServer()

	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, "OK")
	})

	http.HandleFunc("/webhook", server.HandleWebhook)
	http.HandleFunc("/webhook/", server.HandleWebhook)

	addr := fmt.Sprintf(":%s", port)
	log.Printf("Webhook server listening on %s", addr)

	if err := http.ListenAndServe(addr, nil); err != nil {
		log.Fatal(err)
	}
}
