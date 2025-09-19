package interfaces

import (
	"testing"
	"time"
)

func TestNewHub(t *testing.T) {
	hub := NewHub()

	if hub == nil {
		t.Error("NewHub should return a non-nil hub")
	}

	if hub.Topics == nil {
		t.Error("Hub Topics should be initialized")
	}

	if hub.Clients == nil {
		t.Error("Hub Clients should be initialized")
	}

	if hub.Register == nil {
		t.Error("Hub Register channel should be initialized")
	}

	if hub.Unregister == nil {
		t.Error("Hub Unregister channel should be initialized")
	}

	if hub.Broadcast == nil {
		t.Error("Hub Broadcast channel should be initialized")
	}

	if hub.Subscribe == nil {
		t.Error("Hub Subscribe channel should be initialized")
	}

	if hub.Unsubscribe == nil {
		t.Error("Hub Unsubscribe channel should be initialized")
	}
}

func TestNewClient(t *testing.T) {
	hub := NewHub()

	// Create a client with nil connection (for unit testing)
	client := &Client{
		ID:     "test-client",
		Conn:   nil,
		Topics: make(map[string]bool),
		Send:   make(chan ServerMessage, 256),
		Hub:    hub,
	}

	if client.ID != "test-client" {
		t.Error("Client ID should be set correctly")
	}

	if client.Topics == nil {
		t.Error("Client Topics should be initialized")
	}

	if client.Send == nil {
		t.Error("Client Send channel should be initialized")
	}

	if client.Hub != hub {
		t.Error("Client Hub should be set correctly")
	}
}

func TestHubClientRegistration(t *testing.T) {
	hub := NewHub()

	// Create a client with nil connection for testing
	client := &Client{
		ID:     "test-client-1",
		Conn:   nil,
		Topics: make(map[string]bool),
		Send:   make(chan ServerMessage, 256),
		Hub:    hub,
	}

	// Register client
	hub.registerClient(client)

	// Check if client is registered
	if !hub.Clients[client] {
		t.Error("Client should be registered in hub")
	}

	// Check client count
	if len(hub.Clients) != 1 {
		t.Errorf("Expected 1 client, got %d", len(hub.Clients))
	}
}

func TestHubClientUnregistration(t *testing.T) {
	hub := NewHub()
	client := &Client{
		ID:     "test-client-1",
		Conn:   nil,
		Topics: make(map[string]bool),
		Send:   make(chan ServerMessage, 256),
		Hub:    hub,
	}

	// Register client first
	hub.registerClient(client)

	// Subscribe client to a topic
	hub.subscribeToTopic(client, "test-topic")

	// Verify subscription
	if !client.Topics["test-topic"] {
		t.Error("Client should be subscribed to test-topic")
	}

	if len(hub.Topics["test-topic"]) != 1 {
		t.Error("Topic should have 1 subscriber")
	}

	// Unregister client
	hub.unregisterClient(client)

	// Check if client is unregistered
	if hub.Clients[client] {
		t.Error("Client should be unregistered from hub")
	}

	// Check if client is removed from topics
	if len(hub.Topics) != 0 {
		t.Error("Topics should be empty after client unregistration")
	}

	// Check client count
	if len(hub.Clients) != 0 {
		t.Errorf("Expected 0 clients, got %d", len(hub.Clients))
	}
}

func TestHubTopicSubscription(t *testing.T) {
	hub := NewHub()
	client := &Client{
		ID:     "test-client-1",
		Conn:   nil,
		Topics: make(map[string]bool),
		Send:   make(chan ServerMessage, 256),
		Hub:    hub,
	}

	// Register client
	hub.registerClient(client)

	// Subscribe to topic
	hub.subscribeToTopic(client, "test-topic")

	// Check if topic exists
	if hub.Topics["test-topic"] == nil {
		t.Error("Topic should be created")
	}

	// Check if client is subscribed
	if !hub.Topics["test-topic"][client] {
		t.Error("Client should be subscribed to topic")
	}

	if !client.Topics["test-topic"] {
		t.Error("Client should have topic in its subscription list")
	}

	// Check subscriber count
	if len(hub.Topics["test-topic"]) != 1 {
		t.Errorf("Expected 1 subscriber, got %d", len(hub.Topics["test-topic"]))
	}
}

func TestHubTopicUnsubscription(t *testing.T) {
	hub := NewHub()
	client := &Client{
		ID:     "test-client-1",
		Conn:   nil,
		Topics: make(map[string]bool),
		Send:   make(chan ServerMessage, 256),
		Hub:    hub,
	}

	// Register and subscribe
	hub.registerClient(client)
	hub.subscribeToTopic(client, "test-topic")

	// Verify subscription
	if !client.Topics["test-topic"] {
		t.Error("Client should be subscribed initially")
	}

	// Unsubscribe
	hub.unsubscribeFromTopic(client, "test-topic")

	// Check if client is unsubscribed
	if client.Topics["test-topic"] {
		t.Error("Client should be unsubscribed from topic")
	}

	// Check if topic is cleaned up (no subscribers)
	if len(hub.Topics) != 0 {
		t.Error("Empty topics should be cleaned up")
	}
}

