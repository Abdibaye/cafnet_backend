package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/joho/godotenv"

	"github.com/Abdibaye/cafnet_backend/pkg/db"
)

type migrationFile struct {
	version   int64
	name      string
	direction string // "up" or "down"
	filePath  string
}

func main() {
	_ = godotenv.Load()

	migrationsDir := flag.String("path", "migrations", "Path to SQL migrations directory")
	flag.Parse()

	args := flag.Args()
	if len(args) == 0 {
		printUsage()
		os.Exit(1)
	}

	command := args[0]

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	cfg := db.LoadConfigFromEnv()
	pool, err := db.Connect(ctx, cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error connecting to Neon database: %v\n", err)
		os.Exit(1)
	}
	defer pool.Close()

	if err := ensureMigrationsTable(ctx, pool); err != nil {
		fmt.Fprintf(os.Stderr, "Error initializing migrations table: %v\n", err)
		os.Exit(1)
	}

	switch command {
	case "up":
		if err := runUp(ctx, pool, *migrationsDir); err != nil {
			fmt.Fprintf(os.Stderr, "Migration failed: %v\n", err)
			os.Exit(1)
		}
	case "down":
		if err := runDown(ctx, pool, *migrationsDir); err != nil {
			fmt.Fprintf(os.Stderr, "Rollback failed: %v\n", err)
			os.Exit(1)
		}
	case "status":
		if err := printStatus(ctx, pool, *migrationsDir); err != nil {
			fmt.Fprintf(os.Stderr, "Status check failed: %v\n", err)
			os.Exit(1)
		}
	case "version":
		v, err := getLatestAppliedVersion(ctx, pool)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to get version: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Current schema version: %d\n", v)
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", command)
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println("Cafnet Neon PostgreSQL Migration Tool")
	fmt.Println("\nUsage:")
	fmt.Println("  go run cmd/migrate/main.go [options] <command>")
	fmt.Println("\nCommands:")
	fmt.Println("  up      Apply all pending migrations")
	fmt.Println("  down    Revert the last applied migration")
	fmt.Println("  status  Show migration status and history")
	fmt.Println("  version Print current applied schema version")
	fmt.Println("\nOptions:")
	fmt.Println("  -path string  Path to migrations directory (default: migrations)")
}

func ensureMigrationsTable(ctx context.Context, pool *db.Pool) error {
	query := `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version BIGINT PRIMARY KEY,
			applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);
	`
	_, err := pool.Exec(ctx, query)
	return err
}

func getAppliedVersions(ctx context.Context, pool *db.Pool) (map[int64]time.Time, error) {
	rows, err := pool.Query(ctx, "SELECT version, applied_at FROM schema_migrations ORDER BY version ASC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	applied := make(map[int64]time.Time)
	for rows.Next() {
		var v int64
		var t time.Time
		if err := rows.Scan(&v, &t); err != nil {
			return nil, err
		}
		applied[v] = t
	}
	return applied, nil
}

func getLatestAppliedVersion(ctx context.Context, pool *db.Pool) (int64, error) {
	var version int64
	err := pool.QueryRow(ctx, "SELECT COALESCE(MAX(version), 0) FROM schema_migrations").Scan(&version)
	return version, err
}

func scanMigrationFiles(dir string) ([]migrationFile, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("reading migrations directory: %w", err)
	}

	var files []migrationFile
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sql") {
			continue
		}

		parts := strings.Split(entry.Name(), ".")
		if len(parts) < 3 {
			continue
		}

		namePart := parts[0]
		direction := parts[1] // "up" or "down"

		subparts := strings.SplitN(namePart, "_", 2)
		if len(subparts) < 2 {
			continue
		}

		v, err := strconv.ParseInt(subparts[0], 10, 64)
		if err != nil {
			continue
		}

		files = append(files, migrationFile{
			version:   v,
			name:      entry.Name(),
			direction: direction,
			filePath:  filepath.Join(dir, entry.Name()),
		})
	}

	sort.Slice(files, func(i, j int) bool {
		if files[i].version != files[j].version {
			return files[i].version < files[j].version
		}
		return files[i].direction > files[j].direction
	})

	return files, nil
}

