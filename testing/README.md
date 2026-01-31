# End-to-End Test Suite

This directory contains a comprehensive end-to-end test suite for the fish-fry-orders-v2 backend.

## Structure

- `testutil/` - Centralized test utilities
  - `setup.go` - Database setup and teardown, test data population
  - `server.go` - Test server creation and management
  - `helpers.go` - HTTP request helpers and assertions
- `orders_test.go` - Orders API test suite
- `menu_test.go` - Menu items API test suite
- `chat_test.go` - Chat messages API test suite
- `auth_test.go` - Authentication test suite
- `websocket_test.go` - WebSocket functionality test suite

## Setup

### Prerequisites

1. **Docker and Docker Compose** (recommended) - The test suite will automatically spin up isolated PostgreSQL instances
   - OR
2. **PostgreSQL database server** (fallback mode) - If Docker is not available

### Environment Variables

The test suite uses the following environment variables:

**Docker Mode (Default):**
- `TEST_USE_DOCKER` (default: `"true"`) - Set to `"false"` to use existing PostgreSQL instead

**Fallback Mode (when `TEST_USE_DOCKER=false`):**
- `TEST_DB_HOST` (default: `localhost`)
- `TEST_DB_PORT` (default: `5432`)
- `TEST_DB_USER` (default: `postgres`)
- `TEST_DB_PASSWORD` (default: `postgres`)
- `TEST_DB_NAME` (default: `fish_fry_orders_test`)

### Docker Compose Mode

When using Docker Compose (default), the test suite will:
- Automatically generate randomized configuration (ports, database names, credentials)
- Create isolated PostgreSQL containers per test run
- Handle startup and teardown automatically
- Use random ports (50000-60000) to avoid conflicts
- Clean up all resources after tests complete

This ensures:
- No conflicts with running database instances
- Complete isolation between test runs
- No manual database setup required
- Works with both `docker-compose` (v1) and `docker compose` (v2)

### Running Tests

```bash
# Run all tests
go test ./testing/...

# Run specific test suite
go test ./testing/... -run TestOrders

# Run with verbose output
go test ./testing/... -v

# Run with race detector
go test ./testing/... -race
```

## Test Utilities

### TestSetup

The `TestSetup` struct provides:
- Isolated test database (created per test, cleaned up automatically)
- Pre-populated test data (menu items, orders, order items, chat messages)
- Database repository for direct database operations
- Automatic cleanup via `t.Cleanup()`

### TestServer

The `TestServer` provides:
- Full Echo server instance with all routes
- Authentication service with test passwords
- HTTP test server for making requests
- Base URL for constructing requests

### Helper Functions

- `AuthenticatedRequest()` - Makes authenticated HTTP requests
- `UnauthenticatedRequest()` - Makes unauthenticated HTTP requests
- `ParseJSONResponse()` - Parses JSON response bodies
- `AssertStatusCode()` - Asserts HTTP status codes

## Test Data

Each test run creates a fresh database with the following test data:

### Menu Items
- `test-baked-fish` - Baked fish dinner ($12.99, active)
- `test-fried-fish` - Fried fish dinner ($12.99, active)
- `test-kids-pizza` - Kids pizza dinner ($6.99, active)
- `test-inactive-item` - Inactive item ($5.99, inactive)
- `test-extra-fish` - Extra piece of fish ($3.99, active)

### Orders
- Order 1: "Red Toyota Camry" (NEW status)
- Order 2: "Blue Honda Accord" (IN_PROGRESS status)
- Order 3: "White Ford F-150" (COMPLETED status)
- Order 4: "Black Tesla Model 3" (NEW status)

Each order has associated order items and some have chat messages.

## Test Coverage

### Orders API
- ✅ Create order
- ✅ Get all orders (with sorting verification)
- ✅ Get single order
- ✅ Update order status
- ✅ Purge orders (today/all)
- ✅ Authentication requirements
- ✅ Input validation

### Menu Items API
- ✅ Get all menu items (active only)
- ✅ Get single menu item
- ✅ Create menu item
- ✅ Update menu item
- ✅ Delete menu item
- ✅ Update menu items order
- ✅ Authentication requirements
- ✅ Input validation

### Chat Messages API
- ✅ Create message
- ✅ Get messages for order
- ✅ Message ordering
- ✅ Authentication requirements
- ✅ Input validation

### Authentication
- ✅ Login with worker password
- ✅ Login with admin password
- ✅ Login with invalid password
- ✅ Logout
- ✅ Check authentication status
- ✅ Protected endpoint access

### WebSocket
- ✅ Authentication requirement
- ✅ Connection handling
- ✅ Order update broadcasts (structure verified)
- ✅ Stats update broadcasts (structure verified)

## Notes

- **Docker Mode (Default):** Each test creates a unique Docker Compose project with randomized ports and database names to avoid conflicts
- **Fallback Mode:** Each test creates a unique database name with timestamp to avoid conflicts
- Tests are designed to be run in parallel (each has its own database/container)
- Database cleanup is automatic via Go's `t.Cleanup()` mechanism
- Docker containers are automatically stopped and removed after tests
- Test passwords are: `test-worker-password` and `test-admin-password`
- Docker Compose manager uses `docker compose` (v2) - the modern, non-deprecated command

## Future Enhancements

- Real WebSocket connection tests (currently structure-only)
- Performance/load tests
- Integration with CI/CD pipeline
- Test coverage reporting
- More edge case scenarios
