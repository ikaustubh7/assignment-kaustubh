package interfaces

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHubHandlePublish(t *testing.T) {
	hub := NewHub("")
	go hub.Run() // Start hub to handle broadcast messages

	// Test valid publish request
	message := Message{
		Topic:   "test-topic",
		Type:    "event",
		Payload: map[string]interface{}{"test": "data"},
		Sender:  "api",
	}

	messageJSON, _ := json.Marshal(message)
	req, _ := http.NewRequest("POST", "/publish", strings.NewReader(string(messageJSON)))
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(hub.HandlePublish)
	handler.ServeHTTP(rr, req)

	// Check response status
	if status := rr.Code; status != http.StatusOK {
		t.Errorf("Expected status code %d, got %d", http.StatusOK, status)
	}

	// Check response content type
	if contentType := rr.Header().Get("Content-Type"); contentType != "application/json" {
		t.Errorf("Expected Content-Type application/json, got %s", contentType)
	}

	// Check response body
	var response map[string]string
	err := json.Unmarshal(rr.Body.Bytes(), &response)
	if err != nil {
		t.Errorf("Failed to parse response JSON: %v", err)
	}

	if response["status"] != "published" {
		t.Errorf("Expected status 'published', got %s", response["status"])
	}
}

func TestHubHandlePublishInvalidMethod(t *testing.T) {
	hub := NewHub("")

	// Test GET request (should be POST only)
	req, _ := http.NewRequest("GET", "/publish", nil)
	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(hub.HandlePublish)
	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusMethodNotAllowed {
		t.Errorf("Expected status code %d, got %d", http.StatusMethodNotAllowed, status)
	}
}

func TestHubHandlePublishInvalidJSON(t *testing.T) {
	hub := NewHub("")

	// Test invalid JSON
	req, _ := http.NewRequest("POST", "/publish", strings.NewReader("invalid json"))
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(hub.HandlePublish)
	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusBadRequest {
		t.Errorf("Expected status code %d, got %d", http.StatusBadRequest, status)
	}
}

func TestHubHandleStats(t *testing.T) {
	hub := NewHub("")

	// Add some clients and topics for testing
	client1 := &Client{
		ID:     "client1",
		Conn:   nil,
		Topics: make(map[string]bool),
		Send:   make(chan ServerMessage, 256),
		Hub:    hub,
	}
	client2 := &Client{
		ID:     "client2",
		Conn:   nil,
		Topics: make(map[string]bool),
		Send:   make(chan ServerMessage, 256),
		Hub:    hub,
	}

	hub.registerClient(client1)
	hub.registerClient(client2)
	hub.subscribeToTopic(client1, "topic1")
	hub.subscribeToTopic(client2, "topic1")
	hub.subscribeToTopic(client2, "topic2")

	// Test stats request
	req, _ := http.NewRequest("GET", "/stats", nil)
	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(hub.HandleStats)
	handler.ServeHTTP(rr, req)

	// Check response status
	if status := rr.Code; status != http.StatusOK {
		t.Errorf("Expected status code %d, got %d", http.StatusOK, status)
	}

	// Check response content type
	if contentType := rr.Header().Get("Content-Type"); contentType != "application/json" {
		t.Errorf("Expected Content-Type application/json, got %s", contentType)
	}

	// Check response body
	var stats map[string]interface{}
	err := json.Unmarshal(rr.Body.Bytes(), &stats)
	if err != nil {
		t.Errorf("Failed to parse response JSON: %v", err)
	}

	if totalClients, ok := stats["total_clients"].(float64); !ok || int(totalClients) != 2 {
		t.Errorf("Expected total_clients 2, got %v", stats["total_clients"])
	}

	if totalTopics, ok := stats["total_topics"].(float64); !ok || int(totalTopics) != 2 {
		t.Errorf("Expected total_topics 2, got %v", stats["total_topics"])
	}

	if topics, ok := stats["topics"].(map[string]interface{}); ok {
		if topic1Count, ok := topics["topic1"].(float64); !ok || int(topic1Count) != 2 {
			t.Errorf("Expected topic1 to have 2 subscribers, got %v", topics["topic1"])
		}
		if topic2Count, ok := topics["topic2"].(float64); !ok || int(topic2Count) != 1 {
			t.Errorf("Expected topic2 to have 1 subscriber, got %v", topics["topic2"])
		}
	} else {
		t.Error("Expected topics field in stats response")
	}
}

