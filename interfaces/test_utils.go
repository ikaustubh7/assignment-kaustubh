package interfaces

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

// WebSocketConnection interface for testing
type WebSocketConnection interface {
	WriteJSON(v interface{}) error
	ReadJSON(v interface{}) error
	Close() error
	SetReadDeadline(t time.Time) error
	SetWriteDeadline(t time.Time) error
	SetReadLimit(limit int64)
	SetPongHandler(h func(appData string) error)
	WriteMessage(messageType int, data []byte) error
}

// TestClient represents a test WebSocket client
type TestClient struct {
	ID       string
	Conn     *websocket.Conn
	Messages []ServerMessage
	Done     chan bool
	mutex    sync.RWMutex
}

// NewTestClient creates a new test client
func NewTestClient(id string, serverURL string) (*TestClient, error) {
	u, err := url.Parse(serverURL)
	if err != nil {
		return nil, err
	}
	u.Scheme = "ws"
	u.Path = "/ws"

	conn, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		return nil, err
	}

	client := &TestClient{
		ID:       id,
		Conn:     conn,
		Messages: make([]ServerMessage, 0),
		Done:     make(chan bool),
	}

	go client.readMessages()
	return client, nil
}

// readMessages continuously reads messages from the WebSocket
func (tc *TestClient) readMessages() {
	defer func() {
		tc.Done <- true
	}()

	for {
		var msg ServerMessage
		err := tc.Conn.ReadJSON(&msg)
		if err != nil {
			return
		}

		tc.mutex.Lock()
		tc.Messages = append(tc.Messages, msg)
		tc.mutex.Unlock()
	}
}

// SendMessage sends a message to the server
func (tc *TestClient) SendMessage(msg ClientMessage) error {
	return tc.Conn.WriteJSON(msg)
}

// GetMessages returns all received messages
func (tc *TestClient) GetMessages() []ServerMessage {
	tc.mutex.RLock()
	defer tc.mutex.RUnlock()
	return append([]ServerMessage{}, tc.Messages...)
}

// GetLastMessage returns the last received message
func (tc *TestClient) GetLastMessage() *ServerMessage {
	tc.mutex.RLock()
	defer tc.mutex.RUnlock()
	if len(tc.Messages) == 0 {
		return nil
	}
	return &tc.Messages[len(tc.Messages)-1]
}

// WaitForMessage waits for a message of specific type
func (tc *TestClient) WaitForMessage(msgType string, timeout time.Duration) *ServerMessage {
	start := time.Now()
	for time.Since(start) < timeout {
		tc.mutex.RLock()
		for _, msg := range tc.Messages {
			if msg.Type == msgType {
				tc.mutex.RUnlock()
				return &msg
			}
		}
		tc.mutex.RUnlock()
		time.Sleep(10 * time.Millisecond)
	}
	return nil
}

// WaitForMessageCount waits for a specific number of messages
func (tc *TestClient) WaitForMessageCount(count int, timeout time.Duration) bool {
	start := time.Now()
	for time.Since(start) < timeout {
		tc.mutex.RLock()
		if len(tc.Messages) >= count {
			tc.mutex.RUnlock()
			return true
		}
		tc.mutex.RUnlock()
		time.Sleep(10 * time.Millisecond)
	}
	return false
}

// Close closes the test client connection
func (tc *TestClient) Close() {
	tc.Conn.Close()
	<-tc.Done
}

// MockWebSocketConnection creates a mock WebSocket connection for unit tests
type MockWebSocketConnection struct {
	MessagesSent []interface{}
	messageChan  chan interface{}
	closed       bool
	mutex        sync.RWMutex
}

// NewMockWebSocketConnection creates a new mock connection
func NewMockWebSocketConnection() *MockWebSocketConnection {
	return &MockWebSocketConnection{
		MessagesSent: make([]interface{}, 0),
		messageChan:  make(chan interface{}, 100),
	}
}

// WriteJSON mocks writing JSON to WebSocket
func (m *MockWebSocketConnection) WriteJSON(v interface{}) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.MessagesSent = append(m.MessagesSent, v)
	return nil
}

