# Admin Center: API Migration & Platform Governance (Backend 17)

## 1. Overview
Backend 17 migrates the Cafnet Admin Center (`/admin`) from client-side mock/localStorage persistence to the canonical Go REST API backed by Neon PostgreSQL.

Platform administrators (`role === "ADMIN"`) now have backend-authoritative governance over:
- **Operational Metrics & Stats**: Aggregate counts across businesses, submissions, claims, review moderation, and customer reports in a single query (`GET /api/v1/admin/stats`).
- **Business Submissions**: Ingestion, inspection, approval (atomic business creation + primary manager assignment), and rejection with an audit trail reason.
- **Ownership Claims**: Ingestion, review, approval (manager role assignment + `CLAIMED` status), and rejection with an audit trail reason.
- **Review Moderation**: Inspection of reported reviews, moderation action (`REMOVED_BY_ADMIN` or `VISIBLE`) with automatic dynamic review rating recalculation, and report resolution (`ACTIONED` or `DISMISSED`).
- **Business Information Reports**: Inspection of inaccurate business details (wrong hours, wrong location, invalid phone, closed venue), resolution with admin notes, and dismissal.
- **Verification & Publication Controls**: Toggling official verification badge (`VERIFIED` / `UNVERIFIED`) and public listing visibility (`PUBLISHED` / `UNPUBLISHED`).
- **Immutable Audit Ledger**: Platform-wide audit log recorded on every administrative action (`GET /api/v1/admin/actions`).

---

## 2. Core Administrative Workflows

### A. Business Submissions Workflow
```
Customer submits new venue listing (/business/add)
        ↓
Submission stored in `business_submissions` (status: PENDING)
        ↓
Admin reviews submission (/admin/submissions/[id])
        ├── Reject: Requires non-empty audit reason (status: REJECTED)
        └── Approve: Atomic Transaction (Postgres)
              ├── Generates unique slug from business name
              ├── Inserts new business record (`businesses`)
              ├── Inserts default operating hours (`business_hours`)
              ├── Assigns submitter as PRIMARY_OWNER (`business_managers`)
              └── Updates submission (status: APPROVED, created_business_id)
```
- **Self-Approval Prevention**: An admin cannot approve their own submission (`submitter_user_id == admin_user_id`). The backend rejects this with domain error `domain.ErrSelfApprovalForbidden` (HTTP 403 Forbidden).

### B. Business Ownership Claims Workflow
```
User requests ownership of unclaimed business (/business/claim)
        ↓
Claim stored in `business_claims` (status: PENDING)
        ↓
Admin reviews claim (/admin/claims/[id])
        ├── Reject: Requires non-empty audit reason (status: REJECTED)
        └── Approve: Atomic Transaction (Postgres)
              ├── Verifies business exists and is not already claimed
              ├── Assigns claimant as PRIMARY_OWNER (`business_managers`)
              ├── Sets business claim_status = 'CLAIMED' (`businesses`)
              └── Updates claim (status: APPROVED, reviewed_by, reviewed_at)
```
- **Self-Approval Prevention**: An admin cannot approve their own claim (`claimant_user_id == admin_user_id`). Blocked by ReBAC (HTTP 403 Forbidden).

### C. Review Moderation Workflow
```
Customer flags review (/reviews/[id])
        ↓
Report stored in `review_reports` (status: PENDING)
        ↓
Admin inspects reported review (/admin/reviews/[reviewId])
        ├── Moderate Review: status = REMOVED_BY_ADMIN
        │     └── Automatically excluded from public listings (WHERE status = 'VISIBLE')
        │     └── Instantly recalculates business average rating and review counts
        └── Resolve Report: status = ACTIONED or DISMISSED
              └── Recorded with admin resolution notes
```

### D. Business Information Reports Workflow
```
Customer reports incorrect hours, wrong location, or closed status
        ↓
Report stored in `business_reports` (status: PENDING)
        ↓
Admin inspects report & live business snapshot (/admin/reports/[reportId])
        ├── Dismiss: Report dismissed without changing listing (status: DISMISSED)
        └── Mark Actioned: Confirms listing details corrected (status: ACTIONED, admin_notes)
```

---

## 3. API Endpoints

All admin endpoints require an authenticated session with platform role `ADMIN`. Requests by unauthenticated users return HTTP 401 Unauthorized; requests by authenticated non-admin users return HTTP 403 Forbidden.