func TestHubMultipleClientsSubscription(t *testing.T) {
	hub := NewHub()

	// Create multiple clients
	clients := make([]*Client, 3)
	for i := 0; i < 3; i++ {
		clients[i] = &Client{
			ID:     "test-client-" + string(rune('1'+i)),
			Conn:   nil,
			Topics: make(map[string]bool),
			Send:   make(chan ServerMessage, 256),
			Hub:    hub,
		}
		hub.registerClient(clients[i])
	}

	// Subscribe all clients to same topic
	for _, client := range clients {
		hub.subscribeToTopic(client, "shared-topic")
	}

	// Check subscriber count
	if len(hub.Topics["shared-topic"]) != 3 {
		t.Errorf("Expected 3 subscribers, got %d", len(hub.Topics["shared-topic"]))
	}

	// Unsubscribe one client
	hub.unsubscribeFromTopic(clients[0], "shared-topic")

	// Check remaining subscribers
	if len(hub.Topics["shared-topic"]) != 2 {
		t.Errorf("Expected 2 subscribers after unsubscribe, got %d", len(hub.Topics["shared-topic"]))
	}
}

func TestHubCreateTopic(t *testing.T) {
	hub := NewHub()

	// Create topic
	err := hub.CreateTopic("new-topic")
	if err != nil {
		t.Errorf("CreateTopic should not return error: %v", err)
	}

	// Check if topic exists
	if hub.Topics["new-topic"] == nil {
		t.Error("Topic should be created")
	}

	// Try to create same topic again
	err = hub.CreateTopic("new-topic")
	if err == nil {
		t.Error("CreateTopic should return error for existing topic")
	}
}

func TestHubDeleteTopic(t *testing.T) {
	hub := NewHub()

	// Create topic and add subscriber
	client := &Client{
		ID:     "test-client",
		Conn:   nil,
		Topics: make(map[string]bool),
		Send:   make(chan ServerMessage, 256),
		Hub:    hub,
	}
	hub.registerClient(client)
	hub.subscribeToTopic(client, "delete-topic")

	// Delete topic
	err := hub.DeleteTopic("delete-topic")
	if err != nil {
		t.Errorf("DeleteTopic should not return error: %v", err)
	}

	// Check if topic is deleted
	if hub.Topics["delete-topic"] != nil {
		t.Error("Topic should be deleted")
	}

	// Check if client is unsubscribed
	if client.Topics["delete-topic"] {
		t.Error("Client should be unsubscribed from deleted topic")
	}

	// Try to delete non-existing topic
	err = hub.DeleteTopic("non-existing")
	if err == nil {
		t.Error("DeleteTopic should return error for non-existing topic")
	}
}

func TestHubListTopics(t *testing.T) {
	hub := NewHub()

	// Initially should be empty
	topics := hub.ListTopics()
	if len(topics) != 0 {
		t.Errorf("Expected 0 topics initially, got %d", len(topics))
	}

	// Create some topics with subscribers
	client1 := &Client{
		ID:     "client1",
		Conn:   nil,
		Topics: make(map[string]bool),
		Send:   make(chan ServerMessage, 256),
		Hub:    hub,
	}
	hub.registerClient(client1)
	hub.subscribeToTopic(client1, "topic1")

	client2 := &Client{
		ID:     "client2",
		Conn:   nil,
		Topics: make(map[string]bool),
		Send:   make(chan ServerMessage, 256),
		Hub:    hub,
	}
	hub.registerClient(client2)
	hub.subscribeToTopic(client2, "topic1")
	hub.subscribeToTopic(client2, "topic2")

	// List topics
	topics = hub.ListTopics()

	if len(topics) != 2 {
		t.Errorf("Expected 2 topics, got %d", len(topics))
	}

	if topics["topic1"] != 2 {
		t.Errorf("Expected 2 subscribers for topic1, got %d", topics["topic1"])
	}

	if topics["topic2"] != 1 {
		t.Errorf("Expected 1 subscriber for topic2, got %d", topics["topic2"])
	}
}

func TestHubTopicExists(t *testing.T) {
	hub := NewHub()

	// Non-existing topic
	if hub.TopicExists("non-existing") {
		t.Error("TopicExists should return false for non-existing topic")
	}

	// Create topic
	client := &Client{
		ID:     "client",
		Conn:   nil,
		Topics: make(map[string]bool),
		Send:   make(chan ServerMessage, 256),
		Hub:    hub,
	}
	hub.registerClient(client)
	hub.subscribeToTopic(client, "existing-topic")

	// Existing topic
	if !hub.TopicExists("existing-topic") {
		t.Error("TopicExists should return true for existing topic")
	}
}

func TestHubBroadcastMessage(t *testing.T) {
	hub := NewHub()

	// Create clients
	clients := make([]*Client, 2)
	for i := 0; i < 2; i++ {
		clients[i] = &Client{
			ID:     "test-client-" + string(rune('1'+i)),
			Conn:   nil,
			Topics: make(map[string]bool),
			Send:   make(chan ServerMessage, 256),
			Hub:    hub,
		}
		hub.registerClient(clients[i])
		hub.subscribeToTopic(clients[i], "broadcast-topic")
	}

	// Create test message
	message := Message{
		Topic:   "broadcast-topic",
		Type:    "event",
		Payload: &ClientPayload{ID: "test-msg-1", Payload: "Hello World"},
		Sender:  "test-sender",
	}

	// Broadcast message
	hub.broadcastMessage(message)

	// Check if both clients have message in their send channel
	for i, client := range clients {
		select {
		case msg := <-client.Send:
			if msg.Type != "event" {
				t.Errorf("Client %d expected event message, got %s", i, msg.Type)
			}
			if msg.Topic != "broadcast-topic" {
				t.Errorf("Client %d expected broadcast-topic, got %s", i, msg.Topic)
			}
		case <-time.After(100 * time.Millisecond):
			t.Errorf("Client %d did not receive message", i)
		}
	}
}
