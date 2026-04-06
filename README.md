# Boom - Event-Driven Notification System

A scalable notification system built with Go that processes and delivers messages through multiple channels (SMS, Email, Push). It handles high throughput with priority queues, retries failed deliveries with exponential backoff, and provides real-time metrics.

## Architecture

```
┌──────────┐     ┌──────────────┐     ┌──────────────┐     ┌──────────┐     ┌──────────────┐
│  Client   │────>│   Gin API    │────>│  PostgreSQL  │     │  Worker  │────>│ Webhook.site │
│ (HTTP)    │     │  (Handler)   │     │  (Storage)   │     │(Consumer)│     │  (Provider)  │
└──────────┘     └──────┬───────┘     └──────────────┘     └────┬─────┘     └──────────────┘
                        │                                       │
                        │         ┌──────────────┐              │
                        └────────>│   RabbitMQ    │─────────────┘
                                  │ (Priority Qs) │
                                  └──────────────┘
```

### Sequence Diagram

```
Client                API Handler          PostgreSQL         RabbitMQ            Worker             Provider
  │                       │                    │                  │                  │                   │
  │  POST /notification   │                    │                  │                  │                   │
  │──────────────────────>│                    │                  │                  │                   │
  │                       │  INSERT INTO       │                  │                  │                   │
  │                       │───────────────────>│                  │                  │                   │
  │                       │  OK (id)           │                  │                  │                   │
  │                       │<───────────────────│                  │                  │                   │
  │                       │                    │                  │                  │                   │
  │                       │  Publish(priority) │                  │                  │                   │
  │                       │──────────────────────────────────────>│                  │                   │
  │  202 Accepted {id}    │                    │                  │                  │                   │
  │<──────────────────────│                    │                  │                  │                   │
  │                       │                    │                  │                  │                   │
  │                       │                    │                  │  Consume         │                   │
  │                       │                    │                  │─────────────────>│                   │
  │                       │                    │                  │                  │                   │
  │                       │                    │  UPDATE status   │                  │                   │
  │                       │                    │  = processing    │                  │                   │
  │                       │                    │<─────────────────────────────────────│                   │
  │                       │                    │                  │                  │                   │
  │                       │                    │                  │                  │  POST /webhook    │
  │                       │                    │                  │                  │─────────────────> │
  │                       │                    │                  │                  │  202 accepted     │
  │                       │                    │                  │                  │<───────────────── │
  │                       │                    │                  │                  │                   │
  │                       │                    │  UPDATE status   │                  │                   │
  │                       │                    │  = delivered     │                  │                   │
  │                       │                    │<─────────────────────────────────────│                   │
  │                       │                    │                  │  ACK             │                   │
  │                       │                    │                  │<─────────────────│                   │
```

### Retry Flow (on failure)

```
Worker              Provider           PostgreSQL          RabbitMQ
  │                    │                   │                  │
  │  POST /webhook     │                   │                  │
  │───────────────────>│                   │                  │
  │  ERROR / timeout   │                   │                  │
  │<───────────────────│                   │                  │
  │                    │                   │                  │
  │  wait 1s (backoff) │                   │                  │
  │  retry attempt 2   │                   │                  │
  │───────────────────>│                   │                  │
  │  ERROR             │                   │                  │
  │<───────────────────│                   │                  │
  │                    │                   │                  │
  │  wait 2s (backoff) │                   │                  │
  │  retry attempt 3   │                   │                  │
  │───────────────────>│                   │                  │
  │  ERROR             │                   │                  │
  │<───────────────────│                   │                  │
  │                    │                   │                  │
  │                    │  UPDATE status    │                  │
  │                    │  = failed         │                  │
  │                    │<──────────────────│                  │
  │                    │                   │  NACK -> DLQ     │
  │                    │                   │─────────────────>│
```

## Tech Stack

| Component     | Technology                |
|---------------|---------------------------|
| Language      | Go 1.24                   |
| HTTP          | Gin                       |
| Database      | PostgreSQL 16             |
| Queue         | RabbitMQ 4                |
| Testing       | testify, httptest         |
| Container     | Docker, Docker Compose    |
| CI/CD         | GitHub Actions            |

## Project Structure

```
boom/
├── cmd/
│   └── main.go                  # Application entry point
├── internal/
│   ├── api/
│   │   ├── handler.go           # HTTP handlers
│   │   ├── handler_test.go      # Handler unit tests
│   │   ├── middleware.go         # Correlation ID middleware
│   │   └── model/
│   │       └── notification.go  # Request/response models
│   ├── config/
│   │   └── config.go            # Environment configuration
│   ├── domain/
│   │   └── notification.go      # Domain models
│   ├── provider/
│   │   └── sender.go            # Webhook provider (HTTP client)
│   ├── queue/
│   │   ├── publisher.go         # RabbitMQ publisher
│   │   └── mock_publisher.go    # Publisher mock
│   ├── repository/
│   │   ├── notification.go      # PostgreSQL repository
│   │   ├── notification_test.go # Repository unit tests
│   │   └── mock_notification_repository.go
│   └── worker/
│       └── consumer.go          # Queue consumer with retry logic
├── migrations/
│   ├── 001_init.up.sql
│   ├── 001_init.down.sql
│   ├── 002_add_provider_msg_id.up.sql
│   └── 002_add_provider_msg_id.down.sql
├── tests/
│   └── benchmark/
│       └── benchmark_test.go    # Load tests with real DB & queue
├── docs/
│   └── swagger.yaml             # OpenAPI 3.0 specification
├── docker-compose.yml
├── Dockerfile
├── Makefile
└── .github/workflows/ci.yml
```

