package interfaces

import (
	"encoding/json"
	"testing"
	"time"
)

func TestWebSocketIntegrationSubscribeAndPublish(t *testing.T) {
	// Create test server
	server, hub := CreateTestServer()
	defer server.Close()

	// Create test clients
	client1, err := NewTestClient("client1", server.URL)
	if err != nil {
		t.Fatalf("Failed to create test client1: %v", err)
	}
	defer client1.Close()

	client2, err := NewTestClient("client2", server.URL)
	if err != nil {
		t.Fatalf("Failed to create test client2: %v", err)
	}
	defer client2.Close()

	// Wait for connections to be established
	time.Sleep(50 * time.Millisecond)

	// Subscribe both clients to the same topic
	subscribeMsg1 := CreateTestClientMessage("subscribe", "test-topic", "client1", "req-1", nil)
	err = client1.SendMessage(subscribeMsg1)
	if err != nil {
		t.Fatalf("Failed to send subscribe message for client1: %v", err)
	}

	subscribeMsg2 := CreateTestClientMessage("subscribe", "test-topic", "client2", "req-2", nil)
	err = client2.SendMessage(subscribeMsg2)
	if err != nil {
		t.Fatalf("Failed to send subscribe message for client2: %v", err)
	}

	// Wait for subscription acknowledgments
	ack1 := client1.WaitForMessage("ack", time.Second)
	if ack1 == nil {
		t.Error("Client1 should receive subscription acknowledgment")
	} else if ack1.RequestID != "req-1" {
		t.Errorf("Expected RequestID req-1, got %s", ack1.RequestID)
	}

	ack2 := client2.WaitForMessage("ack", time.Second)
	if ack2 == nil {
		t.Error("Client2 should receive subscription acknowledgment")
	} else if ack2.RequestID != "req-2" {
		t.Errorf("Expected RequestID req-2, got %s", ack2.RequestID)
	}

	// Publish a message from client1
	payload := &ClientPayload{
		ID:      "msg-123",
		Payload: map[string]interface{}{"message": "Hello from client1", "timestamp": time.Now().Unix()},
	}
	publishMsg := CreateTestClientMessage("publish", "test-topic", "client1", "req-3", payload)
	err = client1.SendMessage(publishMsg)
	if err != nil {
		t.Fatalf("Failed to send publish message: %v", err)
	}

	// Wait for publish acknowledgment
	publishAck := client1.WaitForMessageWithRequestID("ack", "req-3", time.Second)
	if publishAck == nil {
		t.Error("Client1 should receive publish acknowledgment")
	} else if publishAck.RequestID != "req-3" {
		t.Errorf("Expected RequestID req-3, got %s", publishAck.RequestID)
	}

	// Both clients should receive the published message
	event1 := client1.WaitForMessage("event", time.Second)
	if event1 == nil {
		t.Error("Client1 should receive the published event")
	} else {
		AssertMessageType(t, event1, "event")
		AssertMessageTopic(t, event1, "test-topic")
		if event1.Message == nil {
			t.Error("Event message should have payload")
		} else if event1.Message.ID != "msg-123" {
			t.Errorf("Expected message ID msg-123, got %s", event1.Message.ID)
		}
	}

	event2 := client2.WaitForMessage("event", time.Second)
	if event2 == nil {
		t.Error("Client2 should receive the published event")
	} else {
		AssertMessageType(t, event2, "event")
		AssertMessageTopic(t, event2, "test-topic")
		if event2.Message == nil {
			t.Error("Event message should have payload")
		} else if event2.Message.ID != "msg-123" {
			t.Errorf("Expected message ID msg-123, got %s", event2.Message.ID)
		}
	}

	// Verify hub stats
	topicMap := hub.ListTopics()
	if subscribers, exists := topicMap["test-topic"]; !exists || subscribers != 2 {
		t.Errorf("Expected 2 subscribers for test-topic, got %d", subscribers)
	}
}

