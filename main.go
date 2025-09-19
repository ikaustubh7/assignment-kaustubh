package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// Message represents a broadcast message
type Message struct {
	Topic   string      `json:"topic"`
	Type    string      `json:"type"`
	Payload interface{} `json:"payload"`
	Sender  string      `json:"sender,omitempty"`
}

// Client represents a WebSocket connection
type Client struct {
	ID     string
	Conn   *websocket.Conn
	Topics map[string]bool // Set of subscribed topics
	Send   chan Message    // Buffered channel for outbound messages
	Hub    *Hub
}

// Hub manages all clients and topic subscriptions
type Hub struct {
	// Topic -> Set of clients
	topics map[string]map[*Client]bool

	// All connected clients
	clients map[*Client]bool

	// Channel for client registration
	register chan *Client

	// Channel for client unregistration
	unregister chan *Client

	// Channel for broadcasting messages
	broadcast chan Message

	// Channel for topic subscription
	subscribe chan *SubscribeRequest

	// Channel for topic unsubscription
	unsubscribe chan *SubscribeRequest

	// Mutex for thread-safe operations
	mutex sync.RWMutex

}

// SubscribeRequest represents a subscription/unsubscription request
type SubscribeRequest struct {
	Client *Client
	Topic  string
}

// WebSocket upgrader
var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all origins in this example
	},
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

// NewHub creates a new Hub instance
func NewHub(redisAddr string) *Hub {
	hub := &Hub{
		topics:      make(map[string]map[*Client]bool),
		clients:     make(map[*Client]bool),
		register:    make(chan *Client),
		unregister:  make(chan *Client),
		broadcast:   make(chan Message, 256),
		subscribe:   make(chan *SubscribeRequest),
		unsubscribe: make(chan *SubscribeRequest),
	}

	return hub
}

// Run starts the hub's main event loop
func (h *Hub) Run() {
	for {
		select {
		case client := <-h.register:
			h.registerClient(client)

		case client := <-h.unregister:
			h.unregisterClient(client)

		case message := <-h.broadcast:
			h.broadcastMessage(message)

		case req := <-h.subscribe:
			h.subscribeToTopic(req.Client, req.Topic)

		case req := <-h.unsubscribe:
			h.unsubscribeFromTopic(req.Client, req.Topic)
		}
	}
}

// registerClient adds a new client to the hub
func (h *Hub) registerClient(client *Client) {
	h.mutex.Lock()
	defer h.mutex.Unlock()

	h.clients[client] = true
	log.Printf("Client %s connected. Total clients: %d", client.ID, len(h.clients))
}

// unregisterClient removes a client and cleans up subscriptions
func (h *Hub) unregisterClient(client *Client) {
	h.mutex.Lock()
	defer h.mutex.Unlock()

	if _, ok := h.clients[client]; ok {
		// Remove from all topics
		for topic := range client.Topics {
			if subscribers, exists := h.topics[topic]; exists {
				delete(subscribers, client)
				if len(subscribers) == 0 {
					delete(h.topics, topic)
				}
			}
		}

		// Remove from clients
		delete(h.clients, client)
		close(client.Send)

		log.Printf("Client %s disconnected. Total clients: %d", client.ID, len(h.clients))
	}
}

// subscribeToTopic adds a client to a topic
func (h *Hub) subscribeToTopic(client *Client, topic string) {
	h.mutex.Lock()
	defer h.mutex.Unlock()

	// Initialize topic if it doesn't exist
	if h.topics[topic] == nil {
		h.topics[topic] = make(map[*Client]bool)
	}

	// Add client to topic
	h.topics[topic][client] = true
	client.Topics[topic] = true

	log.Printf("Client %s subscribed to topic '%s'. Subscribers: %d",
		client.ID, topic, len(h.topics[topic]))
}

// unsubscribeFromTopic removes a client from a topic
func (h *Hub) unsubscribeFromTopic(client *Client, topic string) {
	h.mutex.Lock()
	defer h.mutex.Unlock()

	if subscribers, exists := h.topics[topic]; exists {
		delete(subscribers, client)
		delete(client.Topics, topic)

		// Clean up empty topic
		if len(subscribers) == 0 {
			delete(h.topics, topic)
		}

		log.Printf("Client %s unsubscribed from topic '%s'", client.ID, topic)
	}
}

