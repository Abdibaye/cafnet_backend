# CAFNET DATABASE MIGRATION WORKFLOW
**Document ID:** `CAFNET-BACKEND-02-MIGRATIONS`  
**Status:** Operational Guide  
**Engine:** Neon Serverless PostgreSQL  
**Tool:** Cafnet Go Migration Runner (`cmd/migrate/main.go`)  

---

## 1. Migration System Overview

Cafnet uses a native, zero-dependency Go migration runner built on `pgx/v5` and versioned SQL files located in `backend/migrations/`.

### Key Characteristics:
1. **Plain SQL Files:** Uses standard `.up.sql` and `.down.sql` files compatible with `golang-migrate`.
2. **Transactional Execution:** Every migration runs inside an explicit database transaction (`BEGIN ... COMMIT`). If any statement fails, the entire migration aborts and rolls back cleanly without leaving orphan tables.
3. **Audit Ledger:** Tracks applied schema versions in `schema_migrations(version, applied_at)`.
4. **Neon Serverless Compatible:** Operates seamlessly over SSL/TLS pooled connections (`?sslmode=require`) without requiring superuser privileges.

---

## 2. Directory Structure

```
backend/
├── cmd/
│   └── migrate/
│       └── main.go       # Migration CLI runner
├── migrations/
│   ├── 000001_initial_schema.up.sql    # Schema creation, constraints, indexes, triggers
│   └── 000001_initial_schema.down.sql  # Clean rollback script
├── pkg/
│   └── db/
│       └── db.go         # pgx connection pool and config loader
└── tests/
    └── schema_test.go    # Invariant and constraint automated tests
```

---

## 3. CLI Commands & Usage

From the `backend/` directory:

### 3.1 Check Migration Status
Displays which migrations have been applied and which are pending:
```bash
go run cmd/migrate/main.go status
```

### 3.2 Apply Pending Migrations (`up`)
Applies all pending migrations in deterministic ascending order:
```bash
go run cmd/migrate/main.go up
```

### 3.3 Revert Last Migration (`down`)
Reverts the highest applied migration version using its `.down.sql` script:
```bash
go run cmd/migrate/main.go down
```

### 3.4 Print Current Schema Version
```bash
go run cmd/migrate/main.go version
```

---

## 4. Neon Development Workflow

### 4.1 Environment Configuration
1. Copy `.env.example` to `.env`:
   ```bash
   cp .env.example .env
   ```
2. Set your Neon connection string in `.env`:
   ```env
   DATABASE_URL=postgresql://neondb_owner:[PASSWORD]@[ENDPOINT]-pooler.[REGION].aws.neon.tech/neondb?sslmode=require
   ```
   *(Note: `.env` is strictly excluded by `.gitignore` to prevent credential leaks).*

### 4.2 Creating a New Migration
When schema changes are needed in future backend updates:
1. Create a pair of zero-padded SQL files:
   - `migrations/000002_your_change_name.up.sql`
   - `migrations/000002_your_change_name.down.sql`
2. Write the forward migration in `.up.sql`.
3. Write the exact reverse teardown in `.down.sql`.
4. Run `go run cmd/migrate/main.go up` to test application.
5. Run `go run cmd/migrate/main.go down` and then `up` to verify reversibility.
6. Run `go test -v ./tests` to verify that invariants hold.

---

## 5. Rollback Strategy & Production Safeguards

1. **Transactional Boundaries:** If a syntax error, constraint violation, or network timeout occurs mid-migration, the database transaction automatically rolls back. The version is never recorded in `schema_migrations` unless commit succeeds.
2. **Down Migrations Required:** Every `.up.sql` file must have a corresponding `.down.sql` file committed alongside it. Pull requests adding schema migrations without down migrations are rejected.
3. **Destructive Operations:** Columns or tables are NEVER dropped in `.up.sql` without a deprecation phase. Columns to be removed must first be marked deprecated or nullable before removal.