func TestWebSocketIntegrationUnsubscribe(t *testing.T) {
	// Create test server
	server, hub := CreateTestServer()
	defer server.Close()

	// Create test client
	client, err := NewTestClient("client1", server.URL)
	if err != nil {
		t.Fatalf("Failed to create test client: %v", err)
	}
	defer client.Close()

	// Wait for connection to be established
	time.Sleep(50 * time.Millisecond)

	// Subscribe to topic
	subscribeMsg := CreateTestClientMessage("subscribe", "test-topic", "client1", "req-1", nil)
	err = client.SendMessage(subscribeMsg)
	if err != nil {
		t.Fatalf("Failed to send subscribe message: %v", err)
	}

	// Wait for subscription acknowledgment
	ack := client.WaitForMessage("ack", time.Second)
	if ack == nil {
		t.Error("Client should receive subscription acknowledgment")
	}

	// Verify subscription
	topicMap := hub.ListTopics()
	if subscribers, exists := topicMap["test-topic"]; !exists || subscribers != 1 {
		t.Errorf("Expected 1 subscriber for test-topic, got %d", subscribers)
	}

	// Unsubscribe from topic
	unsubscribeMsg := CreateTestClientMessage("unsubscribe", "test-topic", "client1", "req-2", nil)
	err = client.SendMessage(unsubscribeMsg)
	if err != nil {
		t.Fatalf("Failed to send unsubscribe message: %v", err)
	}

	// Wait for unsubscription acknowledgment
	unsubAck := client.WaitForMessageWithRequestID("ack", "req-2", time.Second)
	if unsubAck == nil {
		t.Error("Client should receive unsubscription acknowledgment")
	} else if unsubAck.RequestID != "req-2" {
		t.Errorf("Expected RequestID req-2, got %s", unsubAck.RequestID)
	}

	// Give time for unsubscription to be processed
	time.Sleep(50 * time.Millisecond)

	// Verify unsubscription
	topicMap = hub.ListTopics()
	if _, exists := topicMap["test-topic"]; exists {
		t.Error("test-topic should be removed after all clients unsubscribe")
	}
}

func TestWebSocketIntegrationPingPong(t *testing.T) {
	// Create test server
	server, _ := CreateTestServer()
	defer server.Close()

	// Create test client
	client, err := NewTestClient("client1", server.URL)
	if err != nil {
		t.Fatalf("Failed to create test client: %v", err)
	}
	defer client.Close()

	// Wait for connection to be established
	time.Sleep(50 * time.Millisecond)

	// Send ping message
	pingMsg := CreateTestClientMessage("ping", "", "", "req-ping", nil)
	err = client.SendMessage(pingMsg)
	if err != nil {
		t.Fatalf("Failed to send ping message: %v", err)
	}

	// Wait for pong response
	pong := client.WaitForMessage("pong", time.Second)
	if pong == nil {
		t.Error("Client should receive pong response")
	} else {
		AssertMessageType(t, pong, "pong")
		if pong.RequestID != "req-ping" {
			t.Errorf("Expected RequestID req-ping, got %s", pong.RequestID)
		}
	}
}

