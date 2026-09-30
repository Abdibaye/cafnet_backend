package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/joho/godotenv"

	"github.com/Abdibaye/cafnet_backend/internal/config"
	"github.com/Abdibaye/cafnet_backend/internal/domain"
	"github.com/Abdibaye/cafnet_backend/pkg/db"
)

func main() {
	_ = godotenv.Load()

	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	command := os.Args[1]

	cfg, err := config.LoadServerConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading configuration: %v\n", err)
		os.Exit(1)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	pool, err := db.Connect(ctx, cfg.DB)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to connect to database: %v\n", err)
		os.Exit(1)
	}
	defer pool.Close()

	switch command {
	case "list":
		rows, err := pool.Query(ctx, "SELECT id, phone, full_name, role, is_active, created_at FROM users WHERE role = 'ADMIN' ORDER BY created_at DESC")
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to query admin users: %v\n", err)
			os.Exit(1)
		}
		defer rows.Close()

		fmt.Println("Cafnet Platform Administrators:")
		fmt.Println("--------------------------------------------------------------------------------")
		count := 0
		for rows.Next() {
			count++
			var id, phone, name, role string
			var active bool
			var createdAt time.Time
			if err := rows.Scan(&id, &phone, &name, &role, &active, &createdAt); err != nil {
				continue
			}
			fmt.Printf("[%d] Phone: %s | Name: %s | Active: %v | Created: %s | ID: %s\n", count, phone, name, active, createdAt.Format("2006-01-02"), id)
		}
		if count == 0 {
			fmt.Println("No admin users found.")
		}

	case "promote":
		if len(os.Args) < 3 {
			fmt.Println("Usage: go run ./cmd/admin promote <phone>")
			os.Exit(1)
		}
		rawPhone := os.Args[2]
		normalized, err := domain.NormalizePhone(rawPhone)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Invalid phone format: %v\n", err)
			os.Exit(1)
		}

		res, err := pool.Exec(ctx, "UPDATE users SET role = 'ADMIN', updated_at = NOW() WHERE phone = $1", normalized)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to promote user: %v\n", err)
			os.Exit(1)
		}

		if res.RowsAffected() == 0 {
			fmt.Printf("No existing user found with phone %s.\nTo create a new admin account directly, run:\n  go run ./cmd/admin create %s \"Admin Name\"\n", normalized, normalized)
		} else {
			fmt.Printf("Successfully promoted user %s to ADMIN.\n", normalized)
		}

	case "create":
		fs := flag.NewFlagSet("create", flag.ExitOnError)
		if len(os.Args) < 3 {
			fmt.Println("Usage: go run ./cmd/admin create <phone> [name]")
			os.Exit(1)
		}
		rawPhone := os.Args[2]
		name := "Cafnet Administrator"
		if len(os.Args) >= 4 && !strings.HasPrefix(os.Args[3], "-") {
			name = os.Args[3]
		}
		_ = fs.Parse(os.Args[3:])

		normalized, err := domain.NormalizePhone(rawPhone)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Invalid phone format: %v\n", err)
			os.Exit(1)
		}

		var id string
		err = pool.QueryRow(ctx, `
			INSERT INTO users (phone, full_name, role, is_active, created_at, updated_at)
			VALUES ($1, $2, 'ADMIN', TRUE, NOW(), NOW())
			ON CONFLICT (phone) DO UPDATE 
			SET role = 'ADMIN', full_name = EXCLUDED.full_name, updated_at = NOW()
			RETURNING id
		`, normalized, name).Scan(&id)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to create/set admin user: %v\n", err)
			os.Exit(1)
		}

	case "set-sole":
		if len(os.Args) < 3 {
			fmt.Println("Usage: go run ./cmd/admin set-sole <phone> [name]")
			os.Exit(1)
		}
		rawPhone := os.Args[2]
		name := "Cafnet Administrator"
		if len(os.Args) >= 4 && !strings.HasPrefix(os.Args[3], "-") {
			name = os.Args[3]
		}

		normalized, err := domain.NormalizePhone(rawPhone)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Invalid phone format: %v\n", err)
			os.Exit(1)
		}

		// 1. Demote all other admins to CUSTOMER
		demoted, err := pool.Exec(ctx, "UPDATE users SET role = 'CUSTOMER', updated_at = NOW() WHERE role = 'ADMIN' AND phone != $1", normalized)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to demote other admins: %v\n", err)
			os.Exit(1)
		}

		// 2. Ensure target user is ADMIN
		var id string
		err = pool.QueryRow(ctx, `
			INSERT INTO users (phone, full_name, role, is_active, created_at, updated_at)
			VALUES ($1, $2, 'ADMIN', TRUE, NOW(), NOW())
			ON CONFLICT (phone) DO UPDATE 
			SET role = 'ADMIN', full_name = EXCLUDED.full_name, updated_at = NOW()
			RETURNING id
		`, normalized, name).Scan(&id)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to set sole admin user: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("Sole admin established successfully!\n")
		fmt.Printf("  Demoted %d previous admin accounts to CUSTOMER.\n", demoted.RowsAffected())
		fmt.Printf("  Active Admin: %s (%s) [ID: %s]\n", normalized, name, id)

	case "list-submissions":
		rows, err := pool.Query(ctx, `
			SELECT s.id, s.submitter_user_id, u.phone, u.full_name, s.business_name, s.category, s.area, s.status, s.created_at 
			FROM business_submissions s
			LEFT JOIN users u ON u.id = s.submitter_user_id
			ORDER BY s.created_at DESC
		`)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to query submissions: %v\n", err)
			os.Exit(1)
		}
		defer rows.Close()

		fmt.Println("Business Submissions:")
		fmt.Println("--------------------------------------------------------------------------------")
		count := 0
		for rows.Next() {
			count++
			var id, subId, status, name, cat, area string
			var phone, userName *string
			var createdAt time.Time
			if err := rows.Scan(&id, &subId, &phone, &userName, &name, &cat, &area, &status, &createdAt); err != nil {
				fmt.Printf("Scan error: %v\n", err)
				continue
			}
			pStr := "<none>"
			if phone != nil {
				pStr = *phone
			}
			uStr := "<none>"
			if userName != nil {
				uStr = *userName
			}
			fmt.Printf("[%d] Sub ID: %s | Business: %s | Cat: %s | Area: %s | Status: %s | Submitter: %s (%s) | Date: %s\n",
				count, id, name, cat, area, status, pStr, uStr, createdAt.Format("2006-01-02 15:04:05"))
		}
		if count == 0 {
			fmt.Println("No submissions found in database.")
		}

	default:
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println("Cafnet Admin Management CLI")
	fmt.Println("Usage:")
	fmt.Println("  go run ./cmd/admin list                  List all platform admins")
	fmt.Println("  go run ./cmd/admin promote <phone>       Promote an existing user to ADMIN")
	fmt.Println("  go run ./cmd/admin create <phone> [name] Create or upsert an ADMIN user")
	fmt.Println("  go run ./cmd/admin set-sole <phone> [name] Remove all other admins and set single ADMIN")
}
