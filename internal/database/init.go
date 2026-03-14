package database

import (
	"context"
	"fmt"
	"os"
	"strconv"

	"git.nonahob.net/jacob/fish-fry-orders/internal/config"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Init initializes the database connection and runs migrations
func Init(ctx context.Context) (*pgxpool.Pool, Repository, error) {
	// Create database configuration
	config := NewConfig()

	// Create connection pool
	pool, err := NewPool(ctx, config, DefaultPoolConfig())
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create connection pool: %w", err)
	}

	// Run migrations
	if err := config.Migrate(); err != nil {
		ClosePool(pool)
		return nil, nil, fmt.Errorf("failed to run migrations: %w", err)
	}

	// Create repository using the pool
	repo := NewPostgresRepository(pool)

	return pool, repo, nil
}

// InitFromEnv initializes the database connection from environment variables
func InitFromEnv(ctx context.Context) (*pgxpool.Pool, Repository, error) {
	// Check if we're in a test environment
	if os.Getenv("TEST_DATABASE") == "true" {
		return InitTest(ctx)
	}

	return Init(ctx)
}

// InitFromConfig initializes the database connection from a config struct
func InitFromConfig(ctx context.Context, cfg *config.DatabaseConfig) (*pgxpool.Pool, Repository, error) {
	// Create database configuration
	dbConfig := &Config{
		Host:     cfg.Host,
		Port:     strconv.Itoa(cfg.Port),
		User:     cfg.User,
		Password: cfg.Password,
		DBName:   cfg.DBName,
		SSLMode:  cfg.SSLMode,
	}

	// Create connection pool
	pool, err := NewPool(ctx, dbConfig, DefaultPoolConfig())
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create connection pool: %w", err)
	}

	// Run migrations
	if err := dbConfig.Migrate(); err != nil {
		ClosePool(pool)
		return nil, nil, fmt.Errorf("failed to run migrations: %w", err)
	}

	// Create repository using the pool
	repo := NewPostgresRepository(pool)

	return pool, repo, nil
}

// InitFromConfigNoPing initializes the repository from config without a startup
// database ping or migration run. This is useful for long-running services that
// must remain alive while the database is temporarily unavailable.
func InitFromConfigNoPing(ctx context.Context, cfg *config.DatabaseConfig) (*pgxpool.Pool, Repository, error) {
	dbConfig := &Config{
		Host:     cfg.Host,
		Port:     strconv.Itoa(cfg.Port),
		User:     cfg.User,
		Password: cfg.Password,
		DBName:   cfg.DBName,
		SSLMode:  cfg.SSLMode,
	}

	pool, err := NewPoolNoPing(ctx, dbConfig, DefaultPoolConfig())
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create connection pool: %w", err)
	}

	repo := NewPostgresRepository(pool)
	return pool, repo, nil
}

// InitTest initializes a test database connection
func InitTest(ctx context.Context) (*pgxpool.Pool, Repository, error) {
	// Create test database configuration
	config := &Config{
		Host:     "localhost",
		Port:     "5432",
		User:     "postgres",
		Password: "postgres",
		DBName:   "fish_fry_orders_test",
		SSLMode:  "disable",
	}

	// Create connection pool
	pool, err := NewPool(ctx, config, DefaultPoolConfig())
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create test connection pool: %w", err)
	}

	// Run migrations
	if err := config.Migrate(); err != nil {
		ClosePool(pool)
		return nil, nil, fmt.Errorf("failed to run test migrations: %w", err)
	}

	// Create repository using the pool
	repo := NewPostgresRepository(pool)

	return pool, repo, nil
}
