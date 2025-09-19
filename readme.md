
# WebSocket Backend

A Go-based WebSocket server with Redis support for real-time messaging and topic-based broadcasting.

## Features

- WebSocket connections with topic-based subscriptions
- Redis integration for distributed messaging
- REST API for publishing messages and getting statistics
- CORS support
- Health checks

## Docker Usage

### Build and Run with Docker Compose (Recommended)

This will start both the WebSocket server and Redis:

```bash
docker-compose up --build
```

### Build and Run with Docker

1. Build the Docker image:
```bash
docker build -t websocket-backend .
```

2. Run with Redis (for distributed messaging):
```bash
docker run -p 8080:8080 --link redis:redis websocket-backend
```

3. Run without Redis (single server mode):
```bash
docker run -p 8080:8080 websocket-backend
```

## API Endpoints

- `GET /ws` - WebSocket connection endpoint
- `POST /publish` - Publish a message to a topic
- `GET /stats` - Get server statistics

## WebSocket Message Format

### Subscribe to a topic:
```json
{
  "type": "subscribe",
  "topic": "my-topic"
}
```

### Unsubscribe from a topic:
```json
{
  "type": "unsubscribe",
  "topic": "my-topic"
}
```

### Publish a message:
```json
{
  "type": "publish",
  "topic": "my-topic",
  "payload": "Hello World!"
}
```

## Environment Variables

- `REDIS_ADDR` - Redis server address (optional, defaults to no Redis)

## Ports

- `8080` - HTTP/WebSocket server
- `6379` - Redis server (when using docker-compose)

## Code Assumptions and Design Decisions

This section documents all assumptions and design decisions made in the WebSocket broadcast server implementation.

### 1. WebSocket Connection Management

**Assumptions:**
- WebSocket connections will use the Gorilla WebSocket library upgrader
- All origins are allowed for CORS (`CheckOrigin: true`) - **Not suitable for production**
- Read buffer size: 1024 bytes, Write buffer size: 1024 bytes
- Connection read timeout: 60 seconds with automatic extension on pong
- Connection write timeout: 10 seconds per message
- Ping interval: 54 seconds (to stay under 60-second timeout)

**Rationale:** These timeouts provide reasonable defaults for detecting dead connections while allowing for network latency.

### 2. Client Message Queue Configuration

**Default Settings:**
- **Queue size per client:** 256 messages
- **Backpressure strategy:** Disconnect client when queue is full
- **Alternative strategy:** Drop oldest message when queue is full

**Assumptions:**
- Normal clients can consume messages faster than they're produced
- 256 message buffer is sufficient for typical network delays and client processing
- Slow consumers should be disconnected rather than affecting system performance
- Client channels use Go's buffered channels for non-blocking sends

**Rationale:** Bounded queues prevent memory exhaustion from slow consumers while maintaining system responsiveness.

### 3. Graceful Shutdown Behavior

**Assumptions:**
- **Default shutdown timeout:** 30 seconds
- Clients can disconnect gracefully when notified
- New connections should be rejected during shutdown
- In-flight operations should complete before forced termination
- All clients will receive shutdown notification via info message

**Rationale:** 30 seconds provides reasonable time for clients to save state and disconnect cleanly.

### 4. Topic Management

**Assumptions:**
- Topics are created automatically when first client subscribes
- Empty topics are automatically cleaned up when last client unsubscribes
- Topic names are case-sensitive strings
- No topic name validation or restrictions (allows any non-empty string)
- Topics can be manually created via REST API without requiring subscribers
- Deleting a topic force-unsubscribes all clients and sends info message

**Rationale:** Automatic lifecycle management reduces administrative overhead while manual API provides explicit control when needed.

### 5. Message Format and Protocol

**Client Message Types:**
- `subscribe` - requires: topic, client_id
- `unsubscribe` - requires: topic, client_id  
- `publish` - requires: topic, message
- `ping` - optional: request_id

**Server Message Types:**
- `ack` - acknowledgment of successful operations
- `error` - error responses with code and message
- `event` - broadcast messages to subscribers
- `pong` - response to ping
- `info` - informational messages (e.g., shutdown, topic deletion)

**Assumptions:**
- All messages are JSON-formatted
- Client IDs are provided by clients (not generated server-side)
- Request IDs are optional and echoed back when provided
- Timestamps use RFC3339 format in UTC
- Message payloads are flexible JSON (interface{})
- Unique message IDs are generated using nanosecond timestamps

