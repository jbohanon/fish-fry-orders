package database

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

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

// Migrate runs database migrations with auto-discovery
func (c *Config) Migrate() error {
	db, err := sql.Open("pgx", c.DSN())
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		return fmt.Errorf("failed to ping database: %w", err)
	}

	ctx := context.Background()

	// Check if schema_migrations table exists (indicates new migration system)
	migrationsTableExists := c.tableExists(ctx, db, "schema_migrations")

	// Create schema_migrations table if it doesn't exist
	if err := c.ensureMigrationsTable(ctx, db); err != nil {
		return fmt.Errorf("failed to create migrations table: %w", err)
	}

	// Get migrations directory
	wd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get working directory: %w", err)
	}
	migrationsPath := filepath.Join(wd, "internal", "database", "migrations")

	// Auto-discover migration files
	migrations, err := discoverMigrations(migrationsPath)
	if err != nil {
		return fmt.Errorf("failed to discover migrations: %w", err)
	}

	// Bootstrap: if schema_migrations was just created but other tables exist,
	// mark existing migrations as applied based on what tables/columns exist
	if !migrationsTableExists {
		if err := c.bootstrapMigrationState(ctx, db, migrations); err != nil {
			return fmt.Errorf("failed to bootstrap migration state: %w", err)
		}
	}

	// Get applied migrations
	applied, err := c.getAppliedMigrations(ctx, db)
	if err != nil {
		return fmt.Errorf("failed to get applied migrations: %w", err)
	}

	// Apply pending migrations
	for _, migration := range migrations {
		if applied[migration] {
			continue
		}

		migrationSQL, err := os.ReadFile(filepath.Join(migrationsPath, migration))
		if err != nil {
			return fmt.Errorf("failed to read migration file %s: %w", migration, err)
		}

		if _, err := db.ExecContext(ctx, string(migrationSQL)); err != nil {
			return fmt.Errorf("failed to execute migration %s: %w", migration, err)
		}

		if err := c.recordMigration(ctx, db, migration); err != nil {
			return fmt.Errorf("failed to record migration %s: %w", migration, err)
		}

		fmt.Printf("Applied migration: %s\n", migration)
	}

	return nil
}

// ensureMigrationsTable creates the schema_migrations table if it doesn't exist
func (c *Config) ensureMigrationsTable(ctx context.Context, db *sql.DB) error {
	_, err := db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version VARCHAR(255) PRIMARY KEY,
			applied_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
		)
	`)
	return err
}

// tableExists checks if a table exists in the database
func (c *Config) tableExists(ctx context.Context, db *sql.DB, tableName string) bool {
	var exists bool
	err := db.QueryRowContext(ctx,
		"SELECT EXISTS (SELECT FROM information_schema.tables WHERE table_name = $1)",
		tableName,
	).Scan(&exists)
	return err == nil && exists
}

// columnExists checks if a column exists in a table
func (c *Config) columnExists(ctx context.Context, db *sql.DB, tableName, columnName string) bool {
	var exists bool
	err := db.QueryRowContext(ctx,
		"SELECT EXISTS (SELECT FROM information_schema.columns WHERE table_name = $1 AND column_name = $2)",
		tableName, columnName,
	).Scan(&exists)
	return err == nil && exists
}

// bootstrapMigrationState detects existing schema and marks migrations as applied
// This handles the transition from the old migration system to the new one
func (c *Config) bootstrapMigrationState(ctx context.Context, db *sql.DB, migrations []string) error {
	// Check what already exists to determine which migrations were previously applied
	for _, migration := range migrations {
		alreadyApplied := false

		switch {
		case strings.Contains(migration, "000001_init_schema"):
			// Check if core tables exist
			alreadyApplied = c.tableExists(ctx, db, "menu_items") &&
				c.tableExists(ctx, db, "orders") &&
				c.tableExists(ctx, db, "order_items")

		case strings.Contains(migration, "000002_optional_vehicle"):
			// This migration made vehicle_description optional - check if orders table exists
			// (the column was already there, just made nullable)
			alreadyApplied = c.tableExists(ctx, db, "orders")

		case strings.Contains(migration, "000003_sessions"):
			// Check if sessions table exists
			alreadyApplied = c.tableExists(ctx, db, "sessions")

		default:
			// For unknown migrations, check if it looks like it's been applied
			// by checking if running it would fail (conservative approach: don't mark as applied)
			alreadyApplied = false
		}

		if alreadyApplied {
			if err := c.recordMigration(ctx, db, migration); err != nil {
				return fmt.Errorf("failed to record bootstrapped migration %s: %w", migration, err)
			}
			fmt.Printf("Bootstrapped migration (already applied): %s\n", migration)
		}
	}

	return nil
}

// getAppliedMigrations returns a map of already applied migrations
func (c *Config) getAppliedMigrations(ctx context.Context, db *sql.DB) (map[string]bool, error) {
	applied := make(map[string]bool)

	rows, err := db.QueryContext(ctx, "SELECT version FROM schema_migrations")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var version string
		if err := rows.Scan(&version); err != nil {
			return nil, err
		}
		applied[version] = true
	}

	return applied, rows.Err()
}

// recordMigration records a migration as applied
func (c *Config) recordMigration(ctx context.Context, db *sql.DB, version string) error {
	_, err := db.ExecContext(ctx,
		"INSERT INTO schema_migrations (version, applied_at) VALUES ($1, $2)",
		version, time.Now(),
	)
	return err
}

// discoverMigrations finds all *.up.sql files in the migrations directory
func discoverMigrations(migrationsPath string) ([]string, error) {
	entries, err := os.ReadDir(migrationsPath)
	if err != nil {
		return nil, err
	}

	var migrations []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if strings.HasSuffix(name, ".up.sql") {
			migrations = append(migrations, name)
		}
	}

	// Sort by filename (relies on numeric prefix like 000001_, 000002_, etc.)
	sort.Strings(migrations)

	return migrations, nil
}

// getEnvOrDefault returns the value of an environment variable or a default value
func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