| Method | Endpoint | Description |
| :--- | :--- | :--- |
| `GET` | `/api/v1/admin/stats` | Aggregate dashboard statistics (14 counters) |
| `GET` | `/api/v1/admin/submissions` | List business submissions with optional `status`, `limit`, `offset` |
| `GET` | `/api/v1/admin/submissions/{id}` | Retrieve single business submission detail |
| `POST` | `/api/v1/admin/submissions/{id}/approve` | Atomically approve submission, create business, assign manager |
| `POST` | `/api/v1/admin/submissions/{id}/reject` | Reject submission with required reason |
| `GET` | `/api/v1/admin/claims` | List ownership claims with optional `status`, `limit`, `offset` |
| `GET` | `/api/v1/admin/claims/{id}` | Retrieve single ownership claim detail |
| `POST` | `/api/v1/admin/claims/{id}/approve` | Approve claim, assign manager role, mark business claimed |
| `POST` | `/api/v1/admin/claims/{id}/reject` | Reject claim with required reason |
| `GET` | `/api/v1/admin/reviews/reports` | List review reports with optional `status`, `limit`, `offset` |
| `GET` | `/api/v1/admin/reviews/reports/{id}` | Retrieve single review report detail |
| `POST` | `/api/v1/admin/reviews/reports/{id}/resolve` | Resolve or dismiss review report (`ACTIONED` / `DISMISSED`) |
| `GET` | `/api/v1/admin/reviews/{id}` | Retrieve review for admin moderation inspection |
| `POST` | `/api/v1/admin/reviews/{id}/moderate` | Moderate review visibility (`REMOVED_BY_ADMIN` / `VISIBLE`) |
| `GET` | `/api/v1/admin/business-reports` | List business accuracy reports with optional `status`, `limit`, `offset` |
| `GET` | `/api/v1/admin/business-reports/{id}` | Retrieve single business report detail |
| `POST` | `/api/v1/admin/business-reports/{id}/resolve` | Resolve or dismiss business report (`ACTIONED` / `DISMISSED`) |
| `POST` | `/api/v1/admin/businesses/{id}/verification` | Set business verification badge (`VERIFIED` / `UNVERIFIED`) |
| `POST` | `/api/v1/admin/businesses/{id}/publication` | Set business publication visibility (`PUBLISHED` / `UNPUBLISHED`) |
| `GET` | `/api/v1/admin/actions` | List immutable audit actions log with pagination |

---

## 4. Security & ReBAC Platform Role Enforcement

1. **Backend Authoritative**:
   - The Go HTTP router enforces `middleware.RequireRole("ADMIN")` on all `/api/v1/admin/*` routes.
   - Any request lacking an active session returns HTTP 401; any request with a session where `user.Role != "ADMIN"` returns HTTP 403.
2. **Strict Self-Approval Guard**:
   - Submissions: If `submission.SubmitterUserID == adminUserID`, approval is forbidden (`403 Forbidden`).
   - Claims: If `claim.ClaimantUserID == adminUserID`, approval is forbidden (`403 Forbidden`).
3. **Audit Ledger Logging**:
   - Every administrative action creates an immutable record in `admin_actions` with `admin_user_id`, `action_type`, `entity_type`, `entity_id`, and `metadata` JSON.
4. **Dynamic Rating Recalculation**:
   - Moderating a review to `REMOVED_BY_ADMIN` immediately triggers `recalculateReviewSummary` in Postgres, updating aggregate stars and total review counts without client-side calculation.

---

## 5. Frontend Migration Summary

The Next.js frontend has been migrated to communicate directly with the Go REST API:
- `lib/api/admin.ts`: Strongly typed API client module exporting all 20 admin functions.
- `components/admin/AdminLayoutClient.tsx`: Enforces strict session authentication and `role === "ADMIN"` barrier.
- `components/admin/AdminDashboardOverview.tsx`: Connected to real stats, pending claims, pending submissions, and reports.
- `components/admin/AdminSubmissionsPage.tsx` & `AdminSubmissionDetailPage.tsx`: Connected to submissions endpoints.
- `components/admin/AdminClaimsPage.tsx` & `AdminClaimDetailPage.tsx`: Connected to claims endpoints and business details.
- `components/admin/AdminReviewsPage.tsx` & `AdminReviewDetailPage.tsx`: Connected to review reports and moderation.
- `components/admin/AdminReportsPage.tsx` & `AdminReportDetailPage.tsx`: Connected to business reports and resolution.
- `components/admin/AdminBusinessesPage.tsx` & `AdminBusinessDetailPage.tsx`: Connected to business listings, badge toggle, and publication toggle.
- Legacy mock files (`adminStorage.ts`) are fully replaced by the real API.
