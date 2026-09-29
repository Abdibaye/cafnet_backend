package db

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Config holds PostgreSQL connection pool configuration parameters.
type Config struct {
	DatabaseURL     string
	MaxConns        int32
	MinConns        int32
	MaxConnLifetime time.Duration
	MaxConnIdleTime time.Duration
}

// LoadConfigFromEnv reads connection configuration from environment variables with sensible defaults.
func LoadConfigFromEnv() Config {
	cfg := Config{
		DatabaseURL:     os.Getenv("DATABASE_URL"),
		MaxConns:        10,
		MinConns:        2,
		MaxConnLifetime: 30 * time.Minute,
		MaxConnIdleTime: 5 * time.Minute,
	}

	if maxConnsStr := os.Getenv("DB_MAX_CONNS"); maxConnsStr != "" {
		if v, err := strconv.ParseInt(maxConnsStr, 10, 32); err == nil && v > 0 {
			cfg.MaxConns = int32(v)
		}
	}

	if minConnsStr := os.Getenv("DB_MIN_CONNS"); minConnsStr != "" {
		if v, err := strconv.ParseInt(minConnsStr, 10, 32); err == nil && v >= 0 {
			cfg.MinConns = int32(v)
		}
	}

	if lifetimeStr := os.Getenv("DB_MAX_CONN_LIFETIME"); lifetimeStr != "" {
		if d, err := time.ParseDuration(lifetimeStr); err == nil && d > 0 {
			cfg.MaxConnLifetime = d
		}
	}

	if idleStr := os.Getenv("DB_MAX_CONN_IDLE_TIME"); idleStr != "" {
		if d, err := time.ParseDuration(idleStr); err == nil && d > 0 {
			cfg.MaxConnIdleTime = d
		}
	}

	return cfg
}

// Pool wraps pgxpool.Pool to provide structured database connectivity, pooling, and health checks.
type Pool struct {
	*pgxpool.Pool
}

// Connect establishes a high-performance, context-aware connection pool to Neon PostgreSQL.
func Connect(ctx context.Context, cfg Config) (*Pool, error) {
	if cfg.DatabaseURL == "" {
		return nil, fmt.Errorf("database connection failed: DATABASE_URL environment variable is empty")
	}

	pgxConfig, err := pgxpool.ParseConfig(cfg.DatabaseURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse Neon connection configuration: invalid connection URI syntax")
	}

	pgxConfig.MaxConns = cfg.MaxConns
	pgxConfig.MinConns = cfg.MinConns
	pgxConfig.MaxConnLifetime = cfg.MaxConnLifetime
	pgxConfig.MaxConnIdleTime = cfg.MaxConnIdleTime

	pool, err := pgxpool.NewWithConfig(ctx, pgxConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize pgx connection pool")
	}

	// Verify initial connection health with ping
	pingCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	if err := pool.Ping(pingCtx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("failed to ping Neon PostgreSQL: database unreachable or handshake failed")
	}

	return &Pool{Pool: pool}, nil
}

// Ping checks the liveness of the database connection pool.
func (p *Pool) Ping(ctx context.Context) error {
	return p.Pool.Ping(ctx)
}

// Close gracefully closes all active connections in the pool.
func (p *Pool) Close() {
	if p.Pool != nil {
		p.Pool.Close()
	}
}