## Quick Start

### One-command setup

```bash
docker-compose up
```

This starts PostgreSQL, RabbitMQ, and the Boom service. API is available at `http://localhost:8080`.

### Local development

```bash
# Start dependencies
docker-compose up postgres rabbitmq -d

# Run the application
WEBHOOK_URL=https://webhook.site/<your-uuid> make run
```

### Environment Variables

| Variable       | Default                                                              | Description              |
|----------------|----------------------------------------------------------------------|--------------------------|
| `DATABASE_URL` | `postgres://postgres:postgres@localhost:5432/notifications?sslmode=disable` | PostgreSQL connection    |
| `RABBITMQ_URL` | `amqp://guest:guest@localhost:5672/`                                 | RabbitMQ connection      |
| `WEBHOOK_URL`  | `https://webhook.site/test`                                          | Provider endpoint        |
| `PORT`         | `8080`                                                               | HTTP server port         |
| `LOG_LEVEL`    | `info`                                                               | Log level (info/debug)   |

## API Endpoints

| Method  | Endpoint                          | Description                        |
|---------|-----------------------------------|------------------------------------|
| POST    | `/v1/notifications`               | Create a notification              |
| POST    | `/v1/notifications/batch`         | Create batch (up to 1000)          |
| GET     | `/v1/notifications`               | List with filters and pagination   |
| GET     | `/v1/notifications/:id`           | Get by ID                          |
| GET     | `/v1/notifications/batch/:batchId`| Get by batch ID                    |
| PATCH   | `/v1/notifications/:id/cancel`    | Cancel a pending notification      |
| GET     | `/v1/metrics`                     | Real-time metrics                  |
| GET     | `/v1/health`                      | Health check                       |

## API Examples

### Create a notification

```bash
curl -X POST http://localhost:8080/v1/notifications \
  -H "Content-Type: application/json" \
  -d '{
    "priority": "high",
    "recipient": "+905551234567",
    "channel": "sms",
    "content": "Your order has been shipped!",
    "idempotency_key": "550e8400-e29b-41d4-a716-446655440000"
  }'
```

Response (202):
```json
{"id": "8f9ddd2f-2c0c-4dff-9c3f-1899965cdd11"}
```

### Create batch notifications

```bash
curl -X POST http://localhost:8080/v1/notifications/batch \
  -H "Content-Type: application/json" \
  -d '{
    "notifications": [
      {"priority": "high", "recipient": "+905551234567", "channel": "sms", "content": "Flash sale!", "idempotency_key": "id-1"},
      {"priority": "normal", "recipient": "user@test.com", "channel": "email", "content": "Welcome!", "idempotency_key": "id-2"}
    ]
  }'
```

Response (202):
```json
{
  "batch_id": "725f009a-...",
  "notification_ids": ["ced398cb-...", "033f2c59-..."],
  "total": 2
}
```

### List notifications with filters

```bash
curl "http://localhost:8080/v1/notifications?status=delivered&channel=sms&page=1&page_size=10"
```

### Cancel a pending notification

```bash
curl -X PATCH http://localhost:8080/v1/notifications/<id>/cancel
```

### Get metrics

```bash
curl http://localhost:8080/v1/metrics
```

Response:
```json
{
  "notifications": {
    "total": 1500,
    "by_status": {"delivered": 1200, "pending": 250, "failed": 50},
    "by_channel": {"sms": 800, "email": 500, "push": 200}
  },
  "queues": [
    {"queue": "notifications.high", "messages": 0},
    {"queue": "notifications.normal", "messages": 3},
    {"queue": "notifications.low", "messages": 0}
  ]
}
```

## Key Design Decisions

**Priority Queues**: Three separate RabbitMQ queues (`notifications.high`, `notifications.normal`, `notifications.low`) ensure high-priority messages are processed first. Each queue has a dead-letter queue (DLQ) for failed messages.

**Idempotency**: Every notification requires a unique `idempotency_key`. The database enforces this with a unique constraint, and duplicate requests return `409 Conflict`.

**Retry with Exponential Backoff**: Failed deliveries are retried up to 3 times with increasing delays (1s, 2s, 4s). After all retries fail, the message goes to the DLQ and status is set to `failed`.

**Rate Limiting**: Each channel (sms, email, push) is rate-limited to 100 messages per second using a token bucket algorithm.

**Partial Batch Success**: Batch creation processes each notification independently. Successful and failed items are reported separately in the response, so one failure doesn't block the entire batch.

**Correlation ID**: Every request gets a `X-Correlation-ID` header for tracing. Clients can pass their own or one is auto-generated.

## Running Tests

```bash
# Unit tests
go test ./... -v

# With race detector
go test ./... -v -race

# Benchmarks (requires running PostgreSQL and RabbitMQ)
go test ./tests/benchmark/ -bench=. -benchmem
```

## API Documentation

OpenAPI 3.0 spec is available at `docs/swagger.yaml`. View it at [Swagger Editor](https://editor.swagger.io) by pasting the file content.
