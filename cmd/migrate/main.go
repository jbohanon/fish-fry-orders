package main

import (
	"database/sql"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	_ "github.com/jackc/pgx/v5/stdlib" // Register pgx driver
	"github.com/jbohanon/fish-fry-orders-v2/internal/config"
)

func main() {
	// Parse command line flags
	dsn := flag.String("dsn", "", "Database connection string (overrides config.yaml)")
	command := flag.String("command", "up", "Migration command (up, down, status)")
	flag.Parse()

	var dbDSN string
	if *dsn != "" {
		// Use provided DSN
		dbDSN = *dsn
	} else {
		// Load from config.yaml
		cfg, err := config.Load()
		if err != nil {
			log.Fatalf("Failed to load config: %v", err)
		}
		dbDSN = fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
			cfg.Database.Host,
			cfg.Database.Port,
			cfg.Database.User,
			cfg.Database.Password,
			cfg.Database.DBName,
			cfg.Database.SSLMode,
		)
		log.Printf("Using database: %s@%s:%d/%s", cfg.Database.User, cfg.Database.Host, cfg.Database.Port, cfg.Database.DBName)
	}

	// Set up database connection
	db, err := sql.Open("pgx", dbDSN)
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	// Test connection
	if err := db.Ping(); err != nil {
		log.Fatalf("Failed to ping database: %v", err)
	}

	// Get migrations directory - try executable dir first, then working directory
	var migrationsDir string
	if exePath, err := os.Executable(); err == nil {
		exeDir := filepath.Dir(exePath)
		migrationsDir = filepath.Join(exeDir, "internal", "database", "migrations")
		// Check if it exists
		if _, err := os.Stat(migrationsDir); os.IsNotExist(err) {
			// Fall back to working directory
			wd, _ := os.Getwd()
			migrationsDir = filepath.Join(wd, "internal", "database", "migrations")
		}
	} else {
		wd, _ := os.Getwd()
		migrationsDir = filepath.Join(wd, "internal", "database", "migrations")
	}

	log.Printf("Using migrations directory: %s", migrationsDir)

	// Ensure migrations tracking table exists
	if err := ensureMigrationsTable(db); err != nil {
		log.Fatalf("Failed to ensure migrations table: %v", err)
	}

	// Discover migration files
	upMigrations, downMigrations, err := discoverMigrations(migrationsDir)
	if err != nil {
		log.Fatalf("Failed to discover migrations: %v", err)
	}

	// Run migration command
	switch *command {
	case "up":
		if err := runMigrationsUp(db, upMigrations, migrationsDir); err != nil {
			log.Fatalf("Failed to run migrations: %v", err)
		}
		log.Println("All migrations applied successfully")
	case "down":
		if err := runMigrationsDown(db, downMigrations, migrationsDir); err != nil {
			log.Fatalf("Failed to roll back migrations: %v", err)
		}
		log.Println("All migrations rolled back successfully")
	case "status":
		if err := showMigrationStatus(db, upMigrations); err != nil {
			log.Fatalf("Failed to show migration status: %v", err)
		}
	default:
		log.Fatalf("Invalid command: %s (use: up, down, or status)", *command)
	}
}

// Migration represents a migration file
type Migration struct {
	Number int
	Name   string
	File   string
}