// ReadJSON mocks reading JSON from WebSocket
func (m *MockWebSocketConnection) ReadJSON(v interface{}) error {
	select {
	case msg := <-m.messageChan:
		data, _ := json.Marshal(msg)
		return json.Unmarshal(data, v)
	case <-time.After(100 * time.Millisecond):
		return websocket.ErrCloseSent
	}
}

// SendMockMessage sends a mock message to be read
func (m *MockWebSocketConnection) SendMockMessage(msg interface{}) {
	m.messageChan <- msg
}

// Close mocks closing the connection
func (m *MockWebSocketConnection) Close() error {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.closed = true
	close(m.messageChan)
	return nil
}

// SetReadDeadline mocks setting read deadline
func (m *MockWebSocketConnection) SetReadDeadline(t time.Time) error {
	return nil
}

// SetWriteDeadline mocks setting write deadline
func (m *MockWebSocketConnection) SetWriteDeadline(t time.Time) error {
	return nil
}

// SetReadLimit mocks setting read limit
func (m *MockWebSocketConnection) SetReadLimit(limit int64) {
}

// SetPongHandler mocks setting pong handler
func (m *MockWebSocketConnection) SetPongHandler(h func(appData string) error) {
}

// WriteMessage mocks writing message
func (m *MockWebSocketConnection) WriteMessage(messageType int, data []byte) error {
	return nil
}

// GetSentMessages returns all sent messages
func (m *MockWebSocketConnection) GetSentMessages() []interface{} {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	return append([]interface{}{}, m.MessagesSent...)
}

// TestServer creates a test HTTP server with WebSocket support
func CreateTestServer() (*httptest.Server, *Hub) {
	hub := NewHub("")
	go hub.Run()

	mux := http.NewServeMux()
	mux.HandleFunc("/ws", hub.HandleWebSocket)
	mux.HandleFunc("/publish", hub.HandlePublish)
	mux.HandleFunc("/stats", hub.HandleStats)
	mux.HandleFunc("/topics", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case "POST":
			hub.HandleCreateTopic(w, r)
		case "GET":
			hub.HandleListTopics(w, r)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})
	mux.HandleFunc("/topics/", hub.HandleDeleteTopic)

	server := httptest.NewServer(mux)
	return server, hub
}

// HTTPTestRequest is a helper for making HTTP requests in tests
func HTTPTestRequest(t *testing.T, method, url string, body string) (*http.Response, string) {
	req, err := http.NewRequest(method, url, strings.NewReader(body))
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}

	if method == "POST" {
		req.Header.Set("Content-Type", "application/json")
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("Failed to make request: %v", err)
	}
	defer resp.Body.Close()

	responseBody := make([]byte, 0)
	buffer := make([]byte, 1024)
	for {
		n, err := resp.Body.Read(buffer)
		if n > 0 {
			responseBody = append(responseBody, buffer[:n]...)
		}
		if err != nil {
			break
		}
	}

	return resp, string(responseBody)
}

// AssertMessageType checks if a message has the expected type
func AssertMessageType(t *testing.T, msg *ServerMessage, expectedType string) {
	if msg == nil {
		t.Errorf("Expected message of type %s, but got nil", expectedType)
		return
	}
	if msg.Type != expectedType {
		t.Errorf("Expected message type %s, got %s", expectedType, msg.Type)
	}
}

// AssertMessageTopic checks if a message has the expected topic
func AssertMessageTopic(t *testing.T, msg *ServerMessage, expectedTopic string) {
	if msg == nil {
		t.Errorf("Expected message with topic %s, but got nil", expectedTopic)
		return
	}
	if msg.Topic != expectedTopic {
		t.Errorf("Expected message topic %s, got %s", expectedTopic, msg.Topic)
	}
}

// CreateTestClientMessage creates a test client message
func CreateTestClientMessage(msgType, topic, clientID, requestID string, payload interface{}) ClientMessage {
	msg := ClientMessage{
		Type:      msgType,
		Topic:     topic,
		ClientID:  clientID,
		RequestID: requestID,
	}

	if payload != nil {
		if cp, ok := payload.(*ClientPayload); ok {
			msg.Message = cp
		} else {
			msg.Message = &ClientPayload{
				ID:      "test-id",
				Payload: payload,
			}
		}
	}

	return msg
}