func TestHubHandleCreateTopic(t *testing.T) {
	hub := NewHub("")

	// Test valid create topic request
	reqBody := `{"name":"new-topic"}`
	req, _ := http.NewRequest("POST", "/topics", strings.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(hub.HandleCreateTopic)
	handler.ServeHTTP(rr, req)

	// Check response status
	if status := rr.Code; status != http.StatusCreated {
		t.Errorf("Expected status code %d, got %d", http.StatusCreated, status)
	}

	// Check response content type
	if contentType := rr.Header().Get("Content-Type"); contentType != "application/json" {
		t.Errorf("Expected Content-Type application/json, got %s", contentType)
	}

	// Check response body
	var response map[string]string
	err := json.Unmarshal(rr.Body.Bytes(), &response)
	if err != nil {
		t.Errorf("Failed to parse response JSON: %v", err)
	}

	if response["status"] != "created" {
		t.Errorf("Expected status 'created', got %s", response["status"])
	}
	if response["topic"] != "new-topic" {
		t.Errorf("Expected topic 'new-topic', got %s", response["topic"])
	}

	// Verify topic was actually created
	if !hub.TopicExists("new-topic") {
		t.Error("Topic should have been created")
	}
}

func TestHubHandleCreateTopicInvalidMethod(t *testing.T) {
	hub := NewHub("")

	// Test GET request (should be POST only)
	req, _ := http.NewRequest("GET", "/topics", nil)
	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(hub.HandleCreateTopic)
	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusMethodNotAllowed {
		t.Errorf("Expected status code %d, got %d", http.StatusMethodNotAllowed, status)
	}
}

func TestHubHandleCreateTopicInvalidJSON(t *testing.T) {
	hub := NewHub("")

	// Test invalid JSON
	req, _ := http.NewRequest("POST", "/topics", strings.NewReader("invalid json"))
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(hub.HandleCreateTopic)
	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusBadRequest {
		t.Errorf("Expected status code %d, got %d", http.StatusBadRequest, status)
	}
}

func TestHubHandleCreateTopicEmptyName(t *testing.T) {
	hub := NewHub("")

	// Test empty topic name
	reqBody := `{"name":""}`
	req, _ := http.NewRequest("POST", "/topics", strings.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(hub.HandleCreateTopic)
	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusBadRequest {
		t.Errorf("Expected status code %d, got %d", http.StatusBadRequest, status)
	}
}

func TestHubHandleCreateTopicAlreadyExists(t *testing.T) {
	hub := NewHub("")

	// Create topic first
	hub.CreateTopic("existing-topic")

	// Try to create the same topic again
	reqBody := `{"name":"existing-topic"}`
	req, _ := http.NewRequest("POST", "/topics", strings.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(hub.HandleCreateTopic)
	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusConflict {
		t.Errorf("Expected status code %d, got %d", http.StatusConflict, status)
	}

	// Check response body
	var response map[string]string
	err := json.Unmarshal(rr.Body.Bytes(), &response)
	if err != nil {
		t.Errorf("Failed to parse response JSON: %v", err)
	}

	if response["error"] != "Topic already exists" {
		t.Errorf("Expected error 'Topic already exists', got %s", response["error"])
	}
}

func TestHubHandleDeleteTopic(t *testing.T) {
	hub := NewHub("")

	// Create topic first
	hub.CreateTopic("delete-me")

	// Test valid delete topic request
	req, _ := http.NewRequest("DELETE", "/topics/delete-me", nil)
	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(hub.HandleDeleteTopic)
	handler.ServeHTTP(rr, req)

	// Check response status
	if status := rr.Code; status != http.StatusOK {
		t.Errorf("Expected status code %d, got %d", http.StatusOK, status)
	}

	// Check response content type
	if contentType := rr.Header().Get("Content-Type"); contentType != "application/json" {
		t.Errorf("Expected Content-Type application/json, got %s", contentType)
	}

	// Check response body
	var response map[string]string
	err := json.Unmarshal(rr.Body.Bytes(), &response)
	if err != nil {
		t.Errorf("Failed to parse response JSON: %v", err)
	}

	if response["status"] != "deleted" {
		t.Errorf("Expected status 'deleted', got %s", response["status"])
	}
	if response["topic"] != "delete-me" {
		t.Errorf("Expected topic 'delete-me', got %s", response["topic"])
	}

	// Verify topic was actually deleted
	if hub.TopicExists("delete-me") {
		t.Error("Topic should have been deleted")
	}
}

func TestHubHandleDeleteTopicInvalidMethod(t *testing.T) {
	hub := NewHub("")

	// Test POST request (should be DELETE only)
	req, _ := http.NewRequest("POST", "/topics/test-topic", nil)
	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(hub.HandleDeleteTopic)
	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusMethodNotAllowed {
		t.Errorf("Expected status code %d, got %d", http.StatusMethodNotAllowed, status)
	}
}

func TestHubHandleDeleteTopicInvalidPath(t *testing.T) {
	hub := NewHub("")

	// Test invalid path (no topic name)
	req, _ := http.NewRequest("DELETE", "/topics/", nil)
	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(hub.HandleDeleteTopic)
	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusBadRequest {
		t.Errorf("Expected status code %d, got %d", http.StatusBadRequest, status)
	}
}

func TestHubHandleDeleteTopicNotFound(t *testing.T) {
	hub := NewHub("")

	// Try to delete non-existing topic
	req, _ := http.NewRequest("DELETE", "/topics/non-existing", nil)
	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(hub.HandleDeleteTopic)
	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusNotFound {
		t.Errorf("Expected status code %d, got %d", http.StatusNotFound, status)
	}

	// Check response body
	var response map[string]string
	err := json.Unmarshal(rr.Body.Bytes(), &response)
	if err != nil {
		t.Errorf("Failed to parse response JSON: %v", err)
	}

	if response["error"] != "Topic not found" {
		t.Errorf("Expected error 'Topic not found', got %s", response["error"])
	}
}

func TestHubHandleListTopics(t *testing.T) {
	hub := NewHub("")

	// Add some topics with subscribers
	client1 := &Client{
		ID:     "client1",
		Conn:   nil,
		Topics: make(map[string]bool),
		Send:   make(chan ServerMessage, 256),
		Hub:    hub,
	}
	client2 := &Client{
		ID:     "client2",
		Conn:   nil,
		Topics: make(map[string]bool),
		Send:   make(chan ServerMessage, 256),
		Hub:    hub,
	}

	hub.registerClient(client1)
	hub.registerClient(client2)
	hub.subscribeToTopic(client1, "topic1")
	hub.subscribeToTopic(client2, "topic1")
	hub.subscribeToTopic(client2, "topic2")

	// Test list topics request
	req, _ := http.NewRequest("GET", "/topics", nil)
	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(hub.HandleListTopics)
	handler.ServeHTTP(rr, req)

	// Check response status
	if status := rr.Code; status != http.StatusOK {
		t.Errorf("Expected status code %d, got %d", http.StatusOK, status)
	}

	// Check response content type
	if contentType := rr.Header().Get("Content-Type"); contentType != "application/json" {
		t.Errorf("Expected Content-Type application/json, got %s", contentType)
	}

	// Check response body
	var response map[string]interface{}
	err := json.Unmarshal(rr.Body.Bytes(), &response)
	if err != nil {
		t.Errorf("Failed to parse response JSON: %v", err)
	}

	if count, ok := response["count"].(float64); !ok || int(count) != 2 {
		t.Errorf("Expected count 2, got %v", response["count"])
	}

	if topics, ok := response["topics"].(map[string]interface{}); ok {
		if topic1Count, ok := topics["topic1"].(float64); !ok || int(topic1Count) != 2 {
			t.Errorf("Expected topic1 to have 2 subscribers, got %v", topics["topic1"])
		}
		if topic2Count, ok := topics["topic2"].(float64); !ok || int(topic2Count) != 1 {
			t.Errorf("Expected topic2 to have 1 subscriber, got %v", topics["topic2"])
		}
	} else {
		t.Error("Expected topics field in response")
	}
}

func TestHubHandleListTopicsInvalidMethod(t *testing.T) {
	hub := NewHub("")

	// Test POST request (should be GET only)
	req, _ := http.NewRequest("POST", "/topics", nil)
	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(hub.HandleListTopics)
	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusMethodNotAllowed {
		t.Errorf("Expected status code %d, got %d", http.StatusMethodNotAllowed, status)
	}
}

func TestHubHandleListTopicsEmpty(t *testing.T) {
	hub := NewHub("")

	// Test list topics with no topics
	req, _ := http.NewRequest("GET", "/topics", nil)
	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(hub.HandleListTopics)
	handler.ServeHTTP(rr, req)

	// Check response status
	if status := rr.Code; status != http.StatusOK {
		t.Errorf("Expected status code %d, got %d", http.StatusOK, status)
	}

	// Check response body
	var response map[string]interface{}
	err := json.Unmarshal(rr.Body.Bytes(), &response)
	if err != nil {
		t.Errorf("Failed to parse response JSON: %v", err)
	}

	if count, ok := response["count"].(float64); !ok || int(count) != 0 {
		t.Errorf("Expected count 0, got %v", response["count"])
	}

	if topics, ok := response["topics"].(map[string]interface{}); !ok || len(topics) != 0 {
		t.Errorf("Expected empty topics map, got %v", response["topics"])
	}
}
