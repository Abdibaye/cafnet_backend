# Cafnet Repository Layer Architecture
**Phase:** BACKEND 03 — Go Domain + Repository Data-Access Layer  
**Target Stack:** Go 1.27+, Neon PostgreSQL, `pgx/v5`  

---

## 1. Architectural Philosophy & Boundaries

The Cafnet repository layer serves as the authoritative persistence boundary between the domain business logic and the PostgreSQL database. It strictly isolates SQL statements, relational table structures, and driver-specific operations away from higher-level services.

```
       ┌────────────────────────────────────────────────────────┐
       │             Domain Services / Application              │
       │  (Pure Go business rules, validations, orchestrations) │
       └──────────────────────────┬─────────────────────────────┘
                                  │
                                  ▼  (Context & Domain Interfaces)
       ┌────────────────────────────────────────────────────────┐
       │             Repository Interface Layer                 │
       │    (Contracts: UserRepository, BusinessRepository...)   │
       └──────────────────────────┬─────────────────────────────┘
                                  │
                                  ▼  (Implements interfaces via pgx/v5)
       ┌────────────────────────────────────────────────────────┐
       │         PostgreSQL Data Access Implementations         │
       │    (Parameterized SQL, DBTX transactions, Mapping)     │
       └──────────────────────────┬─────────────────────────────┘
                                  │
                                  ▼  (Connection Pool / TLS Session)
       ┌────────────────────────────────────────────────────────┐
       │              Neon PostgreSQL 18.6 Database             │
       │    (19 tables, Constraints, GIN Indexes, Triggers)     │
       └────────────────────────────────────────────────────────┘
```

### Architectural Principles:
1. **Zero Leaked Relational Details:** Domain models use native Go types (`UserID`, `BusinessID`, minor-unit integers `PriceETBMinor`, RFC3339 `time.Time`). SQL row scans and table aliases never leak into domain models.
2. **Explicit Column Projections:** No `SELECT *` queries exist in any repository. Every column is explicitly projected to prevent schema drift bugs and optimize network serialization.
3. **Context-Driven Concurrency:** Every repository method requires `context.Context` as its first parameter to honor query timeouts, cancellations, and transaction lifecycles.
4. **Transparent Transaction Injection:** Atomic multi-table updates are orchestrated via `TxManager`, which passes transaction context transparently without altering repository method signatures.
5. **Deterministic Error Translation:** Raw `pgconn.PgError` instances and SQLSTATE codes are translated into standardized domain error sentinels (`domain.ErrConflict`, `domain.ErrNotFound`, `domain.ErrConstraintViolation`, `domain.ErrIllegalStateTransition`).

---

## 2. Context-Injected Transaction Management

To execute multiple repository operations within a single atomic database transaction without coupling repositories to one another, Cafnet utilizes a context-injected `TxManager`.

### The `DBTX` Interface
Repositories interact with the database through the `DBTX` interface, which is satisfied by both `*pgxpool.Pool` and `pgx.Tx`:

```go
type DBTX interface {
    Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
    Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
    QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}
```

### Context Extraction
The helper `GetDB(ctx context.Context, pool *pgxpool.Pool) DBTX` inspects the active `context.Context`. If a transaction key is present, it returns the active `pgx.Tx`; otherwise, it falls back to the connection pool:

```go
func (tm *txManager) WithinTransaction(ctx context.Context, fn func(ctx context.Context) error) error {
    tx, err := tm.pool.Begin(ctx)
    if err != nil {
        return MapPostgresError(err, "Transaction")
    }
    defer tx.Rollback(ctx)

    txCtx := context.WithValue(ctx, txKey{}, tx)
    if err := fn(txCtx); err != nil {
        return err
    }
    return tx.Commit(ctx)
}
```

This guarantees complete ACID rollback semantics if any error occurs inside the transaction block, verified by automated rollback tests.

---

## 3. Structured Error Mapping

The repository layer maps all PostgreSQL error codes into domain sentinels via `MapPostgresError`:

| PostgreSQL Condition | SQLSTATE | Domain Error Sentinel | Resolution Rationale |
|---|---|---|---|
| `pgx.ErrNoRows` | N/A | `domain.ErrNotFound` | Target entity does not exist or has been soft-deleted |
| `unique_violation` | `23505` | `domain.ErrConflict` | Phone number, slug, reservation code, or unique pair collision |
| `foreign_key_violation` | `23503` | `domain.ErrConstraintViolation` | Non-existent parent entity reference (e.g. unknown business ID) |
| `restrict_violation` | `23001` | `domain.ErrConstraintViolation` | Prevented deletion of parent record with active dependants |
| `check_violation` (State guard) | `23514` | `domain.ErrIllegalStateTransition` | Disallowed lifecycle jump (e.g. `COMPLETED` -> `PENDING`) |
| `check_violation` (Review guard)| `23514` | `domain.ErrConstraintViolation` | Attempted review creation on a non-`COMPLETED` reservation |
| `check_violation` (General) | `23514` | `domain.ErrInvalidInput` | Rating out of bounds [1-5], negative price, invalid party size |
| Connection drop | `08000`, `57P01` | `domain.ErrDatabaseUnavailable` | Transient network or connection termination |