// ensureMigrationsTable creates the migrations tracking table if it doesn't exist
func ensureMigrationsTable(db *sql.DB) error {
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version VARCHAR(255) PRIMARY KEY,
			applied_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
		)
	`)
	return err
}

// discoverMigrations finds all migration files in the directory and returns them sorted
func discoverMigrations(migrationsDir string) ([]Migration, []Migration, error) {
	entries, err := os.ReadDir(migrationsDir)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read migrations directory: %w", err)
	}

	var upMigrations []Migration
	var downMigrations []Migration

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		filename := entry.Name()
		
		// Parse migration files: 000001_name.up.sql or 000001_name.down.sql
		if strings.HasSuffix(filename, ".up.sql") {
			// Extract number and name
			parts := strings.Split(filename, "_")
			if len(parts) < 2 {
				continue
			}
			
			number, err := strconv.Atoi(parts[0])
			if err != nil {
				log.Printf("Warning: skipping invalid migration file %s (invalid number)", filename)
				continue
			}
			
			// Reconstruct name without number prefix and .up.sql suffix
			name := strings.TrimSuffix(strings.Join(parts[1:], "_"), ".up.sql")
			
			upMigrations = append(upMigrations, Migration{
				Number: number,
				Name:   name,
				File:   filename,
			})
		} else if strings.HasSuffix(filename, ".down.sql") {
			// Extract number and name
			parts := strings.Split(filename, "_")
			if len(parts) < 2 {
				continue
			}
			
			number, err := strconv.Atoi(parts[0])
			if err != nil {
				log.Printf("Warning: skipping invalid migration file %s (invalid number)", filename)
				continue
			}
			
			// Reconstruct name without number prefix and .down.sql suffix
			name := strings.TrimSuffix(strings.Join(parts[1:], "_"), ".down.sql")
			
			downMigrations = append(downMigrations, Migration{
				Number: number,
				Name:   name,
				File:   filename,
			})
		}
	}

	// Sort migrations by number
	sort.Slice(upMigrations, func(i, j int) bool {
		return upMigrations[i].Number < upMigrations[j].Number
	})
	sort.Slice(downMigrations, func(i, j int) bool {
		return downMigrations[i].Number > downMigrations[j].Number // Reverse order for down migrations
	})

	return upMigrations, downMigrations, nil
}

// isMigrationApplied checks if a migration has been applied
func isMigrationApplied(db *sql.DB, migration Migration) (bool, error) {
	var exists bool
	err := db.QueryRow(
		"SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE version = $1)",
		migration.File,
	).Scan(&exists)
	return exists, err
}

// recordMigrationApplied records that a migration has been applied
func recordMigrationApplied(tx *sql.Tx, migration Migration) error {
	_, err := tx.Exec(
		"INSERT INTO schema_migrations (version) VALUES ($1) ON CONFLICT (version) DO NOTHING",
		migration.File,
	)
	return err
}

// recordMigrationRolledBack removes a migration from the tracking table
func recordMigrationRolledBack(tx *sql.Tx, migration Migration) error {
	_, err := tx.Exec("DELETE FROM schema_migrations WHERE version = $1", migration.File)
	return err
}

// runMigrationsUp applies all pending up migrations
func runMigrationsUp(db *sql.DB, migrations []Migration, migrationsDir string) error {
	for _, migration := range migrations {
		// Check if already applied
		applied, err := isMigrationApplied(db, migration)
		if err != nil {
			return fmt.Errorf("failed to check migration status for %s: %w", migration.File, err)
		}
		
		if applied {
			log.Printf("Migration %s already applied (skipping)", migration.File)
			continue
		}

		// Read and execute migration
		migrationPath := filepath.Join(migrationsDir, migration.File)
		sqlContent, err := os.ReadFile(migrationPath)
		if err != nil {
			return fmt.Errorf("failed to read migration file %s: %w", migration.File, err)
		}

		log.Printf("Running migration: %s", migration.File)
		
		// Execute in a transaction
		tx, err := db.Begin()
		if err != nil {
			return fmt.Errorf("failed to begin transaction: %w", err)
		}

		if _, err := tx.Exec(string(sqlContent)); err != nil {
			tx.Rollback()
			return fmt.Errorf("failed to execute migration %s: %w", migration.File, err)
		}

		// Record migration as applied
		if err := recordMigrationApplied(tx, migration); err != nil {
			tx.Rollback()
			return fmt.Errorf("failed to record migration %s: %w", migration.File, err)
		}

		if err := tx.Commit(); err != nil {
			return fmt.Errorf("failed to commit transaction for %s: %w", migration.File, err)
		}

		log.Printf("Migration %s applied successfully", migration.File)
	}

	return nil
}

// runMigrationsDown rolls back applied migrations in reverse order
func runMigrationsDown(db *sql.DB, migrations []Migration, migrationsDir string) error {
	for _, migration := range migrations {
		// Check if migration is applied
		applied, err := isMigrationApplied(db, migration)
		if err != nil {
			return fmt.Errorf("failed to check migration status for %s: %w", migration.File, err)
		}

		if !applied {
			log.Printf("Migration %s not applied (skipping)", migration.File)
			continue
		}

		// Check if down migration file exists
		migrationPath := filepath.Join(migrationsDir, migration.File)
		if _, err := os.Stat(migrationPath); os.IsNotExist(err) {
			log.Printf("Down migration %s not found, skipping", migration.File)
			continue
		}

		// Read and execute down migration
		sqlContent, err := os.ReadFile(migrationPath)
		if err != nil {
			return fmt.Errorf("failed to read migration file %s: %w", migration.File, err)
		}

		log.Printf("Rolling back migration: %s", migration.File)

		// Execute in a transaction
		tx, err := db.Begin()
		if err != nil {
			return fmt.Errorf("failed to begin transaction: %w", err)
		}

		if _, err := tx.Exec(string(sqlContent)); err != nil {
			tx.Rollback()
			return fmt.Errorf("failed to execute down migration %s: %w", migration.File, err)
		}

		// Remove migration from tracking
		if err := recordMigrationRolledBack(tx, migration); err != nil {
			tx.Rollback()
			return fmt.Errorf("failed to record migration rollback %s: %w", migration.File, err)
		}

		if err := tx.Commit(); err != nil {
			return fmt.Errorf("failed to commit transaction for %s: %w", migration.File, err)
		}

		log.Printf("Migration %s rolled back successfully", migration.File)
	}

	return nil
}

// showMigrationStatus displays the status of all migrations
func showMigrationStatus(db *sql.DB, migrations []Migration) error {
	log.Println("Migration status:")

	for _, migration := range migrations {
		applied, err := isMigrationApplied(db, migration)
		if err != nil {
			return fmt.Errorf("failed to check migration status for %s: %w", migration.File, err)
		}

		if applied {
			log.Printf("  ✓ %s - Applied", migration.File)
		} else {
			log.Printf("  ✗ %s - Not applied", migration.File)
		}
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
		strings.Contains(errStr, "relation") && strings.Contains(errStr, "already exists")
}