func TestWebSocketIntegrationErrorHandling(t *testing.T) {
	// Create test server
	server, _ := CreateTestServer()
	defer server.Close()

	// Create test client
	client, err := NewTestClient("client1", server.URL)
	if err != nil {
		t.Fatalf("Failed to create test client: %v", err)
	}
	defer client.Close()

	// Wait for connection to be established
	time.Sleep(50 * time.Millisecond)

	// Test invalid message type
	invalidMsg := CreateTestClientMessage("invalid-type", "", "", "req-1", nil)
	err = client.SendMessage(invalidMsg)
	if err != nil {
		t.Fatalf("Failed to send invalid message: %v", err)
	}

	// Wait for error response
	errorMsg := client.WaitForMessage("error", time.Second)
	if errorMsg == nil {
		t.Error("Client should receive error response")
	} else {
		AssertMessageType(t, errorMsg, "error")
		if errorMsg.Error == nil {
			t.Error("Error message should have error field")
		} else if errorMsg.Error.Code != "INVALID_MESSAGE_TYPE" {
			t.Errorf("Expected error code INVALID_MESSAGE_TYPE, got %s", errorMsg.Error.Code)
		}
	}

	// Test subscribe without topic
	invalidSubscribe := CreateTestClientMessage("subscribe", "", "client1", "req-2", nil)
	err = client.SendMessage(invalidSubscribe)
	if err != nil {
		t.Fatalf("Failed to send invalid subscribe: %v", err)
	}

	// Wait for error response with specific request ID
	errorMsg2 := client.WaitForMessageWithRequestID("error", "req-2", time.Second)
	if errorMsg2 == nil {
		t.Error("Client should receive error response for invalid subscribe")
	} else {
		AssertMessageType(t, errorMsg2, "error")
		if errorMsg2.Error == nil {
			t.Error("Error message should have error field")
		} else if errorMsg2.Error.Code != "INVALID_TOPIC" {
			t.Errorf("Expected error code INVALID_TOPIC, got %s", errorMsg2.Error.Code)
		}
	}

	// Test publish without message
	invalidPublish := CreateTestClientMessage("publish", "test-topic", "", "req-3", nil)
	invalidPublish.Message = nil // Remove message payload
	err = client.SendMessage(invalidPublish)
	if err != nil {
		t.Fatalf("Failed to send invalid publish: %v", err)
	}

	// Wait for error response with specific request ID
	errorMsg3 := client.WaitForMessageWithRequestID("error", "req-3", time.Second)
	if errorMsg3 == nil {
		t.Error("Client should receive error response for invalid publish")
	} else {
		AssertMessageType(t, errorMsg3, "error")
		if errorMsg3.Error == nil {
			t.Error("Error message should have error field")
		} else if errorMsg3.Error.Code != "INVALID_MESSAGE" {
			t.Errorf("Expected error code INVALID_MESSAGE, got %s", errorMsg3.Error.Code)
		}
	}
}

func TestWebSocketIntegrationMultipleTopics(t *testing.T) {
	// Create test server
	server, hub := CreateTestServer()
	defer server.Close()

	// Create test clients
	client1, err := NewTestClient("client1", server.URL)
	if err != nil {
		t.Fatalf("Failed to create test client1: %v", err)
	}
	defer client1.Close()

	client2, err := NewTestClient("client2", server.URL)
	if err != nil {
		t.Fatalf("Failed to create test client2: %v", err)
	}
	defer client2.Close()

	// Wait for connections to be established
	time.Sleep(50 * time.Millisecond)

	// Subscribe client1 to topic1 and topic2
	subscribeMsg1a := CreateTestClientMessage("subscribe", "topic1", "client1", "req-1a", nil)
	err = client1.SendMessage(subscribeMsg1a)
	if err != nil {
		t.Fatalf("Failed to send subscribe message for client1 topic1: %v", err)
	}

	subscribeMsg1b := CreateTestClientMessage("subscribe", "topic2", "client1", "req-1b", nil)
	err = client1.SendMessage(subscribeMsg1b)
	if err != nil {
		t.Fatalf("Failed to send subscribe message for client1 topic2: %v", err)
	}

	// Subscribe client2 to topic1 only
	subscribeMsg2 := CreateTestClientMessage("subscribe", "topic1", "client2", "req-2", nil)
	err = client2.SendMessage(subscribeMsg2)
	if err != nil {
		t.Fatalf("Failed to send subscribe message for client2: %v", err)
	}

	// Wait for all subscription acknowledgments
	for i := 0; i < 2; i++ {
		if !client1.WaitForMessageCount(i+1, time.Second) {
			t.Errorf("Client1 should have received %d ack messages", i+1)
		}
	}

	if !client2.WaitForMessageCount(1, time.Second) {
		t.Error("Client2 should have received 1 ack message")
	}

	// Publish to topic1 (both clients should receive)
	payload1 := &ClientPayload{
		ID:      "msg-topic1",
		Payload: "Message for topic1",
	}
	publishMsg1 := CreateTestClientMessage("publish", "topic1", "client1", "req-pub1", payload1)
	err = client1.SendMessage(publishMsg1)
	if err != nil {
		t.Fatalf("Failed to send publish message to topic1: %v", err)
	}

	// Publish to topic2 (only client1 should receive)
	payload2 := &ClientPayload{
		ID:      "msg-topic2",
		Payload: "Message for topic2",
	}
	publishMsg2 := CreateTestClientMessage("publish", "topic2", "client1", "req-pub2", payload2)
	err = client1.SendMessage(publishMsg2)
	if err != nil {
		t.Fatalf("Failed to send publish message to topic2: %v", err)
	}

	// Wait for messages to be processed
	time.Sleep(100 * time.Millisecond)

	// Verify received messages
	client1Messages := client1.GetMessages()
	client2Messages := client2.GetMessages()

	// Client1 should have: 2 acks + 1 pub ack + 1 pub ack + 2 events (one from each topic)
	if len(client1Messages) < 6 {
		t.Errorf("Client1 should have at least 6 messages, got %d", len(client1Messages))
	}

	// Client2 should have: 1 ack + 1 event (from topic1 only)
	if len(client2Messages) < 2 {
		t.Errorf("Client2 should have at least 2 messages, got %d", len(client2Messages))
	}

	// Count event messages for each client
	client1Events := 0
	client2Events := 0

	for _, msg := range client1Messages {
		if msg.Type == "event" {
			client1Events++
		}
	}

	for _, msg := range client2Messages {
		if msg.Type == "event" {
			client2Events++
		}
	}

	if client1Events != 2 {
		t.Errorf("Client1 should receive 2 event messages, got %d", client1Events)
	}

	if client2Events != 1 {
		t.Errorf("Client2 should receive 1 event message, got %d", client2Events)
	}

	// Verify hub stats
	topicMap := hub.ListTopics()
	if subscribers, exists := topicMap["topic1"]; !exists || subscribers != 2 {
		t.Errorf("Expected 2 subscribers for topic1, got %d", subscribers)
	}
	if subscribers, exists := topicMap["topic2"]; !exists || subscribers != 1 {
		t.Errorf("Expected 1 subscriber for topic2, got %d", subscribers)
	}
}

