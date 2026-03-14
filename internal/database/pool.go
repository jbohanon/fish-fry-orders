package database

import (
	"context"

	libpostgres "git.nonahob.net/jacob/golibs/datastores/sql/postgres"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PoolConfig holds the configuration for the database connection pool.
type PoolConfig = libpostgres.PoolConfig

// DefaultPoolConfig returns the default pool configuration.
func DefaultPoolConfig() *PoolConfig {
	return libpostgres.DefaultPoolConfig()
}

// NewPool creates a new database connection pool.
func NewPool(ctx context.Context, config *Config, poolConfig *PoolConfig) (*pgxpool.Pool, error) {
	libCfg := toLibPostgresConfig(config)
	libPoolCfg := (*libpostgres.PoolConfig)(poolConfig)
	if libPoolCfg == nil {
		libPoolCfg = libpostgres.DefaultPoolConfig()
	}
	return libpostgres.NewPool(ctx, libCfg, libPoolCfg)
}

// NewPoolNoPing creates a new database connection pool without an initial ping.
// This allows services to boot and keep retrying when the database is temporarily unavailable.
func NewPoolNoPing(ctx context.Context, config *Config, poolConfig *PoolConfig) (*pgxpool.Pool, error) {
	pgxConfig, err := pgxpool.ParseConfig(config.DSN())
	if err != nil {
		return nil, err
	}

	if poolConfig == nil {
		poolConfig = DefaultPoolConfig()
	}

	pgxConfig.MaxConns = int32(poolConfig.MaxConns)
	pgxConfig.MinConns = int32(poolConfig.MinConns)
	pgxConfig.MaxConnLifetime = poolConfig.MaxConnLifetime
	pgxConfig.MaxConnIdleTime = poolConfig.MaxConnIdleTime
	pgxConfig.HealthCheckPeriod = poolConfig.HealthCheckPeriod

	return pgxpool.NewWithConfig(ctx, pgxConfig)
}

// ClosePool closes the database connection pool
func ClosePool(pool *pgxpool.Pool) {
	if pool != nil {
		pool.Close()
	}
}
