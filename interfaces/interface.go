package interfaces

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
	Topics map[string]map[*Client]bool

	// All connected clients
	Clients map[*Client]bool

	// Channel for client registration
	Register chan *Client

	// Channel for client unregistration
	Unregister chan *Client

	// Channel for broadcasting messages
	Broadcast chan Message

	// Channel for topic subscription
	Subscribe chan *SubscribeRequest

	// Channel for topic unsubscription
	Unsubscribe chan *SubscribeRequest

	// Mutex for thread-safe operations
	Mutex sync.RWMutex
}

// SubscribeRequest represents a subscription/unsubscription request
type SubscribeRequest struct {
	Client *Client
	Topic  string
}

// WebSocket upgrader
var Upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all origins in this example
	},
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

// NewHub creates a new Hub instance
func NewHub(redisAddr string) *Hub {
	hub := &Hub{
		Topics:      make(map[string]map[*Client]bool),
		Clients:     make(map[*Client]bool),
		Register:    make(chan *Client),
		Unregister:  make(chan *Client),
		Broadcast:   make(chan Message, 256),
		Subscribe:   make(chan *SubscribeRequest),
		Unsubscribe: make(chan *SubscribeRequest),
	}

	return hub
}

// Run starts the hub's main event loop
func (h *Hub) Run() {
	for {
		select {
		case client := <-h.Register:
			h.registerClient(client)

		case client := <-h.Unregister:
			h.unregisterClient(client)

		case message := <-h.Broadcast:
			h.broadcastMessage(message)

		case req := <-h.Subscribe:
			h.subscribeToTopic(req.Client, req.Topic)

		case req := <-h.Unsubscribe:
			h.unsubscribeFromTopic(req.Client, req.Topic)
		}
	}
}

// registerClient adds a new client to the hub
func (h *Hub) registerClient(client *Client) {
	h.Mutex.Lock()
	defer h.Mutex.Unlock()

	h.Clients[client] = true
	log.Printf("Client %s connected. Total clients: %d", client.ID, len(h.Clients))
}

// unregisterClient removes a client and cleans up subscriptions
func (h *Hub) unregisterClient(client *Client) {
	h.Mutex.Lock()
	defer h.Mutex.Unlock()

	if _, ok := h.Clients[client]; ok {
		// Remove from all topics
		for topic := range client.Topics {
			if subscribers, exists := h.Topics[topic]; exists {
				delete(subscribers, client)
				if len(subscribers) == 0 {
					delete(h.Topics, topic)
				}
			}
		}

		// Remove from clients
		delete(h.Clients, client)
		close(client.Send)

		log.Printf("Client %s disconnected. Total clients: %d", client.ID, len(h.Clients))
	}
}

// subscribeToTopic adds a client to a topic
func (h *Hub) subscribeToTopic(client *Client, topic string) {
	h.Mutex.Lock()
	defer h.Mutex.Unlock()

	// Initialize topic if it doesn't exist
	if h.Topics[topic] == nil {
		h.Topics[topic] = make(map[*Client]bool)
	}

	// Add client to topic
	h.Topics[topic][client] = true
	client.Topics[topic] = true

	log.Printf("Client %s subscribed to topic '%s'. Subscribers: %d",
		client.ID, topic, len(h.Topics[topic]))
}

// unsubscribeFromTopic removes a client from a topic
func (h *Hub) unsubscribeFromTopic(client *Client, topic string) {
	h.Mutex.Lock()
	defer h.Mutex.Unlock()

	if subscribers, exists := h.Topics[topic]; exists {
		delete(subscribers, client)
		delete(client.Topics, topic)

		// Clean up empty topic
		if len(subscribers) == 0 {
			delete(h.Topics, topic)
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
	h.Mutex.RLock()
	defer h.Mutex.RUnlock()

	subscribers, exists := h.Topics[message.Topic]
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
				h.Unregister <- c
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
		c.Hub.Unregister <- c
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
			c.Hub.Subscribe <- &SubscribeRequest{Client: c, Topic: topic}
		}

	case "unsubscribe":
		if topic, ok := msg["topic"].(string); ok {
			c.Hub.Unsubscribe <- &SubscribeRequest{Client: c, Topic: topic}
		}

	case "publish":
		if topic, ok := msg["topic"].(string); ok {
			message := Message{
				Topic:   topic,
				Type:    "message",
				Payload: msg["payload"],
				Sender:  c.ID,
			}
			c.Hub.Broadcast <- message
		}
	}
}

// HTTP handlers
func (h *Hub) HandleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := Upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket upgrade error: %v", err)
		return
	}

	clientID := fmt.Sprintf("client_%d", time.Now().UnixNano())
	client := NewClient(clientID, conn, h)

	// Register client
	h.Register <- client

	// Start goroutines
	go client.writePump()
	go client.readPump()
}

// API endpoint to publish messages
func (h *Hub) HandlePublish(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var message Message
	if err := json.NewDecoder(r.Body).Decode(&message); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	h.Broadcast <- message

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "published"})
}

// API endpoint to get hub statistics
func (h *Hub) HandleStats(w http.ResponseWriter, r *http.Request) {
	h.Mutex.RLock()
	defer h.Mutex.RUnlock()

	stats := map[string]interface{}{
		"total_clients": len(h.Clients),
		"total_topics":  len(h.Topics),
		"topics":        make(map[string]int),
	}

	for topic, subscribers := range h.Topics {
		stats["topics"].(map[string]int)[topic] = len(subscribers)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stats)
}