// broadcastMessage sends a message to all subscribers of a topic
func (h *Hub) broadcastMessage(message Message) {

	// Local broadcast
	h.localBroadcast(message)
}

// localBroadcast handles local message broadcasting
func (h *Hub) localBroadcast(message Message) {
	h.mutex.RLock()
	defer h.mutex.RUnlock()

	subscribers, exists := h.topics[message.Topic]
	if !exists {
		return
	}

	count := 0
	for client := range subscribers {
		select {
		case client.Send <- message:
			count++
		default:
			// Client's send channel is full, remove it
			go func(c *Client) {
				h.unregister <- c
			}(client)
		}
	}

	log.Printf("Broadcasted message to topic '%s': %d recipients", message.Topic, count)
}





// NewClient creates a new client
func NewClient(id string, conn *websocket.Conn, hub *Hub) *Client {
	return &Client{
		ID:     id,
		Conn:   conn,
		Topics: make(map[string]bool),
		Send:   make(chan Message, 256),
		Hub:    hub,
	}
}

// writePump handles writing messages to the WebSocket
func (c *Client) writePump() {
	ticker := time.NewTicker(54 * time.Second)
	defer func() {
		ticker.Stop()
		c.Conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.Send:
			c.Conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if !ok {
				c.Conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			if err := c.Conn.WriteJSON(message); err != nil {
				log.Printf("Error writing to client %s: %v", c.ID, err)
				return
			}

		case <-ticker.C:
			c.Conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := c.Conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// readPump handles reading messages from the WebSocket
func (c *Client) readPump() {
	defer func() {
		c.Hub.unregister <- c
		c.Conn.Close()
	}()

	c.Conn.SetReadLimit(512)
	c.Conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	c.Conn.SetPongHandler(func(string) error {
		c.Conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	for {
		var msg map[string]interface{}
		err := c.Conn.ReadJSON(&msg)
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("WebSocket error for client %s: %v", c.ID, err)
			}
			break
		}

		c.handleMessage(msg)
	}
}

// handleMessage processes incoming client messages
func (c *Client) handleMessage(msg map[string]interface{}) {
	msgType, ok := msg["type"].(string)
	if !ok {
		return
	}

	switch msgType {
	case "subscribe":
		if topic, ok := msg["topic"].(string); ok {
			c.Hub.subscribe <- &SubscribeRequest{Client: c, Topic: topic}
		}

	case "unsubscribe":
		if topic, ok := msg["topic"].(string); ok {
			c.Hub.unsubscribe <- &SubscribeRequest{Client: c, Topic: topic}
		}

	case "publish":
		if topic, ok := msg["topic"].(string); ok {
			message := Message{
				Topic:   topic,
				Type:    "message",
				Payload: msg["payload"],
				Sender:  c.ID,
			}
			c.Hub.broadcast <- message
		}
	}
}

// HTTP handlers
func (h *Hub) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket upgrade error: %v", err)
		return
	}

	clientID := fmt.Sprintf("client_%d", time.Now().UnixNano())
	client := NewClient(clientID, conn, h)

	// Register client
	h.register <- client

	// Start goroutines
	go client.writePump()
	go client.readPump()
}

// API endpoint to publish messages
func (h *Hub) handlePublish(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var message Message
	if err := json.NewDecoder(r.Body).Decode(&message); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	h.broadcast <- message

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "published"})
}

// API endpoint to get hub statistics
func (h *Hub) handleStats(w http.ResponseWriter, r *http.Request) {
	h.mutex.RLock()
	defer h.mutex.RUnlock()

	stats := map[string]interface{}{
		"total_clients": len(h.clients),
		"total_topics":  len(h.topics),
		"topics":        make(map[string]int),
	}

	for topic, subscribers := range h.topics {
		stats["topics"].(map[string]int)[topic] = len(subscribers)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stats)
}

func main() {
	// Create hub (pass Redis address for distributed mode)
	// hub := NewHub("localhost:6379") // With Redis
	hub := NewHub("") // Without Redis (single server)

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
	http.HandleFunc("/ws", cors(http.HandlerFunc(hub.handleWebSocket)).ServeHTTP)
	http.HandleFunc("/publish", cors(http.HandlerFunc(hub.handlePublish)).ServeHTTP)
	http.HandleFunc("/stats", cors(http.HandlerFunc(hub.handleStats)).ServeHTTP)

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