All mapped errors are wrapped in `*domain.DomainError`, allowing consumers to use `errors.Is(err, domain.ErrNotFound)` or inspect detailed messages.

---

## 4. Canonical Repository Implementations

### 4.1 UserRepository (`postgres_user.go`)
- **Table:** `users`
- **Methods:** `Create`, `GetByID`, `GetByPhone`, `Update`
- **Guarantees:** Enforces E.164 phone uniqueness (`unique_user_phone`). Updates automatically refresh `updated_at`. Soft-deleted or inactive users are tracked according to domain rules.

### 4.2 BusinessRepository (`postgres_business.go`)
- **Tables:** `businesses`, `business_hours`, `business_images`
- **Methods:** `Create`, `GetByID`, `GetBySlug`, `Update`, `SetPublicationStatus`, `SetVerificationStatus`, `SetClaimStatus`, `List`, `Search`, `GetHours`, `SetHours`, `GetImages`, `SetImages`, `SoftDelete`
- **Guarantees:**
  - Strict orthogonality: `publication_status` (`PUBLISHED`, `UNPUBLISHED`), `verification_status` (`VERIFIED`, `UNVERIFIED`), and `claim_status` (`CLAIMED`, `UNCLAIMED`) mutate independently.
  - Search queries leverage the stored PostgreSQL GIN index `search_vector` (`setweight` on name, category, cuisine, neighborhood, and description).
  - Business hours and images are replaced atomically in batches within the active transaction.
  - Soft-deletion sets `deleted_at = NOW()`, filtering it out from public listings and queries.

### 4.3 OwnershipRepository (`postgres_ownership.go`)
- **Table:** `business_managers`
- **Methods:** `AddManager`, `RemoveManager`, `IsManager`, `GetManagers`, `GetManagedBusinesses`
- **Guarantees:** Enforces composite uniqueness on `(business_id, user_id)`. Distinguishes between `PRIMARY_OWNER` and `MANAGER` roles.

### 4.4 ClaimRepository (`postgres_claim.go`)
- **Table:** `business_claims`
- **Methods:** `Create`, `GetByID`, `GetPendingForBusiness`, `ListPending`, `UpdateStatus`
- **Guarantees:** Prevents duplicate pending claims on the same venue via the partial unique index `idx_unique_pending_business_claim` (`WHERE status = 'PENDING'`).

### 4.5 SubmissionRepository (`postgres_submission.go`)
- **Table:** `business_submissions`
- **Methods:** `Create`, `GetByID`, `ListPending`, `UpdateStatus`
- **Guarantees:** Stores complete discovery payload as structured JSONB. Status progression to `APPROVED` links the created `BusinessID` atomically.

### 4.6 MenuRepository (`postgres_menu.go`)
- **Tables:** `menu_categories`, `menu_items`
- **Methods:**
  - Categories: `CreateCategory`, `GetCategoryByID`, `UpdateCategory`, `DeleteCategory`, `ReorderCategories`, `ListCategories`
  - Items: `CreateItem`, `GetItemByID`, `UpdateItem`, `DeleteItem`, `ReorderItems`, `ListItemsByCategory`
  - Aggregate: `GetFullMenu`
- **Guarantees:**
  - Currency integrity: MenuItem pricing is stored as 64-bit integer minor units (`price_etb_minor`), avoiding IEEE floating-point imprecision.
  - Soft-deleting a category cascades soft-deletion to all associated items.
  - `GetFullMenu` executes a single optimized query grouping items under categories in memory.

### 4.7 UpdateRepository (`postgres_update.go`)
- **Table:** `business_updates`
- **Methods:** `Create`, `GetByID`, `ListActive`, `ListByBusiness`, `Update`, `Delete`
- **Guarantees:**
  - `ListActive` enforces `status = 'PUBLISHED' AND deleted_at IS NULL AND (expires_at IS NULL OR expires_at > NOW())` at the SQL level.
  - Supports chronological feeds ordered by `published_at DESC`.

