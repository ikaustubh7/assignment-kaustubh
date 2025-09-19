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

// Client message types
type ClientMessage struct {
	Type      string         `json:"type"`                // "subscribe" | "unsubscribe" | "publish" | "ping"
	Topic     string         `json:"topic,omitempty"`     // required for subscribe/unsubscribe/publish
	ClientID  string         `json:"client_id,omitempty"` // required for subscribe/unsubscribe
	Message   *ClientPayload `json:"message,omitempty"`   // required for publish
	LastN     int            `json:"last_n,omitempty"`    // optional (subscribe)
	RequestID string         `json:"request_id,omitempty"`
}

type SubscribeMessage struct {
	Type      string `json:"type"`             // always "subscribe"
	Topic     string `json:"topic"`            // required
	ClientID  string `json:"client_id"`        // required
	LastN     int    `json:"last_n,omitempty"` // optional
	RequestID string `json:"request_id,omitempty"`
}

type UnsubscribeMessage struct {
	Type      string `json:"type"`      // always "unsubscribe"
	Topic     string `json:"topic"`     // required
	ClientID  string `json:"client_id"` // required
	RequestID string `json:"request_id,omitempty"`
}

type PublishMessage struct {
	Type      string         `json:"type"`    // always "publish"
	Topic     string         `json:"topic"`   // required
	Message   PublishPayload `json:"message"` // required
	RequestID string         `json:"request_id,omitempty"`
}

type PublishPayload struct {
	ID      string      `json:"id"`      // UUID
	Payload interface{} `json:"payload"` // flexible JSON
}

type ClientPayload struct {
	ID      string      `json:"id"`      // UUID
	Payload interface{} `json:"payload"` // flexible JSON
}

type PingMessage struct {
	Type      string `json:"type"`                 // always "ping"
	RequestID string `json:"request_id,omitempty"` // optional
}

// Server message types
type ServerMessage struct {
	Type      string         `json:"type"`                 // "ack" | "event" | "error" | "pong" | "info"
	RequestID string         `json:"request_id,omitempty"` // echoed from client if provided
	Topic     string         `json:"topic,omitempty"`      // required for event
	Message   *ServerPayload `json:"message,omitempty"`    // present for event/info
	Error     *ServerError   `json:"error,omitempty"`      // present for error
	TS        string         `json:"ts,omitempty"`         // ISO timestamp
}

// Payload wrapper
type ServerPayload struct {
	ID      string      `json:"id"`
	Payload interface{} `json:"payload"`
}

// Error wrapper
type ServerError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// Legacy Message for internal hub communication (keeping for compatibility)
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
	Topics map[string]bool    // Set of subscribed topics
	Send   chan ServerMessage // Buffered channel for outbound messages
	Hub    *Hub
}

// Helper methods for creating server messages
func (c *Client) sendAck(requestID string) {
	msg := ServerMessage{
		Type:      "ack",
		RequestID: requestID,
		TS:        time.Now().UTC().Format(time.RFC3339),
	}
	select {
	case c.Send <- msg:
	default:
		// Channel full, close client
		c.Hub.Unregister <- c
	}
}

func (c *Client) sendError(requestID, code, message string) {
	msg := ServerMessage{
		Type:      "error",
		RequestID: requestID,
		Error: &ServerError{
			Code:    code,
			Message: message,
		},
		TS: time.Now().UTC().Format(time.RFC3339),
	}
	select {
	case c.Send <- msg:
	default:
		// Channel full, close client
		c.Hub.Unregister <- c
	}
}

