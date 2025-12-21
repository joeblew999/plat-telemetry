package cmd

import (
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/joeblew99/plat-telemetry/sync/pkg/webhook"
)

// Watch starts the webhook server
func Watch() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	server := webhook.NewServer()

	// Health check endpoint
	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, "OK")
	})

	// Webhook endpoint
	http.HandleFunc("/webhook", server.HandleWebhook)
	http.HandleFunc("/webhook/", server.HandleWebhook)

	addr := fmt.Sprintf(":%s", port)
	log.Printf("▶ Webhook server listening on %s", addr)

	if err := http.ListenAndServe(addr, nil); err != nil {
		log.Fatal(err)
	}
}
