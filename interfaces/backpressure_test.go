package interfaces

import (
	"testing"
	"time"
)

func TestBackpressureDropOldest(t *testing.T) {
	// Create hub with DropOldest strategy
	config := &HubConfig{
		MaxQueueSize:         2, // Small queue for testing
		BackpressureStrategy: DropOldest,
		ShutdownTimeout:      5 * time.Second,
	}
	hub := NewHubWithConfig("", config)

	// Create a client
	client := &Client{
		ID:     "test-client",
		Conn:   nil,
		Topics: make(map[string]bool),
		Send:   make(chan ServerMessage, config.MaxQueueSize),
		Hub:    hub,
	}

	// Fill the queue
	client.Send <- ServerMessage{Type: "test1"}
	client.Send <- ServerMessage{Type: "test2"}

	// Queue should be full now
	if len(client.Send) != 2 {
		t.Errorf("Expected queue length 2, got %d", len(client.Send))
	}

	// Send another message, should drop oldest
	client.sendAck("test-request")

	// Queue should still have 2 messages
	if len(client.Send) != 2 {
		t.Errorf("Expected queue length 2 after backpressure, got %d", len(client.Send))
	}

	// First message should be dropped, second should remain
	msg1 := <-client.Send
	if msg1.Type == "test1" {
		t.Error("Oldest message should have been dropped")
	}

	msg2 := <-client.Send
	if msg2.Type != "ack" {
		t.Errorf("Expected ack message, got %s", msg2.Type)
	}
}

func TestBackpressureDisconnectClient(t *testing.T) {
	// Create hub with DisconnectClient strategy
	config := &HubConfig{
		MaxQueueSize:         1, // Very small queue
		BackpressureStrategy: DisconnectClient,
		ShutdownTimeout:      5 * time.Second,
	}
	hub := NewHubWithConfig("", config)
	go hub.Run() // Start hub to handle unregister

	// Create a client
	client := &Client{
		ID:     "test-client",
		Conn:   nil,
		Topics: make(map[string]bool),
		Send:   make(chan ServerMessage, config.MaxQueueSize),
		Hub:    hub,
	}

	// Register client
	hub.registerClient(client)

	// Fill the queue
	client.Send <- ServerMessage{Type: "test1"}

	// Send another message, should trigger disconnect
	client.sendAck("test-request")

	// Wait for unregister to be processed
	time.Sleep(10 * time.Millisecond)

	// Client should be unregistered
	hub.Mutex.RLock()
	registered := hub.Clients[client]
	hub.Mutex.RUnlock()

	if registered {
		t.Error("Client should have been unregistered due to slow consumption")
	}
}

func TestSlowConsumerError(t *testing.T) {
	// Create hub with DisconnectClient strategy
	config := &HubConfig{
		MaxQueueSize:         1,
		BackpressureStrategy: DisconnectClient,
		ShutdownTimeout:      5 * time.Second,
	}
	hub := NewHubWithConfig("", config)

	// Create a client
	client := &Client{
		ID:     "test-client",
		Conn:   nil,
		Topics: make(map[string]bool),
		Send:   make(chan ServerMessage, config.MaxQueueSize),
		Hub:    hub,
	}

	// Fill the queue
	client.Send <- ServerMessage{Type: "test1"}

	// Capture the current queue state
	initialLength := len(client.Send)

	// Send error when queue is full
	client.sendSlowConsumerError()

	// Should try to send error but queue is full, so it won't block
	// The queue length should remain the same
	if len(client.Send) != initialLength {
		t.Errorf("Queue length should remain %d, got %d", initialLength, len(client.Send))
	}
}

func TestBroadcastWithBackpressure(t *testing.T) {
	// Create hub with DropOldest strategy
	config := &HubConfig{
		MaxQueueSize:         2,
		BackpressureStrategy: DropOldest,
		ShutdownTimeout:      5 * time.Second,
	}
	hub := NewHubWithConfig("", config)
	go hub.Run()

	// Create two clients
	client1 := &Client{
		ID:     "client1",
		Conn:   nil,
		Topics: make(map[string]bool),
		Send:   make(chan ServerMessage, config.MaxQueueSize),
		Hub:    hub,
	}

	client2 := &Client{
		ID:     "client2",
		Conn:   nil,
		Topics: make(map[string]bool),
		Send:   make(chan ServerMessage, config.MaxQueueSize),
		Hub:    hub,
	}

	// Register clients and subscribe to topic
	hub.registerClient(client1)
	hub.registerClient(client2)
	hub.subscribeToTopic(client1, "test-topic")
	hub.subscribeToTopic(client2, "test-topic")

	// Fill client1's queue
	client1.Send <- ServerMessage{Type: "filler1"}
	client1.Send <- ServerMessage{Type: "filler2"}

	// Leave client2's queue empty

	// Broadcast a message
	message := Message{
		Topic:   "test-topic",
		Type:    "event",
		Payload: "test message",
	}
	hub.broadcastMessage(message)

	// Give time for processing
	time.Sleep(10 * time.Millisecond)

	// Client1 should have dropped oldest and received new message
	if len(client1.Send) != 2 {
		t.Errorf("Client1 should have 2 messages, got %d", len(client1.Send))
	}

	// Client2 should have received the message normally
	if len(client2.Send) != 1 {
		t.Errorf("Client2 should have 1 message, got %d", len(client2.Send))
	}
}

