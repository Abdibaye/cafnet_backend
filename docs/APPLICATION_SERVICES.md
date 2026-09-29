# Cafnet Application Service Layer (Backend 06)

## 1. Architectural Role & Layering

The application service layer orchestrates Cafnet business use cases. It sits strictly between the future transport layer (HTTP/API / REST controllers) and the persistence layer (Repositories & Neon PostgreSQL).

```
Transport Layer (HTTP / Handlers / REST Controllers) [Backend 07]
                       ↓
Identity & Authentication Context (User, Session) [Backend 04]
                       ↓
Security & Ownership Authorization (ReBAC / RBAC) [Backend 05]
                       ↓
Application Services (Discovery, Customer, Business, Menu, Updates, Reservations, Reviews, Reports, Admin) [Backend 06]
                       ↓
Domain Models & Invariant State Machines [Backend 01 / 03]
                       ↓
PostgreSQL Repositories & TxManager [Backend 02 / 03]
                       ↓
Neon PostgreSQL (Serverless AWS us-east-2)
```

### Architectural Principles:
1. **Transport Agnostic**: Services receive strongly-typed Go domain models (`*domain.User`, IDs, and DTO inputs). They never parse HTTP requests, query strings, headers, or cookies.
2. **Explicit Dependency Injection**: Repositories, transaction managers, and security services are injected via constructor functions. No global state, package singletons, or hidden database connections.
3. **No Direct SQL**: Services never execute raw SQL queries. All persistence is delegated to repository interfaces.
4. **Authoritative Authorization**: Services invoke `AuthorizationService` for ReBAC/RBAC enforcement on every privileged operation. Services do not duplicate permission logic.
5. **Atomic Transaction Orchestration**: Multi-step state transitions and mutations are governed by `repository.TxManager.WithinTransaction(ctx, func(txCtx) error)`.

---

## 2. Service Boundaries & Catalog

### 2.1 Discovery Service (`service.DiscoveryService`)
Handles public-facing discovery, search, and aggregated profile composition.
- **Contract**:
  - `ListBusinesses(ctx, filter)`: Filters strictly by `PublicationStatus == PUBLISHED`.
  - `GetBusiness(ctx, id)`: Returns `domain.ErrNotFound` if business is unpublished.
  - `GetBusinessBySlug(ctx, slug)`: Slug-based lookup enforcing published status.
  - `SearchBusinesses(ctx, query, limit, offset)`: Full-text search leveraging PostgreSQL GIN indexes.
  - `GetPublicBusinessProfile(ctx, id)`: Composes complete venue profile aggregating business details, opening hours, gallery images, full menu categories/items, active updates, review summary, and recent reviews.

### 2.2 Customer Service (`service.CustomerService`)
Orchestrates authenticated customer interactions:
- **Saved Places**: `SaveBusiness`, `UnsaveBusiness`, `ListSavedBusinesses`, `IsBusinessSaved` (idempotent `ON CONFLICT DO NOTHING`).
- **Followed Venues**: `FollowBusiness`, `UnfollowBusiness`, `ListFollowedBusinesses`, `IsBusinessFollowed`.
- **Venue Claims**: `SubmitClaim` (validates business exists, checks not already claimed, prevents duplicate pending claims), `ListMyClaims`, `GetMyClaim`.
- **Venue Submissions**: `CreateSubmission` (records user suggestion with status `PENDING`), `ListMySubmissions`, `GetMySubmission`.

### 2.3 Business Service (`service.BusinessService`)
Provides venue management capabilities for authorized primary owners and managers:
- **Operations**: `GetManagedBusiness`, `GetManagedBusinesses`, `UpdateBusiness`, `SetHours`, `GetHours`, `SetImages`, `GetImages`.
- **Security**: Every mutation checks `authzService.CanEditBusiness(ctx, actor, businessID)`. Listing managed venues uses `ownershipRepo.GetManagedBusinesses(ctx, actor.ID)` or platform admin oversight.
- **Scope**: Managers cannot alter platform-controlled publication status, claim status, or verification badges.

### 2.4 Menu Service (`service.MenuService`)
Orchestrates category and item management with precise minor-unit pricing:
- **Operations**:
  - Categories: `CreateCategory`, `UpdateCategory`, `DeleteCategory`, `ReorderCategories`.
  - Items: `CreateItem`, `UpdateItem`, `DeleteItem`, `ReorderItems`.
  - Public Menu: `GetFullMenu(ctx, businessID)`.
- **Invariants**:
  - Strict ReBAC via `authzService.CanManageMenu(ctx, actor, businessID)`.
  - IDOR Prevention: Target category and item must belong to the caller's managed `businessID`.
  - Exact Currency: `PriceETBMinor` in Ethiopian Santims (1 ETB = 100 Santims).

### 2.5 Updates Service (`service.UpdatesService`)
Orchestrates time-sensitive broadcast posts:
- **Operations**: `CreateUpdate`, `UpdateUpdate`, `DeleteUpdate`, `ListBusinessUpdates`.
- **Types**: `domain.UpdateTypeAnnouncement`, `domain.UpdateTypeOffer`, `domain.UpdateTypeEvent`.
- **Visibility**: Active updates are public; expired updates are only accessible to authorized business managers and administrators.

