package database

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/jackc/pgx/v5"
	_ "github.com/jackc/pgx/v5/stdlib" // Register pgx driver
)

// Config holds the database configuration
type Config struct {
	Host     string
	Port     string
	User     string
	Password string
	DBName   string
	SSLMode  string
}

// NewConfig creates a new database configuration from environment variables
func NewConfig() *Config {
	return &Config{
		Host:     getEnvOrDefault("DB_HOST", "localhost"),
		Port:     getEnvOrDefault("DB_PORT", "5432"),
		User:     getEnvOrDefault("DB_USER", "postgres"),
		Password: getEnvOrDefault("DB_PASSWORD", "postgres"),
		DBName:   getEnvOrDefault("DB_NAME", "fish_fry_orders"),
		SSLMode:  getEnvOrDefault("DB_SSL_MODE", "disable"),
	}
}

// DSN returns the database connection string
func (c *Config) DSN() string {
	return fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		c.Host, c.Port, c.User, c.Password, c.DBName, c.SSLMode)
}

// Connect creates a new database connection
func (c *Config) Connect() (*pgx.Conn, error) {
	conn, err := pgx.Connect(c.Context(), c.DSN())
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}
	return conn, nil
}

// Context returns a context for database operations
func (c *Config) Context() context.Context {
	return context.Background()
}

// Migrate runs database migrations
func (c *Config) Migrate() error {
	db, err := sql.Open("pgx", c.DSN())
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}
	defer db.Close()

	// Test connection
	if err := db.Ping(); err != nil {
		return fmt.Errorf("failed to ping database: %w", err)
	}

	// Get absolute path to migrations directory
	wd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get working directory: %w", err)
	}
	migrationsPath := filepath.Join(wd, "internal", "database", "migrations")

	ctx := context.Background()

	// Check if migrations are already applied by checking if tables exist
	var tableExists bool
	checkErr := db.QueryRowContext(ctx, "SELECT EXISTS (SELECT FROM information_schema.tables WHERE table_name = 'menu_items')").Scan(&tableExists)
	if checkErr == nil && tableExists {
		// Check if order ID conversion has been applied - check both orders.id and order_items.order_id
		var ordersIDType, orderItemsIDType string
		ordersErr := db.QueryRowContext(ctx, `
			SELECT data_type 
			FROM information_schema.columns 
			WHERE table_name = 'orders' AND column_name = 'id'
		`).Scan(&ordersIDType)
		orderItemsErr := db.QueryRowContext(ctx, `
			SELECT data_type 
			FROM information_schema.columns 
			WHERE table_name = 'order_items' AND column_name = 'order_id'
		`).Scan(&orderItemsIDType)
		
		if ordersErr == nil && orderItemsErr == nil && ordersIDType == "integer" && orderItemsIDType == "integer" {
			// All migrations applied - but double-check with fix function
			if err := FixOrderIDSchema(db); err != nil {
				return fmt.Errorf("failed to verify/fix schema: %w", err)
			}
			return nil
		}
		// Initial schema exists but order ID conversion not fully applied
		// Apply the second migration
		migrationSQL, err := os.ReadFile(filepath.Join(migrationsPath, "000002_convert_order_ids_to_integer.up.sql"))
		if err != nil {
			return fmt.Errorf("failed to read migration file: %w", err)
		}
		if _, err := db.ExecContext(ctx, string(migrationSQL)); err != nil {
			if !isDuplicateError(err) {
				return fmt.Errorf("failed to execute migration: %w", err)
			}
		}
		// After migration, verify/fix schema
		if err := FixOrderIDSchema(db); err != nil {
			return fmt.Errorf("failed to verify/fix schema after migration: %w", err)
		}
		return nil
	}

	// Execute migrations in order
	migrations := []string{
		"000001_init_schema.up.sql",
		"000002_convert_order_ids_to_integer.up.sql",
	}

	for _, migrationFile := range migrations {
		migrationSQL, err := os.ReadFile(filepath.Join(migrationsPath, migrationFile))
		if err != nil {
			return fmt.Errorf("failed to read migration file %s: %w", migrationFile, err)
		}

		if _, err := db.ExecContext(ctx, string(migrationSQL)); err != nil {
			if !isDuplicateError(err) {
				return fmt.Errorf("failed to execute migration %s: %w", migrationFile, err)
			}
		}
	}

	// After all migrations, verify/fix schema
	if err := FixOrderIDSchema(db); err != nil {
		return fmt.Errorf("failed to verify/fix schema after migrations: %w", err)
	}

	return nil
}

// isDuplicateError checks if the error indicates objects already exist
func isDuplicateError(err error) bool {
	if err == nil {
		return false
	}
	errStr := strings.ToLower(err.Error())
	return strings.Contains(errStr, "already exists") ||
		strings.Contains(errStr, "duplicate key") ||
		(strings.Contains(errStr, "relation") && strings.Contains(errStr, "already exists"))
}

// getEnvOrDefault returns the value of an environment variable or a default value
func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