### 4.8 ReservationRepository (`postgres_reservation.go`)
- **Table:** `reservations`
- **Methods:** `Create`, `GetByID`, `GetByCode`, `List`, `Confirm`, `Decline`, `Cancel`, `Complete`
- **Guarantees:**
  - Status progression is validated and guarded by PostgreSQL trigger `trg_reservations_status_guard`.
  - Illegal transitions (such as transitioning from `CANCELLED` or `COMPLETED`) raise `check_violation` and return `domain.ErrIllegalStateTransition`.
  - Declining a reservation requires a non-empty decline reason.
  - Timestamps (`confirmed_at`, `declined_at`, `completed_at`, `cancelled_at`) are automatically managed.

### 4.9 ReviewRepository (`postgres_review.go`)
- **Tables:** `reviews`, `review_helpful_votes`
- **Methods:** `Create`, `GetByID`, `GetByReservationID`, `ListForBusiness`, `ListForCustomer`, `Update`, `SoftDelete`, `SetModerationStatus`, `GetSummary`, `AddHelpfulVote`, `RemoveHelpfulVote`, `HasVotedHelpful`
- **Guarantees:**
  - Verified Dining Integrity: Database trigger `trg_reviews_reservation_completed` forbids review creation unless the corresponding reservation has reached status `COMPLETED`.
  - Aggregated Statistics: `GetSummary` executes a single aggregation query computing `total_count`, `average` rating, and 5-star rating distribution (`[5]int`) using SQL filter clauses.
  - One review per reservation enforced by `unique_reservation_review`.
  - Helpful voting enforces one vote per user per review with immediate count derivation.

### 4.10 ReportRepository (`postgres_report.go`)
- **Tables:** `review_reports`, `business_reports`
- **Methods:** `CreateReviewReport`, `GetReviewReportByID`, `ListPendingReviewReports`, `ResolveReviewReport`, `CreateBusinessReport`, `GetBusinessReportByID`, `ListPendingBusinessReports`, `ResolveBusinessReport`
- **Guarantees:** Enforces unique reporting (`unique_reporter_review`) preventing community spam. Stores resolution timestamps, actions, and administrative notes.

### 4.11 CustomerListRepository (`postgres_customer_list.go`)
- **Tables:** `saved_businesses`, `followed_businesses`
- **Methods:** `SaveBusiness`, `UnsaveBusiness`, `IsSaved`, `ListSavedBusinesses`, `FollowBusiness`, `UnfollowBusiness`, `IsFollowing`, `ListFollowedBusinesses`
- **Guarantees:** Idempotent bookmarking via `ON CONFLICT (user_id, business_id) DO NOTHING`. Returns full business entities joined in a single query.

### 4.12 AdminActionRepository (`postgres_admin.go`)
- **Table:** `admin_actions`
- **Methods:** `Record`, `List`
- **Guarantees:** Append-only immutable audit trail. Records administrative IDs, action types, entity references, and arbitrary metadata payloads via JSONB.

---

## 5. Verification & Test Suite Summary

The repository layer is tested against a live Neon PostgreSQL 18.6 instance. All 20 integration tests pass:

```
=== RUN   TestUserRepository                      --- PASS (4.86s)
=== RUN   TestBusinessRepository                  --- PASS (11.10s)
=== RUN   TestOwnershipClaimSubmissionRepositories --- PASS (8.36s)
=== RUN   TestMenuRepository                      --- PASS (8.72s)
=== RUN   TestUpdateRepository                    --- PASS (5.70s)
=== RUN   TestReservationAndReviewLifecycle       --- PASS (8.59s)
=== RUN   TestCustomerListsAndReports             --- PASS (8.57s)
=== RUN   TestAdminActionRepository               --- PASS (3.23s)
=== RUN   TestTxManagerAtomicityAndRollback       --- PASS (4.66s)
=== RUN   TestInvariant_DuplicateSaveRejected     --- PASS (3.56s)
=== RUN   TestInvariant_DuplicateFollowRejected   --- PASS (3.71s)
=== RUN   TestInvariant_ReviewReservationIntegrity--- PASS (5.81s)
=== RUN   TestInvariant_ReviewHelpfulVoteUniqueness-- PASS (5.00s)
=== RUN   TestInvariant_InvalidRatingRejected     --- PASS (4.52s)
=== RUN   TestInvariant_InvalidPartySizeRejected  --- PASS (3.90s)
=== RUN   TestInvariant_InvalidStateValueRejected --- PASS (2.37s)
=== RUN   TestInvariant_BusinessManagerUniqueness --- PASS (3.77s)
=== RUN   TestInvariant_PendingClaimUniqueness    --- PASS (3.92s)
=== RUN   TestInvariant_ReservationStateMachineTransitions --- PASS (5.73s)
=== RUN   TestInvariant_ForeignKeyIntegrityReservationReview --- PASS (5.30s)
PASS (ok github.com/Abdibaye/cafnet_backend/tests 111.385s)
```