### 2.6 Reservation Service (`service.ReservationService`)
Coordinates table reservations under the canonical Request & Confirm dining model:
- **Customer Use Cases**:
  - `CreateReservation`: Validates party size (1–50), generates human-readable alphanumeric confirmation code (`RES-YYYYMMDD-XXXX`), sets status `PENDING`.
  - `GetCustomerReservation` / `ListCustomerReservations`: Scoped strictly to the authenticated diner.
  - `CancelReservation`: Customer cancellation permitted only when reservation is `PENDING` or `CONFIRMED`.
- **Manager Use Cases**:
  - `ListBusinessReservations` / `GetBusinessReservation`: Authorized manager view with date and status filters.
  - `ConfirmReservation`: Atomic state progression `PENDING` → `CONFIRMED`.
  - `DeclineReservation`: Atomic state progression `PENDING` → `DECLINED` requiring mandatory decline reason.
  - `CompleteReservation`: Atomic state progression `CONFIRMED` → `COMPLETED`. Unlocks verified dining review submission!

### 2.7 Review Service (`service.ReviewService`)
Enforces the core Cafnet platform value proposition: **100% Authoritative Verified Dining**:
- **Verified Dining Invariant**:
  - A review CANNOT be submitted without referencing a valid `ReservationID`.
  - `res.CustomerUserID` must match `actor.ID`.
  - `res.Status` must be `COMPLETED`. Any attempt to review an uncompleted or cancelled reservation fails with `domain.ErrIllegalStateTransition`.
  - Duplicate Review Prevention: A completed visit can only be reviewed once (`domain.ErrConflict`).
- **Review Author Control**: Only the original reviewer can edit (`UpdateReview`) or soft-delete (`DeleteReview`) their review.
- **Engagement**: `MarkHelpful`, `UnmarkHelpful`, `HasVotedHelpful` with idempotent duplicate vote handling.

### 2.8 Report Service (`service.ReportService`)
Provides safe community flag submission for suspicious reviews or inaccurate business data:
- `CreateReviewReport`: Reported against an existing review with reason (`spam`, `offensive`, `fake`, `other`).
- `CreateBusinessReport`: Reported against an existing business with reason (`WRONG_HOURS`, `WRONG_PHONE`, `WRONG_LOCATION`, `WRONG_INFORMATION`, `CLOSED`, `DUPLICATE`, `OTHER`).
- Reports default to `PENDING` for administrative triage.

### 2.9 Admin Service (`service.AdminService`)
Unifies platform governance and auditing, delegating to `OwnershipWorkflowService` and repositories:
- **Submissions**: `ListPendingSubmissions`, `ApproveSubmission`, `RejectSubmission`.
- **Claims**: `ListPendingClaims`, `ApproveClaim`, `RejectClaim`.
- **Report Resolution**: `ListPendingReviewReports`, `ResolveReviewReport`, `ListPendingBusinessReports`, `ResolveBusinessReport`.
- **Review Moderation**: `ModerateReview` (takedown / status change).
- **Publication & Badging**: `SetBusinessPublicationStatus`, `SetBusinessVerificationStatus`.
- **Immutable Audit Trail**: Every sensitive administrative mutation automatically records a structured `domain.AdminAction` log entry.

---

## 3. Concurrency & Transaction Boundaries

Multi-step operations are wrapped in transaction closures using `TxManager`:
```go
err := s.txMgr.WithinTransaction(ctx, func(txCtx context.Context) error {
    // 1. Fetch current entity within transaction context
    res, err := s.reservationRepo.GetByID(txCtx, id)
    if err != nil {
        return err
    }
    // 2. Validate state machine transition
    if res.Status != domain.ReservationStatusPending {
        return domain.ErrIllegalStateTransition
    }
    // 3. Mutate entity with reviewer attribution
    return s.reservationRepo.Confirm(txCtx, id, &actor.ID)
})
```

---

## 4. Error Handling & Sentinel Taxonomy

Application services map all low-level database errors into canonical domain sentinels defined in `internal/domain/errors.go`:
- `domain.ErrNotFound`: Resource does not exist or is hidden by visibility rules.
- `domain.ErrUnauthenticated`: Actor is nil or lacks valid session identity.
- `domain.ErrForbidden`: Caller lacks platform role or ReBAC management authority.
- `domain.ErrNotBusinessManager`: User does not manage target business.
- `domain.ErrResourceNotManaged`: IDOR detection (attempt to access category/item belonging to another business).
- `domain.ErrConflict`: Duplicate claim, review, or unique constraint violation.
- `domain.ErrIllegalStateTransition`: Invalid state progression (e.g. reviewing before visit completion, confirming non-pending reservation).
- `domain.ErrInvalidInput`: Field validation failure (invalid rating, empty name, malformed date).

---

## 5. Verification & Test Coverage

The application service layer is verified through automated integration tests in `tests/service_test.go` running against live Neon PostgreSQL:
- `TestDiscoveryService`: Tests public filtering, slug resolution, search, and profile composition.
- `TestCustomerService`: Tests saved places idempotency, follows, claim submission, and venue submissions.
- `TestBusinessAndMenuService`: Tests ReBAC manager authorization, profile updates, opening hours, minor-unit pricing, and category/item IDOR prevention.
- `TestUpdatesService`: Tests announcement publication, editing, listing, and deletion.
- `TestReservationAndReviewServiceWorkflow`: Tests end-to-end dining lifecycle: booking creation → rejection of premature review → confirmation → completion → verified dining review creation → rejection of duplicate review → helpful voting.
- `TestAdminServiceOrchestration`: Tests administrative delegation, RBAC protection, report resolution, and audit log generation.
