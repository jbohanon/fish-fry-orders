package database

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// PoolConfig holds the configuration for the database connection pool
type PoolConfig struct {
	MaxConns          int
	MinConns          int
	MaxConnLifetime   time.Duration
	MaxConnIdleTime   time.Duration
	HealthCheckPeriod time.Duration
}

// DefaultPoolConfig returns the default pool configuration
func DefaultPoolConfig() *PoolConfig {
	return &PoolConfig{
		MaxConns:          10,
		MinConns:          2,
		MaxConnLifetime:   time.Hour,
		MaxConnIdleTime:   time.Minute * 30,
		HealthCheckPeriod: time.Minute,
	}
}

// NewPool creates a new database connection pool
func NewPool(ctx context.Context, config *Config, poolConfig *PoolConfig) (*pgxpool.Pool, error) {
	if poolConfig == nil {
		poolConfig = DefaultPoolConfig()
	}

	// Create connection pool configuration
	pgxConfig, err := pgxpool.ParseConfig(config.DSN())
	if err != nil {
		return nil, fmt.Errorf("failed to parse pool config: %w", err)
	}

	// Set pool configuration
	pgxConfig.MaxConns = int32(poolConfig.MaxConns)
	pgxConfig.MinConns = int32(poolConfig.MinConns)
	pgxConfig.MaxConnLifetime = poolConfig.MaxConnLifetime
	pgxConfig.MaxConnIdleTime = poolConfig.MaxConnIdleTime
	pgxConfig.HealthCheckPeriod = poolConfig.HealthCheckPeriod

	// Create connection pool
	pool, err := pgxpool.NewWithConfig(ctx, pgxConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create connection pool: %w", err)
	}

	// Test connection
	if err := pool.Ping(ctx); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return pool, nil
}

// ClosePool closes the database connection pool
func ClosePool(pool *pgxpool.Pool) {
	if pool != nil {
		pool.Close()
	}
}
