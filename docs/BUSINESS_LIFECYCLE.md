# Cafnet — Real Business Lifecycle & Discovery Architecture

This document specifies the end-to-end business lifecycle in Cafnet, establishing the single canonical business model spanning user submission, administrative review, owner management via Business Center, and public discovery.

---

## 1. Lifecycle Architecture

The unified lifecycle is defined as:

```
  Authenticated User
         ↓
  Submit Venue Recommendation (/business/add)
         ↓
  Pending Business Submission (`business_submissions`)
         ↓
  Admin Review Queue (/admin/submissions)
    ↙                     ↘
Rejected                Approved
(Status: REJECTED,        ↓
 Reason recorded)     Atomic PostgreSQL Transaction:
                       1. Create Canonical Business (`businesses`)
                       2. Assign Submitter as PRIMARY_OWNER (`business_managers`)
                       3. Mark Submission APPROVED (`created_business_id`)
                       4. Record Immutable Audit Action (`admin_actions`)
                          ↓
                      Business Manager Access
                      (Business Center: /business/dashboard)
                          ↓
                      Manage Profile, Schedule, Menus, Updates
                          ↓
                      Customer Discovery
                      (Homepage, Explore, Cafes, Restaurants, Drinks, Search, Profile)
                          ↓
                      One Canonical PostgreSQL Business Entity
```

---

## 2. Canonical Business Invariant

1. **One Source of Truth**: There is exactly one canonical business entity stored in PostgreSQL (`businesses`). Mock business datasets (`mockBusinesses.ts`, `businessProfileData.ts`) are completely bypassed in production/customer discovery.
2. **Zero Silent Mock Fallbacks**:
   - Customer discovery (Homepage, Explore, Cafés, Restaurants, Drinks, Search, Profile) queries the Go backend API.
   - If no businesses exist in the database, the frontend renders authentic empty states ("No businesses found yet"), never hardcoded mock places.
3. **No Phantom Owners**: Only authenticated users with explicit manager records in `business_managers` have mutation access to a venue.

---

## 3. Submission Flow

- **Endpoint**: `POST /api/v1/businesses/submissions` (Alias: `POST /api/v1/submissions`)
- **Authentication**: Required (`authRequired`). Unauthenticated users attempting to access `/business/add` are prompted to sign in or create an account.
- **Validation**:
  - `business_name`: 2 to 150 characters, trimmed.
  - `area`: 2 to 100 characters, trimmed.
  - `category`: Must match one of `restaurant`, `cafe`, or `drinks`.
- **Duplicate Protection**:
  - Enforced at both application and database levels.
  - Partial Unique Index:
    ```sql
    CREATE UNIQUE INDEX idx_unique_pending_business_submission
    ON business_submissions(LOWER(TRIM(business_name)), LOWER(TRIM(area)))
    WHERE status = 'PENDING';
    ```
  - Submitting an identical venue while another is pending returns `409 Conflict` (`ErrConflict`).

---

## 4. Administrative Review & Promotion

- **Endpoints**:
  - `GET /api/v1/admin/submissions`: Lists pending/reviewed submissions.
  - `POST /api/v1/admin/submissions/{id}/approve`: Approves submission.
  - `POST /api/v1/admin/submissions/{id}/reject`: Rejects submission with required reason.
- **Transactional Promotion**:
  Approval is executed within an atomic database transaction (`OwnershipWorkflowService.ApproveSubmission`):
  1. Creates canonical `domain.Business` record in `businesses`.
  2. Sets `publication_status = 'PUBLISHED'`, `verification_status = 'VERIFIED'`, `claim_status = 'CLAIMED'`.
  3. Inserts submitter as `PRIMARY_OWNER` in `business_managers`.
  4. Updates submission status to `APPROVED` with `created_business_id` set.
  5. Records immutable audit record in `admin_actions`.
- **Self-Approval Protection**:
  If an administrator attempts to approve their own submission (`sub.SubmitterUserID == adminUser.ID`), the system returns `403 Forbidden` (`ErrSelfApprovalForbidden`).

---

## 5. Business Center Access & ReBAC

- **Endpoints**:
  - `GET /api/v1/business-center/managed` (Alias: `GET /api/v1/businesses/managed`)
  - `GET /api/v1/businesses/{id}/manage`
  - `PATCH /api/v1/businesses/{id}`
  - `GET /api/v1/businesses/{id}/hours` & `PUT /api/v1/businesses/{id}/hours`
  - `GET /api/v1/businesses/{id}/menu` & category/item mutations
  - `POST /api/v1/businesses/{id}/updates` & updates mutations
  - `GET /api/v1/businesses/{id}/reservations` & status changes
- **ReBAC Boundary**:
  All venue mutations verify `OwnershipRepository.IsManager(ctx, actor.ID, businessID)`. Unauthorized users receive `403 Forbidden` (`ErrForbidden`).

---

## 6. Public Customer Discovery

- **Public Endpoints**:
  - `GET /api/v1/businesses`: Lists published venues with category, neighborhood, pagination filters.
  - `GET /api/v1/businesses/{id}`: Aggregated public profile (business metadata, hours, gallery, menu, active updates, review summary, recent reviews).
  - `GET /api/v1/slugs/{slug}`: Slug lookup for canonical URL routing.
- **Empty States**:
  When discovery endpoints return empty arrays, UI displays clean, responsive empty-state cards guiding users to explore other categories or submit places they love.
