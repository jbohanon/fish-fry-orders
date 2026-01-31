package testutil

import (
	"context"
	"fmt"
	"math/rand"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
)

// DockerComposeManager manages a PostgreSQL database via docker compose
type DockerComposeManager struct {
	composeFile   string
	projectName   string
	dbPort        int
	dbName        string
	dbUser        string
	dbPassword    string
	workDir       string
	started       bool
	mu            sync.Mutex
	cleanupCalled bool
}

// DockerComposeConfig holds configuration for the docker-compose manager
type DockerComposeConfig struct {
	WorkDir    string // Directory where docker-compose.yml will be created
	ProjectName string // Docker Compose project name (optional, will be randomized if empty)
}

// NewDockerComposeManager creates a new Docker Compose manager with randomized configuration
func NewDockerComposeManager(cfg DockerComposeConfig) (*DockerComposeManager, error) {
	if cfg.WorkDir == "" {
		workDir, err := os.MkdirTemp("", "fish-fry-test-*")
		if err != nil {
			return nil, fmt.Errorf("failed to create temp directory: %w", err)
		}
		cfg.WorkDir = workDir
	}

	// Generate random values to avoid conflicts
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	
	projectName := cfg.ProjectName
	if projectName == "" {
		projectName = fmt.Sprintf("fishfrytest%d", rng.Intn(999999))
	}

	// Use a random port in the range 50000-60000 to avoid conflicts
	dbPort := 50000 + rng.Intn(10000)

	// Random database name and credentials
	dbName := fmt.Sprintf("testdb_%d", rng.Intn(999999))
	dbUser := fmt.Sprintf("testuser_%d", rng.Intn(999999))
	dbPassword := fmt.Sprintf("testpass_%d", rng.Intn(999999))

	composeFile := filepath.Join(cfg.WorkDir, "docker-compose.yml")

	manager := &DockerComposeManager{
		composeFile: composeFile,
		projectName: projectName,
		dbPort:      dbPort,
		dbName:      dbName,
		dbUser:      dbUser,
		dbPassword:  dbPassword,
		workDir:     cfg.WorkDir,
		started:     false,
	}

	// Create docker-compose.yml file
	if err := manager.createComposeFile(); err != nil {
		return nil, fmt.Errorf("failed to create docker-compose file: %w", err)
	}

	return manager, nil
}

// createComposeFile creates the docker-compose.yml file
func (dcm *DockerComposeManager) createComposeFile() error {
	composeContent := fmt.Sprintf(`services:
  postgres:
    image: postgres:15-alpine
    container_name: %s_postgres
    environment:
      POSTGRES_DB: %s
      POSTGRES_USER: %s
      POSTGRES_PASSWORD: %s
    ports:
      - "%d:5432"
    volumes:
      - postgres_data_%s:/var/lib/postgresql/data
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U %s -d %s"]
      interval: 2s
      timeout: 5s
      retries: 10
    tmpfs:
      - /var/run/postgresql
    command: >
      postgres
      -c max_connections=200
      -c shared_buffers=256MB
      -c effective_cache_size=1GB
      -c maintenance_work_mem=64MB
      -c checkpoint_completion_target=0.9
      -c wal_buffers=16MB
      -c default_statistics_target=100
      -c random_page_cost=1.1
      -c effective_io_concurrency=200
      -c work_mem=4MB
      -c min_wal_size=1GB
      -c max_wal_size=4GB

volumes:
  postgres_data_%s:
`, dcm.projectName, dcm.dbName, dcm.dbUser, dcm.dbPassword, dcm.dbPort, dcm.projectName, dcm.dbUser, dcm.dbName, dcm.projectName)

	return os.WriteFile(dcm.composeFile, []byte(composeContent), 0644)
}

