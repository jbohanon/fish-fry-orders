package database

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	libmigrate "git.nonahob.net/jacob/golibs/datastores/sql/migrate"
	libpostgres "git.nonahob.net/jacob/golibs/datastores/sql/postgres"
	"github.com/jackc/pgx/v5"
	_ "github.com/jackc/pgx/v5/stdlib" // Register pgx driver
)

// Config holds the database configuration
type Config struct {
	ctx      context.Context
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
		Host:     getEnvOrPanic("DB_HOST", "localhost"),
		Port:     getEnvOrPanic("DB_PORT", "5432"),
		User:     getEnvOrPanic("DB_USER", "postgres"),
		Password: getEnvOrPanic("DB_PASSWORD", "postgres"),
		DBName:   getEnvOrPanic("DB_NAME", "fish_fry_orders"),
		SSLMode:  getEnvOrPanic("DB_SSL_MODE", "disable"),
		ctx:      context.Background(),
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
	if c.ctx == nil {
		c.ctx = context.Background()
	}
	return c.ctx
}

// Migrate runs database migrations with auto-discovery
func (c *Config) Migrate() error {
	migrationsPath, err := resolveMigrationsPath()
	if err != nil {
		return err
	}

	libCfg := toLibPostgresConfig(c)
	return libCfg.Migrate(&libmigrate.Options{
		MigrationsDir:  migrationsPath,
		BootstrapTable: "sessions",
	})
}

func toLibPostgresConfig(c *Config) *libpostgres.Config {
	return &libpostgres.Config{
		Host:     c.Host,
		Port:     c.Port,
		User:     c.User,
		Password: c.Password,
		DBName:   c.DBName,
		SSLMode:  c.SSLMode,
	}
}

func resolveMigrationsPath() (string, error) {
	// Preferred path: relative to this source file (works from tests and binaries).
	if _, thisFile, _, ok := runtime.Caller(0); ok {
		candidate := filepath.Join(filepath.Dir(thisFile), "migrations")
		if stat, err := os.Stat(candidate); err == nil && stat.IsDir() {
			return candidate, nil
		}
	}

	// Fallback path: relative to current working directory.
	wd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("failed to get working directory: %w", err)
	}
	candidate := filepath.Join(wd, "internal", "database", "migrations")
	if stat, err := os.Stat(candidate); err == nil && stat.IsDir() {
		return candidate, nil
	}

	return "", fmt.Errorf("failed to locate migrations directory")
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

// bootstrapMigrationState detects existing schema and marks migrations as applied
// This handles databases that existed before the schema_migrations table
func (c *Config) bootstrapMigrationState(ctx context.Context, db *sql.DB, migrations []string) error {
	// Check if the schema already exists (sessions table is a good indicator of full schema)
	if !c.tableExists(ctx, db, "sessions") {
		// Fresh database, no bootstrapping needed
		return nil
	}

	// Schema exists, mark all current migrations as applied
	for _, migration := range migrations {
		if err := c.recordMigration(ctx, db, migration); err != nil {
			return fmt.Errorf("failed to record bootstrapped migration %s: %w", migration, err)
		}
		fmt.Printf("Bootstrapped migration (already applied): %s\n", migration)
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
func getEnvOrPanic(key, errMsg string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	panic(errMsg)
}
