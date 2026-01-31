package testutil

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jbohanon/fish-fry-orders-v2/internal/database"
	"github.com/jbohanon/fish-fry-orders-v2/internal/types"
)

// TestSetup holds all the test infrastructure
type TestSetup struct {
	DBPool     *pgxpool.Pool
	DBRepo     database.Repository
	TestData   *TestData
	testDBName string
	adminPool  *pgxpool.Pool
	dockerMgr  *DockerComposeManager
}

// TestData holds pre-populated test data
type TestData struct {
	MenuItems    []types.DBMenuItem
	Orders       []types.DBOrder
	OrderItems   map[int][]types.DBOrderItem   // orderID -> items
	ChatMessages map[int][]types.DBChatMessage // orderID -> messages
}

// SetupTest creates a test database using Docker Compose, runs migrations, and populates test data
func SetupTest(t *testing.T) *TestSetup {
	t.Helper()

	ctx := context.Background()

	// Check if we should use Docker Compose or existing database
	useDocker := os.Getenv("TEST_USE_DOCKER")
	if useDocker == "" {
		useDocker = "true" // Default to using Docker
	}

	var dockerMgr *DockerComposeManager
	var dbHost, dbPort, dbUser, dbPassword, dbName string

	if useDocker == "true" {
		// Create Docker Compose manager
		var err error
		dockerMgr, err = NewDockerComposeManager(DockerComposeConfig{})
		if err != nil {
			t.Fatalf("Failed to create Docker Compose manager: %v", err)
		}

		// Start the database
		if err := dockerMgr.Start(ctx); err != nil {
			dockerMgr.Cleanup()
			t.Fatalf("Failed to start Docker Compose database: %v", err)
		}

		// Get connection info from Docker manager
		var dbPortInt int
		dbHost, dbPortInt, dbUser, dbPassword, dbName = dockerMgr.GetConnectionInfo()
		dbPort = strconv.Itoa(dbPortInt)
	} else {
		// Use existing database (fallback mode)
		dbHost = os.Getenv("TEST_DB_HOST")
		if dbHost == "" {
			dbHost = "localhost"
		}
		dbPort = os.Getenv("TEST_DB_PORT")
		if dbPort == "" {
			dbPort = "5432"
		}
		dbUser = os.Getenv("TEST_DB_USER")
		if dbUser == "" {
			dbUser = "postgres"
		}
		dbPassword = os.Getenv("TEST_DB_PASSWORD")
		if dbPassword == "" {
			dbPassword = "postgres"
		}
		dbName = os.Getenv("TEST_DB_NAME")
		if dbName == "" {
			dbName = "fish_fry_orders_test"
		}

		// Create a unique database name for this test run to avoid conflicts
		dbName = fmt.Sprintf("%s_%d", dbName, time.Now().UnixNano())

		// Connect to postgres database to create the test database
		adminDSN := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=postgres sslmode=disable",
			dbHost, dbPort, dbUser, dbPassword)

		adminPool, err := pgxpool.New(ctx, adminDSN)
		if err != nil {
			t.Fatalf("Failed to connect to postgres database: %v", err)
		}

		// Terminate any existing connections to the test database
		_, _ = adminPool.Exec(ctx, fmt.Sprintf(`
			SELECT pg_terminate_backend(pid)
			FROM pg_stat_activity
			WHERE datname = '%s' AND pid <> pg_backend_pid()
		`, dbName))

		// Drop the test database if it exists
		_, _ = adminPool.Exec(ctx, fmt.Sprintf("DROP DATABASE IF EXISTS %s", dbName))

		// Create the test database
		_, err = adminPool.Exec(ctx, fmt.Sprintf("CREATE DATABASE %s", dbName))
		if err != nil {
			adminPool.Close()
			t.Fatalf("Failed to create test database: %v", err)
		}

		// Store admin pool for cleanup
		setup := &TestSetup{
			adminPool:  adminPool,
			testDBName: dbName,
		}
		_ = setup
	}

	// Create database configuration
	dbConfig := &database.Config{
		Host:     dbHost,
		Port:     dbPort,
		User:     dbUser,
		Password: dbPassword,
		DBName:   dbName,
		SSLMode:  "disable",
	}

	// Create connection pool
	pool, err := database.NewPool(ctx, dbConfig, database.DefaultPoolConfig())
	if err != nil {
		if dockerMgr != nil {
			dockerMgr.Cleanup()
		}
		t.Fatalf("Failed to create connection pool: %v", err)
	}

	// Run migrations
	if err := dbConfig.Migrate(); err != nil {
		pool.Close()
		if dockerMgr != nil {
			dockerMgr.Cleanup()
		}
		t.Fatalf("Failed to run migrations: %v", err)
	}

	// Create repository
	repo := database.NewPostgresRepository(pool)

	// Populate test data
	testData := populateTestData(ctx, t, repo)

	setup := &TestSetup{
		DBPool:     pool,
		DBRepo:     repo,
		TestData:   testData,
		testDBName: dbName,
		dockerMgr:  dockerMgr,
	}

	// Register cleanup function
	t.Cleanup(func() {
		setup.Teardown(t)
	})

	return setup
}

