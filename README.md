# Cafnet Backend

High-performance Go REST API backend and domain service for Cafnet, powered by Neon Serverless PostgreSQL and S3-compatible media storage.

## Architecture & Technology

- **Language & Runtime**: Go 1.23+
- **HTTP Routing**: Go standard library `net/http` (`/api/v1`)
- **Database Access**: `jackc/pgx/v5` connection pool with Neon PostgreSQL
- **Security & Auth**: E.164 phone normalization, cryptographic OTP, SHA-256 opaque session tokens, HTTP-only SameSite cookies
- **Vector Search**: `pgvector` for AI assistant semantic restaurant & menu discovery
- **Object Storage**: S3-compatible cloud storage (Tigris / Cloudflare R2 / AWS S3) via AWS SDK v2

## Directory Layout

- `cmd/server/` - Main HTTP API entrypoint
- `cmd/migrate/` - Database migration runner (`up`, `down`, `status`, `version`)
- `cmd/admin/` - Administrative CLI tooling (role promotion, taxonomy seeding)
- `cmd/ingest/` - Bulk ingestion scripts
- `internal/domain/` - Domain entities, data invariants, state machines, and sentinel errors
- `internal/http/` - REST handlers, routers, and security middleware
- `internal/repository/` - Database queries, transaction management (`TxManager`), and pgx error mapping
- `internal/service/` - Business logic layer (Discovery, Reservations, Reviews, Menus, Assistant)
- `internal/storage/` - Object storage drivers
- `migrations/` - SQL migration files (000001 - 000009)
- `docs/` - Exhaustive architectural specifications and domain models (see [docs/README.md](docs/README.md))

## Getting Started

### 1. Configure Environment
Copy `.env.example` and set your database connection string:
```bash
cp .env.example .env
```

### 2. Run Database Migrations
```bash
go run ./cmd/migrate up
```

### 3. Start the Server
```bash
go run ./cmd/server
```
The server listens on `http://localhost:8080`.

### 4. Run Tests
```bash
go test -v ./...
```

For complete domain documentation, state machine diagrams, and database schemas, see [docs/README.md](docs/README.md).
