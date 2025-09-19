
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