func runUp(ctx context.Context, pool *db.Pool, dir string) error {
	applied, err := getAppliedVersions(ctx, pool)
	if err != nil {
		return err
	}

	files, err := scanMigrationFiles(dir)
	if err != nil {
		return err
	}

	count := 0
	for _, f := range files {
		if f.direction != "up" {
			continue
		}

		if _, exists := applied[f.version]; exists {
			continue // Already applied
		}

		fmt.Printf("Applying migration %s...\n", f.name)
		content, err := os.ReadFile(f.filePath)
		if err != nil {
			return fmt.Errorf("failed to read %s: %w", f.filePath, err)
		}

		// Execute migration inside an explicit database transaction
		tx, err := pool.BeginTx(ctx, pgx.TxOptions{})
		if err != nil {
			return fmt.Errorf("failed to begin transaction: %w", err)
		}

		if _, err := tx.Exec(ctx, string(content)); err != nil {
			_ = tx.Rollback(ctx)
			return fmt.Errorf("error executing %s: %w", f.name, err)
		}

		if _, err := tx.Exec(ctx, "INSERT INTO schema_migrations (version) VALUES ($1)", f.version); err != nil {
			_ = tx.Rollback(ctx)
			return fmt.Errorf("failed to record migration %d: %w", f.version, err)
		}

		if err := tx.Commit(ctx); err != nil {
			return fmt.Errorf("failed to commit migration transaction: %w", err)
		}

		fmt.Printf("Successfully applied %s\n", f.name)
		count++
	}

	if count == 0 {
		fmt.Println("Database is already up to date. No migrations to apply.")
	} else {
		fmt.Printf("Completed applying %d migration(s).\n", count)
	}
	return nil
}

func runDown(ctx context.Context, pool *db.Pool, dir string) error {
	latest, err := getLatestAppliedVersion(ctx, pool)
	if err != nil {
		return err
	}
	if latest == 0 {
		fmt.Println("No migrations have been applied. Nothing to revert.")
		return nil
	}

	files, err := scanMigrationFiles(dir)
	if err != nil {
		return err
	}

	var targetFile *migrationFile
	for _, f := range files {
		if f.version == latest && f.direction == "down" {
			targetFile = &f
			break
		}
	}

	if targetFile == nil {
		return fmt.Errorf("down migration file not found for version %06d", latest)
	}

	fmt.Printf("Reverting migration %s (version %d)...\n", targetFile.name, latest)
	content, err := os.ReadFile(targetFile.filePath)
	if err != nil {
		return fmt.Errorf("failed to read %s: %w", targetFile.filePath, err)
	}

	tx, err := pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin rollback transaction: %w", err)
	}

	if _, err := tx.Exec(ctx, string(content)); err != nil {
		_ = tx.Rollback(ctx)
		return fmt.Errorf("error rolling back %s: %w", targetFile.name, err)
	}

	if _, err := tx.Exec(ctx, "DELETE FROM schema_migrations WHERE version = $1", latest); err != nil {
		_ = tx.Rollback(ctx)
		return fmt.Errorf("failed to remove version %d from schema_migrations: %w", latest, err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("failed to commit rollback transaction: %w", err)
	}

	fmt.Printf("Successfully reverted version %d.\n", latest)
	return nil
}

func printStatus(ctx context.Context, pool *db.Pool, dir string) error {
	applied, err := getAppliedVersions(ctx, pool)
	if err != nil {
		return err
	}

	files, err := scanMigrationFiles(dir)
	if err != nil {
		return err
	}

	fmt.Println("Migration Status:")
	fmt.Println("==========================================================================")
	fmt.Printf("%-10s %-45s %-12s\n", "Version", "Migration", "Status")
	fmt.Println("--------------------------------------------------------------------------")

	seen := make(map[int64]bool)
	for _, f := range files {
		if f.direction != "up" || seen[f.version] {
			continue
		}
		seen[f.version] = true

		if t, ok := applied[f.version]; ok {
			fmt.Printf("%-10d %-45s Applied (%s)\n", f.version, f.name, t.Format("2006-01-02 15:04:05"))
		} else {
			fmt.Printf("%-10d %-45s Pending\n", f.version, f.name)
		}
	}
	fmt.Println("==========================================================================")
	return nil
}
