# WebSocket Broadcast Server Test Suite

This directory contains comprehensive test suites for the WebSocket broadcast server implementation. The tests are organized into different categories to ensure complete coverage of all functionality.

## Test Files Overview

### 1. `test_utils.go`
Contains utility functions and mock implementations for testing:
- **TestClient**: Simulates real WebSocket clients for integration testing
- **MockWebSocketConnection**: Mock implementation for unit testing
- **Helper functions**: For creating test servers, HTTP requests, and assertions

### 2. `hub_test.go` 
Unit tests for Hub functionality:
- Hub initialization and configuration
- Client registration and unregistration
- Topic subscription and unsubscription
- Message broadcasting
- Topic management (create, delete, list, exists)
- Concurrent operations handling

### 3. `client_test.go`
Unit tests for Client functionality:
- Client message handling (subscribe, unsubscribe, publish, ping)
- Error handling and validation
- Response message generation (ack, error, pong)
- Channel overflow handling

### 4. `http_test.go`
Tests for HTTP API endpoints:
- `/publish` - Message publishing via HTTP
- `/stats` - Server statistics retrieval
- `/topics` - Topic management (POST, GET, DELETE)
- Error handling for invalid requests
- Content-Type and status code validation

### 5. `integration_test.go`
End-to-end integration tests:
- Real WebSocket connections
- Subscribe → Publish → Receive message flow
- Multi-client scenarios
- Multiple topic subscriptions
- Ping/Pong functionality
- Error message propagation
- HTTP API integration with WebSocket clients

## Running Tests

### Quick Start
```bash
# Run all tests
go test -v ./interfaces

# Run specific test file
go test -v ./interfaces -run TestHub
go test -v ./interfaces -run TestClient
go test -v ./interfaces -run TestWebSocketIntegration
```

### Using the Test Runner Script
```bash
# Make the script executable
chmod +x run-tests.sh

# Run all test suites with detailed output
./run-tests.sh
```

The test runner script provides:
- Colored output for better readability
- Individual test suite results
- Overall summary
- Proper exit codes for CI/CD integration

### Test Coverage
```bash
# Generate test coverage report
go test -coverprofile=coverage.out ./interfaces
go tool cover -html=coverage.out -o coverage.html

# View coverage in terminal
go test -cover ./interfaces
```

## Test Categories

### Unit Tests
- **Scope**: Individual functions and methods
- **Isolation**: Uses mocks and doesn't require real connections
- **Focus**: Logic validation, error handling, data structures

### Integration Tests  
- **Scope**: Full system functionality
- **Real connections**: Actual WebSocket connections to test server
- **Focus**: End-to-end workflows, protocol compliance

### HTTP API Tests
- **Scope**: REST endpoints
- **Method**: HTTP requests to test server
- **Focus**: Request/response validation, status codes, JSON formatting

## Key Test Scenarios

### 1. Basic WebSocket Operations
- Client connection and disconnection
- Message sending and receiving
- Protocol compliance (ping/pong)

### 2. Topic Management
- Subscribe to topics
- Unsubscribe from topics
- Automatic cleanup of empty topics
- Topic creation and deletion via API

### 3. Message Broadcasting
- Single topic, multiple subscribers
- Multiple topics per client
- Message routing accuracy
- Payload integrity

### 4. Error Handling
- Invalid message types
- Missing required fields
- Connection failures
- Channel overflow protection

### 5. Concurrent Operations
- Multiple clients connecting simultaneously
- Concurrent subscriptions and publications
- Thread safety validation
- Race condition prevention

### 6. HTTP/WebSocket Integration
- Publishing via HTTP API
- Receiving via WebSocket connections
- Statistics retrieval
- Topic management via REST API

## Test Data Structures

### ClientMessage
```go
{
    "type": "subscribe|unsubscribe|publish|ping",
    "topic": "topic-name",
    "client_id": "client-identifier", 
    "message": {
        "id": "message-id",
        "payload": {...}
    },
    "request_id": "request-identifier"
}
```

### ServerMessage
```go
{
    "type": "ack|event|error|pong|info",
    "request_id": "echoed-request-id",
    "topic": "topic-name",
    "message": {
        "id": "message-id", 
        "payload": {...}
    },
    "error": {
        "code": "ERROR_CODE",
        "message": "Error description"
    },
    "ts": "2024-01-01T00:00:00Z"
}
```

## Assertions and Validations

The test suite includes helper functions for common validations:
- **AssertMessageType**: Validates message type
- **AssertMessageTopic**: Validates message topic
- **HTTPTestRequest**: Makes HTTP requests with proper headers
- **WaitForMessage**: Waits for specific message types with timeout

## Adding New Tests

### For New Features
1. Add unit tests in appropriate `*_test.go` file
2. Add integration test if feature involves multiple components
3. Update this README with new test scenarios

### Test Naming Convention
- `Test[Component][Feature]` - e.g., `TestHubClientRegistration`
- `Test[Component][Feature][Scenario]` - e.g., `TestClientHandleInvalidMessage`

### Best Practices
- Use table-driven tests for multiple similar scenarios
- Include both positive and negative test cases
- Test error conditions and edge cases
- Use meaningful test data and assertions
- Clean up resources (close connections, stop servers)

## Troubleshooting

### Common Issues
1. **Tests timeout**: Increase timeout values or check for deadlocks
2. **Port conflicts**: Tests use random ports via `httptest.NewServer()`
3. **Race conditions**: Ensure proper synchronization with channels and mutexes
4. **Resource leaks**: Always close clients and servers in defer statements

### Debug Tips
- Add `time.Sleep()` for timing-sensitive tests
- Use `t.Logf()` for debug output
- Check hub statistics during tests
- Verify message counts and types

## CI/CD Integration

The test suite is designed for automated testing:
- Proper exit codes (0 for success, 1 for failure)
- Structured output for parsing
- Timeout handling
- No external dependencies

Example GitHub Actions workflow:
```yaml
- name: Run Tests
  run: |
    chmod +x run-tests.sh
    ./run-tests.sh
```