func TestWebSocketIntegrationHttpApiPublish(t *testing.T) {
	// Create test server
	server, _ := CreateTestServer()
	defer server.Close()

	// Create test client
	client, err := NewTestClient("client1", server.URL)
	if err != nil {
		t.Fatalf("Failed to create test client: %v", err)
	}
	defer client.Close()

	// Wait for connection to be established
	time.Sleep(50 * time.Millisecond)

	// Subscribe client to topic
	subscribeMsg := CreateTestClientMessage("subscribe", "api-topic", "client1", "req-1", nil)
	err = client.SendMessage(subscribeMsg)
	if err != nil {
		t.Fatalf("Failed to send subscribe message: %v", err)
	}

	// Wait for subscription acknowledgment
	ack := client.WaitForMessage("ack", time.Second)
	if ack == nil {
		t.Error("Client should receive subscription acknowledgment")
	}

	// Publish message via HTTP API
	message := Message{
		Topic:   "api-topic",
		Type:    "event",
		Payload: map[string]interface{}{"source": "http-api", "message": "Hello from HTTP API"},
		Sender:  "api",
	}

	messageJSON, _ := json.Marshal(message)
	resp, body := HTTPTestRequest(t, "POST", server.URL+"/publish", string(messageJSON))

	// Check HTTP response
	if resp.StatusCode != 200 {
		t.Errorf("Expected status code 200, got %d", resp.StatusCode)
	}

	// Client should receive the published message
	event := client.WaitForMessage("event", time.Second)
	if event == nil {
		t.Error("Client should receive the published event from HTTP API")
	} else {
		AssertMessageType(t, event, "event")
		AssertMessageTopic(t, event, "api-topic")
		if event.Message == nil {
			t.Error("Event message should have payload")
		} else {
			// The payload should contain the HTTP API message
			payloadMap, ok := event.Message.Payload.(map[string]interface{})
			if !ok {
				t.Error("Event payload should be a map")
			} else if payloadMap["source"] != "http-api" {
				t.Errorf("Expected source 'http-api', got %v", payloadMap["source"])
			}
		}
	}

	_ = body // Suppress unused variable warning
}
