# Fish Fry Orders

A full-stack order management system built for parish fish fry events. It handles real-time order tracking, menu management, session-based daily operations, and historical reporting.

## Architecture

- **Backend**: Go (Echo framework, PostgreSQL, WebSocket for real-time updates, gRPC/protobuf definitions)
- **Frontend**: React + TypeScript (Vite, deployed via Nginx)
- **Infrastructure**: Docker, Helm charts for Kubernetes deployment (Istio VirtualService)

## Features

- **Session-based operations** -- sessions auto-create on first order and expire at end of day; admins can extend or close sessions manually
- **Real-time order tracking** via WebSocket -- all connected clients see order status changes instantly
- **Menu management** -- admins can create, update, reorder, and deactivate menu items
- **Price capture at order time** -- order items snapshot the menu item name and price, so historical data stays accurate even if prices change
- **Role-based auth** -- worker and admin roles with password-based login and session cookies
- **Historical session comparison** -- compare order counts, revenue, and item breakdowns across past sessions
- **Daily order numbers** -- human-friendly order numbers that reset per session (e.g. "Order #7")

## Project Structure

```
cmd/
  server/         # Main HTTP/WebSocket server entrypoint
  migrate/        # Database migration CLI tool
internal/
  api/            # HTTP handlers (orders, menu, sessions)
  auth/           # Authentication service and middleware
  config/         # YAML config loading
  database/       # PostgreSQL repository, connection pooling, migrations
  logger/         # Structured logging
  metrics/        # Prometheus metrics
  types/          # Shared DB types and proto conversions
proto/            # Protobuf definitions and generated Go code
frontend/         # React + TypeScript SPA
testing/          # Integration tests with Docker Compose-managed Postgres
helm/             # Helm chart with per-environment value overrides
docs/             # Architecture and design documents
```

## Getting Started

### Prerequisites

- Go 1.24+
- PostgreSQL 15+
- Node.js 18+ (for frontend)

### Backend

```bash
# Copy and edit config
cp config.yaml config.local.yaml
# Edit config.local.yaml with your DB credentials

# Run database migrations
go run cmd/migrate/main.go -command up

# Start the server
go run cmd/server/main.go
```

The API server listens on `:8080` by default.

### Frontend

```bash
cd frontend
npm ci
npm run dev
```

The dev server runs on `http://localhost:5173` and proxies API requests to the backend.

### Docker

```bash
# Build both images
make docker

# Or individually
make docker-api
make docker-frontend
```

### Helm Deployment

The project ships with Helm charts supporting three environments: `prod`, `demo`, and `dev`.

```bash
# Deploy to production
make helm-deploy ENV=prod

# Deploy to demo
make helm-deploy ENV=demo

# Check status
make helm-status ENV=prod
```

## Configuration

Configuration is loaded from `config.yaml` with environment variable substitution for secrets:

| Key | Description | Default |
|-----|-------------|---------|
| `database.host` | PostgreSQL host | `localhost` |
| `database.port` | PostgreSQL port | `5432` |
| `database.dbname` | Database name | `fish_fry_orders` |
| `http.address` | HTTP listen address | `:8080` |
| `auth.worker_password` | Password for worker role | (from env `AUTH_WORKER_PASSWORD`) |
| `auth.admin_password` | Password for admin role | (from env `AUTH_ADMIN_PASSWORD`) |
| `allowed_origins` | CORS / WebSocket allowed origins | `localhost:5173`, `localhost:8080` |

## Testing

```bash
# Run all tests (requires Docker for integration tests)
make test

# Run integration/E2E tests only
make test-e2e
```

Integration tests use Docker Compose to spin up a temporary PostgreSQL instance, run migrations, seed test data, and tear down after each test.

## License

Private.