// Start starts the PostgreSQL container
func (dcm *DockerComposeManager) Start(ctx context.Context) error {
	dcm.mu.Lock()
	defer dcm.mu.Unlock()

	if dcm.started {
		return nil
	}

	// Check if docker compose is available
	if err := dcm.checkDockerCompose(); err != nil {
		return fmt.Errorf("docker compose check failed: %w", err)
	}

	// Start the containers using docker compose (v2)
	cmd := exec.CommandContext(ctx, "docker", "compose", "-f", dcm.composeFile, "-p", dcm.projectName, "up", "-d")
	cmd.Dir = dcm.workDir
	// Don't capture stdout/stderr for startup to avoid cluttering test output
	// Errors will still be visible if the command fails
	cmd.Stdout = nil
	cmd.Stderr = nil

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to start docker compose: %w", err)
	}

	// Wait for PostgreSQL to be ready
	// This function now handles both container startup and postgres readiness
	if err := dcm.waitForPostgres(ctx); err != nil {
		dcm.Stop(ctx) // Try to clean up on failure
		return fmt.Errorf("postgres failed to become ready: %w", err)
	}

	dcm.started = true
	return nil
}

// waitForPostgres waits for PostgreSQL to be ready by attempting database connections
func (dcm *DockerComposeManager) waitForPostgres(ctx context.Context) error {
	timeout := time.After(60 * time.Second)
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	dsn := fmt.Sprintf("host=localhost port=%d user=%s password=%s dbname=%s sslmode=disable",
		dcm.dbPort, dcm.dbUser, dcm.dbPassword, dcm.dbName)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-timeout:
			return fmt.Errorf("timeout waiting for postgres to be ready after 60 seconds")
		case <-ticker.C:
			// Try to connect to the database directly
			// This is more reliable than docker exec and won't hang
			checkCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
			
			// First, try a simple TCP connection to see if the port is open
			conn, err := net.DialTimeout("tcp", net.JoinHostPort("localhost", strconv.Itoa(dcm.dbPort)), 1*time.Second)
			if err == nil {
				conn.Close()
				
				// Port is open, now try a real database connection
				pgxConn, err := pgx.Connect(checkCtx, dsn)
				if err == nil {
					// Test the connection
					if err := pgxConn.Ping(checkCtx); err == nil {
						pgxConn.Close(checkCtx)
						cancel()
						return nil
					}
					pgxConn.Close(checkCtx)
				}
			}
			cancel()
		}
	}
}

// Stop stops and removes the PostgreSQL container
func (dcm *DockerComposeManager) Stop(ctx context.Context) error {
	dcm.mu.Lock()
	defer dcm.mu.Unlock()

	if !dcm.started {
		return nil
	}

	// Stop and remove containers, networks, and volumes using docker compose (v2)
	cmd := exec.CommandContext(ctx, "docker", "compose", "-f", dcm.composeFile, "-p", dcm.projectName, "down", "-v", "--remove-orphans")
	cmd.Dir = dcm.workDir
	// Don't capture stdout/stderr for cleanup to avoid cluttering test output
	cmd.Stdout = nil
	cmd.Stderr = nil

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to stop docker compose: %w", err)
	}

	dcm.started = false
	return nil
}

// Cleanup removes the docker-compose file and work directory
func (dcm *DockerComposeManager) Cleanup() error {
	dcm.mu.Lock()
	defer dcm.mu.Unlock()

	if dcm.cleanupCalled {
		return nil
	}

	ctx := context.Background()
		if dcm.started {
			if err := dcm.Stop(ctx); err != nil {
				// Log but don't fail cleanup
				fmt.Fprintf(os.Stderr, "Warning: failed to stop docker compose during cleanup: %v\n", err)
			}
		}

	// Remove the work directory
	if dcm.workDir != "" {
		if err := os.RemoveAll(dcm.workDir); err != nil {
			return fmt.Errorf("failed to remove work directory: %w", err)
		}
	}

	dcm.cleanupCalled = true
	return nil
}

// GetConnectionInfo returns the database connection information
func (dcm *DockerComposeManager) GetConnectionInfo() (host string, port int, user string, password string, dbName string) {
	return "localhost", dcm.dbPort, dcm.dbUser, dcm.dbPassword, dcm.dbName
}

// checkDockerCompose checks if docker compose (v2) is available
func (dcm *DockerComposeManager) checkDockerCompose() error {
	// Check for docker compose (v2)
	cmd := exec.Command("docker", "compose", "version")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("docker compose not found. Please install Docker with the compose plugin")
	}
	return nil
}

// IsStarted returns whether the database has been started
func (dcm *DockerComposeManager) IsStarted() bool {
	dcm.mu.Lock()
	defer dcm.mu.Unlock()
	return dcm.started
}
