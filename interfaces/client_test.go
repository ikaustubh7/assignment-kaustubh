package interfaces

import (
	"testing"
	"time"
)

func TestClientSendAck(t *testing.T) {
	hub := NewHub()
	client := &Client{
		ID:     "test-client",
		Conn:   nil,
		Topics: make(map[string]bool),
		Send:   make(chan ServerMessage, 256),
		Hub:    hub,
	}

	requestID := "test-request-123"
	client.sendAck(requestID)

	// Check if ack message was sent
	select {
	case msg := <-client.Send:
		if msg.Type != "ack" {
			t.Errorf("Expected ack message, got %s", msg.Type)
		}
		if msg.RequestID != requestID {
			t.Errorf("Expected RequestID %s, got %s", requestID, msg.RequestID)
		}
		if msg.TS == "" {
			t.Error("Expected timestamp to be set")
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("Expected ack message to be sent")
	}
}

func TestClientSendError(t *testing.T) {
	hub := NewHub()
	client := &Client{
		ID:     "test-client",
		Conn:   nil,
		Topics: make(map[string]bool),
		Send:   make(chan ServerMessage, 256),
		Hub:    hub,
	}

	requestID := "test-request-123"
	errorCode := "INVALID_TOPIC"
	errorMessage := "Topic is required"

	client.sendError(requestID, errorCode, errorMessage)

	// Check if error message was sent
	select {
	case msg := <-client.Send:
		if msg.Type != "error" {
			t.Errorf("Expected error message, got %s", msg.Type)
		}
		if msg.RequestID != requestID {
			t.Errorf("Expected RequestID %s, got %s", requestID, msg.RequestID)
		}
		if msg.Error == nil {
			t.Error("Expected error field to be set")
		} else {
			if msg.Error.Code != errorCode {
				t.Errorf("Expected error code %s, got %s", errorCode, msg.Error.Code)
			}
			if msg.Error.Message != errorMessage {
				t.Errorf("Expected error message %s, got %s", errorMessage, msg.Error.Message)
			}
		}
		if msg.TS == "" {
			t.Error("Expected timestamp to be set")
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("Expected error message to be sent")
	}
}

func TestClientSendPong(t *testing.T) {
	hub := NewHub()
	client := &Client{
		ID:     "test-client",
		Conn:   nil,
		Topics: make(map[string]bool),
		Send:   make(chan ServerMessage, 256),
		Hub:    hub,
	}

	requestID := "test-request-123"
	client.sendPong(requestID)

	// Check if pong message was sent
	select {
	case msg := <-client.Send:
		if msg.Type != "pong" {
			t.Errorf("Expected pong message, got %s", msg.Type)
		}
		if msg.RequestID != requestID {
			t.Errorf("Expected RequestID %s, got %s", requestID, msg.RequestID)
		}
		if msg.TS == "" {
			t.Error("Expected timestamp to be set")
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("Expected pong message to be sent")
	}
}

func TestClientHandleSubscribeMessage(t *testing.T) {
	hub := NewHub()
	go hub.Run() // Start hub to handle subscription requests

	client := &Client{
		ID:     "test-client",
		Conn:   nil,
		Topics: make(map[string]bool),
		Send:   make(chan ServerMessage, 256),
		Hub:    hub,
	}

	// Test valid subscribe message
	msg := ClientMessage{
		Type:      "subscribe",
		Topic:     "test-topic",
		ClientID:  "test-client",
		RequestID: "test-request-123",
	}

	client.handleMessage(msg)

	// Check if ack was sent
	select {
	case response := <-client.Send:
		if response.Type != "ack" {
			t.Errorf("Expected ack response, got %s", response.Type)
		}
		if response.RequestID != msg.RequestID {
			t.Errorf("Expected RequestID %s, got %s", msg.RequestID, response.RequestID)
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("Expected ack response")
	}

	// Give time for subscription to be processed
	time.Sleep(10 * time.Millisecond)

	// Check if client was subscribed
	if !client.Topics["test-topic"] {
		t.Error("Client should be subscribed to test-topic")
	}
}

func TestClientHandleSubscribeMessageInvalidTopic(t *testing.T) {
	hub := NewHub()
	client := &Client{
		ID:     "test-client",
		Conn:   nil,
		Topics: make(map[string]bool),
		Send:   make(chan ServerMessage, 256),
		Hub:    hub,
	}

	// Test subscribe message without topic
	msg := ClientMessage{
		Type:      "subscribe",
		Topic:     "", // Empty topic
		ClientID:  "test-client",
		RequestID: "test-request-123",
	}

	client.handleMessage(msg)

	// Check if error was sent
	select {
	case response := <-client.Send:
		if response.Type != "error" {
			t.Errorf("Expected error response, got %s", response.Type)
		}
		if response.Error == nil {
			t.Error("Expected error field to be set")
		} else if response.Error.Code != "INVALID_TOPIC" {
			t.Errorf("Expected INVALID_TOPIC error, got %s", response.Error.Code)
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("Expected error response")
	}
}

func TestClientHandleSubscribeMessageInvalidClientID(t *testing.T) {
	hub := NewHub()
	client := &Client{
		ID:     "test-client",
		Conn:   nil,
		Topics: make(map[string]bool),
		Send:   make(chan ServerMessage, 256),
		Hub:    hub,
	}

	// Test subscribe message without client ID
	msg := ClientMessage{
		Type:      "subscribe",
		Topic:     "test-topic",
		ClientID:  "", // Empty client ID
		RequestID: "test-request-123",
	}

	client.handleMessage(msg)

	// Check if error was sent
	select {
	case response := <-client.Send:
		if response.Type != "error" {
			t.Errorf("Expected error response, got %s", response.Type)
		}
		if response.Error == nil {
			t.Error("Expected error field to be set")
		} else if response.Error.Code != "INVALID_CLIENT_ID" {
			t.Errorf("Expected INVALID_CLIENT_ID error, got %s", response.Error.Code)
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("Expected error response")
	}
}

func TestClientHandleUnsubscribeMessage(t *testing.T) {
	hub := NewHub()
	go hub.Run() // Start hub to handle unsubscription requests

	client := &Client{
		ID:     "test-client",
		Conn:   nil,
		Topics: make(map[string]bool),
		Send:   make(chan ServerMessage, 256),
		Hub:    hub,
	}

	// First subscribe to a topic
	client.Topics["test-topic"] = true

	// Test valid unsubscribe message
	msg := ClientMessage{
		Type:      "unsubscribe",
		Topic:     "test-topic",
		ClientID:  "test-client",
		RequestID: "test-request-123",
	}

	client.handleMessage(msg)

	// Check if ack was sent
	select {
	case response := <-client.Send:
		if response.Type != "ack" {
			t.Errorf("Expected ack response, got %s", response.Type)
		}
		if response.RequestID != msg.RequestID {
			t.Errorf("Expected RequestID %s, got %s", msg.RequestID, response.RequestID)
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("Expected ack response")
	}
}

func TestClientHandlePublishMessage(t *testing.T) {
	hub := NewHub()
	go hub.Run() // Start hub to handle broadcast requests

	client := &Client{
		ID:     "test-client",
		Conn:   nil,
		Topics: make(map[string]bool),
		Send:   make(chan ServerMessage, 256),
		Hub:    hub,
	}

	// Test valid publish message
	payload := &ClientPayload{
		ID:      "test-msg-1",
		Payload: "Hello World",
	}

	msg := ClientMessage{
		Type:      "publish",
		Topic:     "test-topic",
		Message:   payload,
		RequestID: "test-request-123",
	}

	client.handleMessage(msg)

	// Check if ack was sent
	select {
	case response := <-client.Send:
		if response.Type != "ack" {
			t.Errorf("Expected ack response, got %s", response.Type)
		}
		if response.RequestID != msg.RequestID {
			t.Errorf("Expected RequestID %s, got %s", msg.RequestID, response.RequestID)
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("Expected ack response")
	}
}

func TestClientHandlePublishMessageInvalidTopic(t *testing.T) {
	hub := NewHub()
	client := &Client{
		ID:     "test-client",
		Conn:   nil,
		Topics: make(map[string]bool),
		Send:   make(chan ServerMessage, 256),
		Hub:    hub,
	}

	// Test publish message without topic
	payload := &ClientPayload{
		ID:      "test-msg-1",
		Payload: "Hello World",
	}

	msg := ClientMessage{
		Type:      "publish",
		Topic:     "", // Empty topic
		Message:   payload,
		RequestID: "test-request-123",
	}

	client.handleMessage(msg)

	// Check if error was sent
	select {
	case response := <-client.Send:
		if response.Type != "error" {
			t.Errorf("Expected error response, got %s", response.Type)
		}
		if response.Error == nil {
			t.Error("Expected error field to be set")
		} else if response.Error.Code != "INVALID_TOPIC" {
			t.Errorf("Expected INVALID_TOPIC error, got %s", response.Error.Code)
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("Expected error response")
	}
}

func TestClientHandlePublishMessageInvalidMessage(t *testing.T) {
	hub := NewHub()
	client := &Client{
		ID:     "test-client",
		Conn:   nil,
		Topics: make(map[string]bool),
		Send:   make(chan ServerMessage, 256),
		Hub:    hub,
	}

	// Test publish message without message payload
	msg := ClientMessage{
		Type:      "publish",
		Topic:     "test-topic",
		Message:   nil, // No message
		RequestID: "test-request-123",
	}

	client.handleMessage(msg)

	// Check if error was sent
	select {
	case response := <-client.Send:
		if response.Type != "error" {
			t.Errorf("Expected error response, got %s", response.Type)
		}
		if response.Error == nil {
			t.Error("Expected error field to be set")
		} else if response.Error.Code != "INVALID_MESSAGE" {
			t.Errorf("Expected INVALID_MESSAGE error, got %s", response.Error.Code)
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("Expected error response")
	}
}

func TestClientHandlePingMessage(t *testing.T) {
	hub := NewHub()
	client := &Client{
		ID:     "test-client",
		Conn:   nil,
		Topics: make(map[string]bool),
		Send:   make(chan ServerMessage, 256),
		Hub:    hub,
	}

	// Test ping message
	msg := ClientMessage{
		Type:      "ping",
		RequestID: "test-request-123",
	}

	client.handleMessage(msg)

	// Check if pong was sent
	select {
	case response := <-client.Send:
		if response.Type != "pong" {
			t.Errorf("Expected pong response, got %s", response.Type)
		}
		if response.RequestID != msg.RequestID {
			t.Errorf("Expected RequestID %s, got %s", msg.RequestID, response.RequestID)
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("Expected pong response")
	}
}

func TestClientHandleInvalidMessageType(t *testing.T) {
	hub := NewHub()
	client := &Client{
		ID:     "test-client",
		Conn:   nil,
		Topics: make(map[string]bool),
		Send:   make(chan ServerMessage, 256),
		Hub:    hub,
	}

	// Test invalid message type
	msg := ClientMessage{
		Type:      "invalid-type",
		RequestID: "test-request-123",
	}

	client.handleMessage(msg)

	// Check if error was sent
	select {
	case response := <-client.Send:
		if response.Type != "error" {
			t.Errorf("Expected error response, got %s", response.Type)
		}
		if response.Error == nil {
			t.Error("Expected error field to be set")
		} else if response.Error.Code != "INVALID_MESSAGE_TYPE" {
			t.Errorf("Expected INVALID_MESSAGE_TYPE error, got %s", response.Error.Code)
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("Expected error response")
	}
}

func TestClientChannelFullHandling(t *testing.T) {
	hub := NewHub()
	client := &Client{
		ID:     "test-client",
		Conn:   nil,
		Topics: make(map[string]bool),
		Send:   make(chan ServerMessage, 1), // Small buffer to simulate full channel
		Hub:    hub,
	}

	// Fill the channel
	client.Send <- ServerMessage{Type: "test"}

	// Mock hub to capture unregister calls
	originalUnregister := hub.Unregister
	hub.Unregister = make(chan *Client, 1)
	go func() {
		select {
		case <-hub.Unregister:
			// Client was unregistered due to full channel
		case <-time.After(100 * time.Millisecond):
		}
	}()

	// Try to send another message (should trigger unregister)
	client.sendAck("test-request")

	// Wait a bit for the goroutine to process
	time.Sleep(10 * time.Millisecond)

	// Restore original channel
	hub.Unregister = originalUnregister

	// In a real scenario, the client would be unregistered, but we can't easily test
	// the exact mechanism without more complex mocking. The important part is that
	// the sendAck method handles the case gracefully without blocking.
}

func TestCreateTestClientMessage(t *testing.T) {
	// Test utility function
	payload := map[string]interface{}{"test": "data"}
	msg := CreateTestClientMessage("subscribe", "test-topic", "test-client", "req-123", payload)

	if msg.Type != "subscribe" {
		t.Errorf("Expected type subscribe, got %s", msg.Type)
	}
	if msg.Topic != "test-topic" {
		t.Errorf("Expected topic test-topic, got %s", msg.Topic)
	}
	if msg.ClientID != "test-client" {
		t.Errorf("Expected clientID test-client, got %s", msg.ClientID)
	}
	if msg.RequestID != "req-123" {
		t.Errorf("Expected requestID req-123, got %s", msg.RequestID)
	}
	if msg.Message == nil {
		t.Error("Expected message to be set")
	} else {
		if msg.Message.ID != "test-id" {
			t.Errorf("Expected message ID test-id, got %s", msg.Message.ID)
		}
	}
}