func (c *Client) sendPong(requestID string) {
	msg := ServerMessage{
		Type:      "pong",
		RequestID: requestID,
		TS:        time.Now().UTC().Format(time.RFC3339),
	}
	select {
	case c.Send <- msg:
	default:
		// Channel full, close client
		c.Hub.Unregister <- c
	}
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

	// Convert internal Message to ServerMessage for clients
	serverMsg := ServerMessage{
		Type:  "event",
		Topic: message.Topic,
		TS:    time.Now().UTC().Format(time.RFC3339),
	}

	// Handle payload conversion
	if message.Payload != nil {
		if clientPayload, ok := message.Payload.(*ClientPayload); ok {
			serverMsg.Message = &ServerPayload{
				ID:      clientPayload.ID,
				Payload: clientPayload.Payload,
			}
		} else {
			// For backward compatibility, wrap simple payloads
			serverMsg.Message = &ServerPayload{
				ID:      fmt.Sprintf("msg_%d", time.Now().UnixNano()),
				Payload: message.Payload,
			}
		}
	}

	count := 0
	for client := range subscribers {
		select {
		case client.Send <- serverMsg:
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

// Topic management methods
func (h *Hub) CreateTopic(name string) error {
	h.Mutex.Lock()
	defer h.Mutex.Unlock()

	if _, exists := h.Topics[name]; exists {
		return fmt.Errorf("topic already exists")
	}

	h.Topics[name] = make(map[*Client]bool)
	log.Printf("Topic '%s' created", name)
	return nil
}

func (h *Hub) DeleteTopic(name string) error {
	h.Mutex.Lock()
	defer h.Mutex.Unlock()

	subscribers, exists := h.Topics[name]
	if !exists {
		return fmt.Errorf("topic not found")
	}

	// Disconnect all subscribers from this topic
	for client := range subscribers {
		delete(client.Topics, name)
		// Send unsubscribe notification to client
		msg := ServerMessage{
			Type:  "info",
			Topic: name,
			Message: &ServerPayload{
				ID:      fmt.Sprintf("info_%d", time.Now().UnixNano()),
				Payload: "Topic has been deleted",
			},
			TS: time.Now().UTC().Format(time.RFC3339),
		}
		select {
		case client.Send <- msg:
		default:
			// Client channel full, will be cleaned up later
		}
	}

	delete(h.Topics, name)
	log.Printf("Topic '%s' deleted with %d subscribers", name, len(subscribers))
	return nil
}

func (h *Hub) ListTopics() map[string]int {
	h.Mutex.RLock()
	defer h.Mutex.RUnlock()

	topics := make(map[string]int)
	for topic, subscribers := range h.Topics {
		topics[topic] = len(subscribers)
	}
	return topics
}

func (h *Hub) TopicExists(name string) bool {
	h.Mutex.RLock()
	defer h.Mutex.RUnlock()

	_, exists := h.Topics[name]
	return exists
}

// NewClient creates a new client
func NewClient(id string, conn *websocket.Conn, hub *Hub) *Client {
	return &Client{
		ID:     id,
		Conn:   conn,
		Topics: make(map[string]bool),
		Send:   make(chan ServerMessage, 256),
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
		var msg ClientMessage
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
func (c *Client) handleMessage(msg ClientMessage) {
	switch msg.Type {
	case "subscribe":
		if msg.Topic == "" {
			c.sendError(msg.RequestID, "INVALID_TOPIC", "Topic is required for subscribe")
			return
		}
		if msg.ClientID == "" {
			c.sendError(msg.RequestID, "INVALID_CLIENT_ID", "Client ID is required for subscribe")
			return
		}

		c.Hub.Subscribe <- &SubscribeRequest{Client: c, Topic: msg.Topic}
		c.sendAck(msg.RequestID)

	case "unsubscribe":
		if msg.Topic == "" {
			c.sendError(msg.RequestID, "INVALID_TOPIC", "Topic is required for unsubscribe")
			return
		}
		if msg.ClientID == "" {
			c.sendError(msg.RequestID, "INVALID_CLIENT_ID", "Client ID is required for unsubscribe")
			return
		}

		c.Hub.Unsubscribe <- &SubscribeRequest{Client: c, Topic: msg.Topic}
		c.sendAck(msg.RequestID)

	case "publish":
		if msg.Topic == "" {
			c.sendError(msg.RequestID, "INVALID_TOPIC", "Topic is required for publish")
			return
		}
		if msg.Message == nil {
			c.sendError(msg.RequestID, "INVALID_MESSAGE", "Message is required for publish")
			return
		}

		message := Message{
			Topic:   msg.Topic,
			Type:    "event",
			Payload: msg.Message,
			Sender:  c.ID,
		}
		c.Hub.Broadcast <- message
		c.sendAck(msg.RequestID)

	case "ping":
		c.sendPong(msg.RequestID)

	default:
		c.sendError(msg.RequestID, "INVALID_MESSAGE_TYPE", "Unknown message type: "+msg.Type)
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

// Topic management endpoints
func (h *Hub) HandleCreateTopic(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Name string `json:"name"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	if req.Name == "" {
		http.Error(w, "Topic name is required", http.StatusBadRequest)
		return
	}

	err := h.CreateTopic(req.Name)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusConflict)
		json.NewEncoder(w).Encode(map[string]string{
			"error": "Topic already exists",
		})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{
		"status": "created",
		"topic":  req.Name,
	})
}

func (h *Hub) HandleDeleteTopic(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract topic name from URL path
	path := r.URL.Path
	if len(path) < 8 || path[:8] != "/topics/" {
		http.Error(w, "Invalid path", http.StatusBadRequest)
		return
	}

	topicName := path[8:] // Remove "/topics/" prefix
	if topicName == "" {
		http.Error(w, "Topic name is required", http.StatusBadRequest)
		return
	}

	err := h.DeleteTopic(topicName)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{
			"error": "Topic not found",
		})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"status": "deleted",
		"topic":  topicName,
	})
}

func (h *Hub) HandleListTopics(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	topics := h.ListTopics()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"topics": topics,
		"count":  len(topics),
	})
}