// Teardown cleans up the test database
func (ts *TestSetup) Teardown(t *testing.T) {
	t.Helper()

	if ts.DBPool != nil {
		ts.DBPool.Close()
	}

	// If using Docker Compose, clean it up
	if ts.dockerMgr != nil {
		if err := ts.dockerMgr.Cleanup(); err != nil {
			t.Logf("Warning: Failed to cleanup Docker Compose: %v", err)
		}
		return
	}

	// Otherwise, clean up the traditional way
	if ts.adminPool == nil || ts.testDBName == "" {
		return
	}

	ctx := context.Background()

	// Terminate any remaining connections
	_, _ = ts.adminPool.Exec(ctx, fmt.Sprintf(`
		SELECT pg_terminate_backend(pid)
		FROM pg_stat_activity
		WHERE datname = '%s' AND pid <> pg_backend_pid()
	`, ts.testDBName))

	// Drop the test database
	_, err := ts.adminPool.Exec(ctx, fmt.Sprintf("DROP DATABASE IF EXISTS %s", ts.testDBName))
	if err != nil {
		t.Logf("Warning: Failed to drop test database: %v", err)
	}

	ts.adminPool.Close()
}

// populateTestData creates test data in the database
func populateTestData(ctx context.Context, t *testing.T, repo database.Repository) *TestData {
	// Create menu items
	menuItems := []types.DBMenuItem{
		{ID: "test-baked-fish", Name: "Baked fish dinner", Price: 12.99, IsActive: true, DisplayOrder: 1},
		{ID: "test-fried-fish", Name: "Fried fish dinner", Price: 12.99, IsActive: true, DisplayOrder: 2},
		{ID: "test-kids-pizza", Name: "Kids pizza dinner", Price: 6.99, IsActive: true, DisplayOrder: 3},
		{ID: "test-inactive-item", Name: "Inactive item", Price: 5.99, IsActive: false, DisplayOrder: 4},
		{ID: "test-extra-fish", Name: "Extra piece of fish", Price: 3.99, IsActive: true, DisplayOrder: 5},
	}

	for i := range menuItems {
		item := &menuItems[i]
		if err := repo.CreateMenuItem(ctx, item); err != nil {
			t.Fatalf("Failed to create test menu item: %v", err)
		}
	}

	// Create orders
	orders := []types.DBOrder{
		{VehicleDescription: "Red Toyota Camry", Status: "NEW"},
		{VehicleDescription: "Blue Honda Accord", Status: "IN_PROGRESS"},
		{VehicleDescription: "White Ford F-150", Status: "COMPLETED"},
		{VehicleDescription: "Black Tesla Model 3", Status: "NEW"},
	}

	for i := range orders {
		order := &orders[i]
		if err := repo.CreateOrder(ctx, order); err != nil {
			t.Fatalf("Failed to create test order: %v", err)
		}
	}

	// Create order items
	orderItems := make(map[int][]types.DBOrderItem)

	// Order 1 items
	orderItems[orders[0].ID] = []types.DBOrderItem{
		{OrderID: orders[0].ID, MenuItemID: "test-baked-fish", Quantity: 2},
		{OrderID: orders[0].ID, MenuItemID: "test-kids-pizza", Quantity: 1},
	}

	// Order 2 items
	orderItems[orders[1].ID] = []types.DBOrderItem{
		{OrderID: orders[1].ID, MenuItemID: "test-fried-fish", Quantity: 1},
		{OrderID: orders[1].ID, MenuItemID: "test-extra-fish", Quantity: 2},
	}

	// Order 3 items
	orderItems[orders[2].ID] = []types.DBOrderItem{
		{OrderID: orders[2].ID, MenuItemID: "test-baked-fish", Quantity: 3},
	}

	// Order 4 items
	orderItems[orders[3].ID] = []types.DBOrderItem{
		{OrderID: orders[3].ID, MenuItemID: "test-fried-fish", Quantity: 1},
		{OrderID: orders[3].ID, MenuItemID: "test-kids-pizza", Quantity: 2},
		{OrderID: orders[3].ID, MenuItemID: "test-extra-fish", Quantity: 1},
	}

	for orderID, items := range orderItems {
		for j := range items {
			item := &items[j]
			if err := repo.CreateOrderItem(ctx, item); err != nil {
				t.Fatalf("Failed to create test order item: %v", err)
			}
			orderItems[orderID][j] = *item // Update with generated ID
		}
	}

	// Create chat messages
	chatMessages := make(map[int][]types.DBChatMessage)

	// Messages for order 1
	chatMessages[orders[0].ID] = []types.DBChatMessage{
		{OrderID: orders[0].ID, Content: "Order received", SenderRole: "WORKER"},
		{OrderID: orders[0].ID, Content: "Starting preparation", SenderRole: "ADMIN"},
	}

	// Messages for order 2
	chatMessages[orders[1].ID] = []types.DBChatMessage{
		{OrderID: orders[1].ID, Content: "In the kitchen", SenderRole: "WORKER"},
	}

	for orderID, messages := range chatMessages {
		for j := range messages {
			msg := &messages[j]
			if err := repo.CreateChatMessage(ctx, msg); err != nil {
				t.Fatalf("Failed to create test chat message: %v", err)
			}
			chatMessages[orderID][j] = *msg // Update with generated ID
		}
	}

	return &TestData{
		MenuItems:    menuItems,
		Orders:       orders,
		OrderItems:   orderItems,
		ChatMessages: chatMessages,
	}
}