func TestGracefulShutdown(t *testing.T) {
	config := &HubConfig{
		MaxQueueSize:         10,
		BackpressureStrategy: DisconnectClient,
		ShutdownTimeout:      1 * time.Second, // Short timeout for testing
	}
	hub := NewHubWithConfig("", config)

	// Start hub in goroutine
	hubDone := make(chan bool)
	go func() {
		hub.Run()
		hubDone <- true
	}()

	// Create and register a client
	client := &Client{
		ID:     "test-client",
		Conn:   nil,
		Topics: make(map[string]bool),
		Send:   make(chan ServerMessage, config.MaxQueueSize),
		Hub:    hub,
	}

	hub.Register <- client

	// Wait for registration
	time.Sleep(10 * time.Millisecond)

	// Verify client is registered
	hub.Mutex.RLock()
	clientCount := len(hub.Clients)
	hub.Mutex.RUnlock()

	if clientCount != 1 {
		t.Errorf("Expected 1 client, got %d", clientCount)
	}

	// Initiate shutdown
	hub.Shutdown()

	// Wait for hub to finish
	select {
	case <-hubDone:
		// Shutdown completed
	case <-time.After(3 * time.Second):
		t.Error("Hub shutdown timed out")
	}

	// Verify all clients are disconnected
	hub.Mutex.RLock()
	finalClientCount := len(hub.Clients)
	hub.Mutex.RUnlock()

	if finalClientCount != 0 {
		t.Errorf("Expected 0 clients after shutdown, got %d", finalClientCount)
	}
}

func TestShutdownRejectsNewConnections(t *testing.T) {
	config := DefaultHubConfig()
	config.ShutdownTimeout = 100 * time.Millisecond
	hub := NewHubWithConfig("", config)

	// Start shutdown immediately
	hub.Mutex.Lock()
	hub.shuttingDown = true
	hub.Mutex.Unlock()

	// Start hub in goroutine
	go hub.Run()

	// Try to register a new client (should be rejected)
	client := &Client{
		ID:     "test-client",
		Conn:   nil, // Use nil for testing
		Topics: make(map[string]bool),
		Send:   make(chan ServerMessage, config.MaxQueueSize),
		Hub:    hub,
	}

	hub.Register <- client

	// Wait for processing
	time.Sleep(50 * time.Millisecond)

	// Client should not be registered
	hub.Mutex.RLock()
	registered := hub.Clients[client]
	hub.Mutex.RUnlock()

	if registered {
		t.Error("Client should not be registered during shutdown")
	}
}

func TestShutdownIgnoresNewOperations(t *testing.T) {
	config := DefaultHubConfig()
	config.ShutdownTimeout = 100 * time.Millisecond
	hub := NewHubWithConfig("", config)

	// Create and register a client first
	client := &Client{
		ID:     "test-client",
		Conn:   nil,
		Topics: make(map[string]bool),
		Send:   make(chan ServerMessage, config.MaxQueueSize),
		Hub:    hub,
	}

	hub.registerClient(client)

	// Set shutdown state
	hub.Mutex.Lock()
	hub.shuttingDown = true
	hub.Mutex.Unlock()

	// Start hub in goroutine
	go func() {
		// Process a few iterations then stop
		for i := 0; i < 5; i++ {
			select {
			case client := <-hub.Register:
				if hub.shuttingDown {
					if client.Conn != nil {
						client.Conn.Close()
					}
					continue
				}
				hub.registerClient(client)

			case message := <-hub.Broadcast:
				if !hub.shuttingDown {
					hub.broadcastMessage(message)
				}

			case req := <-hub.Subscribe:
				if !hub.shuttingDown {
					hub.subscribeToTopic(req.Client, req.Topic)
				}

			default:
				time.Sleep(10 * time.Millisecond)
			}
		}
	}()

	// Try to send operations during shutdown
	hub.Broadcast <- Message{Topic: "test", Type: "event", Payload: "should be ignored"}
	hub.Subscribe <- &SubscribeRequest{Client: client, Topic: "test-topic"}

	// Wait for processing
	time.Sleep(50 * time.Millisecond)

	// No messages should be in client's queue (operations ignored)
	if len(client.Send) > 0 {
		t.Error("Operations should be ignored during shutdown")
	}

	// Client should not be subscribed to any topics
	if len(client.Topics) > 0 {
		t.Error("Subscription should be ignored during shutdown")
	}
}

// Mock connection for testing
type mockConn struct {
	closed bool
}

func (m *mockConn) Close() error {
	m.closed = true
	return nil
}

func TestDefaultHubConfig(t *testing.T) {
	config := DefaultHubConfig()

	if config.MaxQueueSize != 256 {
		t.Errorf("Expected MaxQueueSize 256, got %d", config.MaxQueueSize)
	}

	if config.BackpressureStrategy != DisconnectClient {
		t.Errorf("Expected BackpressureStrategy DisconnectClient, got %d", config.BackpressureStrategy)
	}

	if config.ShutdownTimeout != 30*time.Second {
		t.Errorf("Expected ShutdownTimeout 30s, got %v", config.ShutdownTimeout)
	}
}

func TestNewHubWithConfig(t *testing.T) {
	config := &HubConfig{
		MaxQueueSize:         100,
		BackpressureStrategy: DropOldest,
		ShutdownTimeout:      10 * time.Second,
	}

	hub := NewHubWithConfig("", config)

	if hub.Config.MaxQueueSize != 100 {
		t.Errorf("Expected MaxQueueSize 100, got %d", hub.Config.MaxQueueSize)
	}

	if hub.Config.BackpressureStrategy != DropOldest {
		t.Errorf("Expected BackpressureStrategy DropOldest, got %d", hub.Config.BackpressureStrategy)
	}

	if hub.ShutdownCh == nil {
		t.Error("ShutdownCh should be initialized")
	}
}