**Rationale:** JSON provides language-agnostic messaging, flexible payload supports various use cases.

### 6. Error Handling Strategy

**Error Codes:**
- `INVALID_TOPIC` - missing or empty topic name
- `INVALID_CLIENT_ID` - missing client ID for subscribe/unsubscribe
- `INVALID_MESSAGE` - missing message payload for publish
- `INVALID_MESSAGE_TYPE` - unknown message type
- `TOPIC_NOT_FOUND` - attempting to publish to non-existent topic
- `SLOW_CONSUMER` - client queue overflow, connection will be terminated

**Assumptions:**
- Clients handle error responses appropriately
- Invalid messages don't crash the connection (send error and continue)
- Only slow consumer errors result in disconnection
- Best-effort delivery for error messages (may be dropped if queue full)

### 7. Concurrency and Thread Safety

**Assumptions:**
- High concurrent load with multiple goroutines per client (read/write pumps)
- Hub operations are serialized through channels to avoid race conditions
- Map access requires mutex protection for statistics and topic management
- Client registration/unregistration is thread-safe via channels
- Broadcast operations don't block the main hub loop

**Design Pattern:** Actor model using channels for message passing between goroutines.

### 8. Performance and Scalability

**Assumptions:**
- **Broadcast channel buffer:** 256 messages for hub-level operations
- Memory usage bounded by: (number_of_clients × queue_size × average_message_size)
- CPU usage scales with message throughput and client count
- Network I/O is the primary bottleneck, not CPU or memory
- Go's garbage collector can handle frequent small message allocations

**Performance Characteristics:**
- Supports thousands of concurrent connections
- Low-latency message delivery via channel-based routing
- High throughput via non-blocking broadcast operations

### 9. Production Deployment Assumptions

**Docker Environment:**
- **Go version:** 1.21 (as specified in go.mod and Dockerfile)
- **Base image:** Alpine Linux for minimal attack surface
- **User:** Non-root user (appuser:appgroup, UID/GID 1001)
- **Health check:** HTTP GET to `/stats` endpoint every 30 seconds
- **Port exposure:** Only 8080 (HTTP/WebSocket)

**Security Assumptions:**
- Application runs as non-root user in container
- No sensitive data in environment variables (except optional Redis address)
- CORS allows all origins (**SECURITY RISK** - should be restricted in production)
- No authentication or authorization implemented (**REQUIRES ADDITION** for production)

### 10. Network and Infrastructure

**Assumptions:**
- **Load balancer:** If used, must support WebSocket sticky sessions
- **Firewall:** Port 8080 accessible for client connections
- **Reverse proxy:** If used, must properly forward WebSocket upgrade headers
- **Monitoring:** Health check endpoint (`/stats`) provides basic monitoring data

### 11. Error Recovery and Fault Tolerance

**Assumptions:**
- Network interruptions will trigger WebSocket close events
- Clients will reconnect automatically after connection loss
- No message persistence - messages lost if no active subscribers
- No message replay capability for reconnecting clients
- Hub restart loses all connection state (clients must reconnect)

**Limitations:**
- No built-in message queuing or persistence
- No automatic client reconnection logic
- No cluster coordination for multi-instance deployments

### 12. Testing Environment Assumptions

**Test Configuration:**
- Uses `httptest.NewServer()` for integration tests (random ports)
- Mock WebSocket connections for unit tests
- Test timeouts generally set to 5-10 seconds
- Tests assume clean state between runs
- No external dependencies required for testing

### 13. Configuration and Customization

**Assumptions:**
- Configuration is set at startup time (no runtime reconfiguration)
- Default values are suitable for typical development/testing scenarios
- Production deployments will customize queue sizes and timeouts
- No configuration file support - all configuration via code or environment

**Customizable Parameters:**
```go
type HubConfig struct {
    MaxQueueSize         int                  // Default: 256
    BackpressureStrategy BackpressureStrategy // Default: DisconnectClient
    ShutdownTimeout      time.Duration        // Default: 30 seconds
}
```

### 14. Backward Compatibility

**Assumptions:**
- Message format is stable and won't break existing clients
- Protocol version is implicit (no explicit versioning)
- New features added with optional fields to maintain compatibility
- Legacy message types supported for internal hub communication

### Summary

These assumptions reflect design decisions optimized for:
- **Simplicity:** Easy to understand and deploy
- **Performance:** Low latency, high throughput
- **Reliability:** Graceful handling of slow consumers and failures  
- **Development:** Comprehensive testing and debugging support

