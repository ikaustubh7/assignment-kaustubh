package main

import (
	"log"
	"net/http"

	interfaces "websocket-broadcast/interfaces"
)

func main() {
	// Create hub (pass Redis address for distributed mode)
	// hub := interfaces.NewHub("localhost:6379") // With Redis
	hub := interfaces.NewHub("") // Without Redis (single server)

	// Start hub
	go hub.Run()

	cors := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
			if r.Method == "OPTIONS" {
				w.WriteHeader(http.StatusNoContent)
				return
			}
			next.ServeHTTP(w, r)
		})
	}

	// Setup HTTP routes
	http.HandleFunc("/ws", cors(http.HandlerFunc(hub.HandleWebSocket)).ServeHTTP)
	http.HandleFunc("/publish", cors(http.HandlerFunc(hub.HandlePublish)).ServeHTTP)
	http.HandleFunc("/stats", cors(http.HandlerFunc(hub.HandleStats)).ServeHTTP)

	// Topic management routes
	http.HandleFunc("/topics", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case "POST":
			cors(http.HandlerFunc(hub.HandleCreateTopic)).ServeHTTP(w, r)
		case "GET":
			cors(http.HandlerFunc(hub.HandleListTopics)).ServeHTTP(w, r)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})

	http.HandleFunc("/topics/", cors(http.HandlerFunc(hub.HandleDeleteTopic)).ServeHTTP)

	// Serve static files for testing
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			http.ServeFile(w, r, "index.html")
		} else {
			http.NotFound(w, r)
		}
	})

	log.Println("Server starting on :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
