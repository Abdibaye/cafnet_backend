# Cafnet Authorization & Business Ownership Architecture
**Phase:** BACKEND 05 — Authorization + Business Ownership  
**Target Stack:** Go 1.27+, Neon PostgreSQL 18.6, Context-Aware ReBAC + RBAC  
**Document ID:** `CAFNET-BACKEND-05-AUTHZ`  
**Status:** Canonical Implementation  

---

## 1. Core Principles: Authentication vs. Authorization

Cafnet enforces strict separation between identity and authority:

| Concept | Question Answered | Authority Mechanism | Scope |
|---|---|---|---|
| **Authentication** | *"Who is this user?"* | Go-native phone OTP & opaque session cookie | User Identity (`domain.User`) |
| **Authorization** | *"What is this user allowed to do on this specific resource?"* | Dual-layer: Platform RBAC + Venue ReBAC (`BusinessManager`) | Operations on Resources |

### Fundamental Invariants:
1. **Authenticated User $\ne$ Business Owner:** An authenticated customer holds zero inherent management privileges over any venue.
2. **Business Manager $\ne$ Owner of Every Business:** Managing Venue A grants zero access, visibility, or mutation rights over Venue B.
3. **No Role-Only Business Access:** Business management is never granted via a global user role (e.g. `user.role == 'OWNER'`). It is strictly evaluated through the dynamic `BusinessManager` relationship table.
4. **No Frontend Trust:** Hidden buttons, disabled inputs, client-side route guards, and local storage state are user experience helpers, not security boundaries. All authorization checks execute server-side.
5. **Fail-Closed Security:** If an identity is missing, unverified, unassociated, or if a database query fails, authorization defaults to immediate denial (`ErrUnauthenticated` or `ErrForbidden`).

---

## 2. Platform Role vs. Business Relationship

Every user interacts with Cafnet through a single canonical `domain.User` record:

```
┌────────────────────────────────────────────────────────┐
│               CANONICAL USER IDENTITY                  │
│                      (User ID)                         │
└───────────────┬────────────────────────┬───────────────┘
                │                        │
                ▼                        ▼
     [PLATFORM RBAC LAYER]    [VENUE ReBAC LAYER]
     (Stored on User.role)    (Evaluated via BusinessManager M:N)
        ├── CUSTOMER             ├── Venue A: Primary Owner (ALLOW)
        └── ADMIN                ├── Venue B: Manager (ALLOW)
                                 └── Venue C: Regular User (DENY)
```

### Context Isolation
- **Customer Context:** Saving, following, requesting reservations, writing eligible reviews for completed personal visits.
- **Business Manager Context:** Editing venue profile, opening hours, gallery images, menu categories/items, broadcast updates, and managing dining reservations for *specifically approved* venues.
- **Platform Admin Context:** Platform governance, approving/rejecting submissions and claims, verifying listing badges, moderating flagged reviews, resolving business reports.

---

## 3. Formal Authorization Predicate & Insecure Direct Object Reference (IDOR) Protection

A user $U$ is authorized to manage business $B$ if and only if:

$$\text{can\_manage\_business}(U, B) \iff \Big(U.\text{role} = \text{'ADMIN'}\Big) \lor \Big(\exists M \in \text{BusinessManagers} : M.\text{user\_id} = U.\text{id} \land M.\text{business\_id} = B.\text{id}\Big)$$

### IDOR Attack Mitigation
If User A manages Venue A, changing request parameters to target Venue B (`/businesses/B`) is strictly blocked:
```go
// AuthorizationService checks dynamic relationship
canManage, err := authzSvc.CanManageBusiness(ctx, userA, bizB.ID)
// canManage == false -> returns domain.ErrNotBusinessManager (HTTP 403 Forbidden)
```
Multi-business management is fully supported: User A can manage Venues A and C simultaneously while remaining completely barred from Venue B.

---

## 4. Explicit Permission Vocabulary

Permissions are strongly typed constants (`domain.Permission`):

```go
// Business Management Permissions (ReBAC via BusinessManager)
PermissionBusinessViewManagement     Permission = "business.view_management"
PermissionBusinessEdit               Permission = "business.edit"
PermissionBusinessManageMenu         Permission = "business.manage_menu"
PermissionBusinessManageUpdates      Permission = "business.manage_updates"
PermissionBusinessManageReservations Permission = "business.manage_reservations"

// Platform Governance Permissions (RBAC via UserRole == ADMIN)
PermissionAdminViewDashboard         Permission = "admin.view_dashboard"
PermissionAdminReviewSubmissions     Permission = "admin.business_submission_review"
PermissionAdminReviewClaims          Permission = "admin.claim_review"
PermissionAdminModerateReviews       Permission = "admin.review_moderation"
PermissionAdminModerateReports       Permission = "admin.business_report_moderation"
PermissionAdminManageBadges          Permission = "admin.badge_and_publication"
```

---

## 5. Resource Ownership Policies

### 5.1 Customer Private Data
Operations on private customer data (bookmarks, follows, account settings) require exact identity matching:
$$\text{can\_access\_user\_data}(U, \text{TargetUser}) \iff U.\text{id} = \text{TargetUser}.\text{id}$$
User 1 cannot view, unsave, or unfollow resources belonging to User 2.

### 5.2 Review Ownership
- **Author:** Can edit or delete their own review (`review.customer_user_id == user.ID`).
- **Business Manager:** Cannot edit or delete customer reviews (must submit a `ReviewReport` for moderation).
- **Admin:** Can take down abusive reviews via official moderation actions (`AdminActionReviewRemoved`).

### 5.3 Reservation Cancellation
A customer may only cancel their own reservation if it is currently in an active state (`PENDING` or `CONFIRMED`). Reservations already in `COMPLETED`, `DECLINED`, or `CANCELLED` states reject cancellation with `domain.ErrIllegalStateTransition`.

---

## 6. Atomic Approval Workflows & Self-Escalation Prevention

Administrative approval workflows mutate multiple tables and grant management rights. To prevent partial state corruption and privilege escalation, these workflows execute inside atomic database transactions (`TxManager`):

### 6.1 Claim Approval Lifecycle (`ApproveClaim`)
```sql
BEGIN;
-- 1. Verify caller is Platform Admin
-- 2. Verify claimant != admin (Self-approval strictly forbidden: ErrSelfApprovalForbidden)
-- 3. Update claim status to APPROVED
UPDATE business_claims SET status = 'APPROVED', reviewed_by = $1, reviewed_at = NOW() WHERE id = $2;
-- 4. Grant BusinessManager relationship
INSERT INTO business_managers (business_id, user_id, manager_role, granted_by) VALUES (...);
-- 5. Mark business as CLAIMED
UPDATE businesses SET claim_status = 'CLAIMED' WHERE id = $3;
-- 6. Record append-only audit trail
INSERT INTO admin_actions (admin_user_id, action_type, target_entity_type, target_entity_id, metadata) VALUES (...);
COMMIT;
```

### 6.2 Submission Approval Lifecycle (`ApproveSubmission`)
1. Only platform admins can approve.
2. Submitter cannot approve their own submission (`ErrSelfApprovalForbidden`).
3. Inside a single transaction:
   - Creates the canonical `Business` record (`publication_status = PUBLISHED`, `claim_status = CLAIMED`).
   - Assigns the submitter as `PRIMARY_OWNER` in `business_managers`.
   - Transitions submission status to `APPROVED` with `created_business_id` linked.
   - Writes immutable audit log into `admin_actions`.

### 6.3 State Guard Enforcement:
- **Pending Claims:** Grant **zero** management access.
- **Rejected Claims:** Grant **zero** management access.
- **Pending Submissions:** Do not create manageable businesses until approved.

---

## 7. Future API Enforcement Model (Backend 06 Contract)

When HTTP endpoints are implemented in Backend 06, request flow enforces authentication, authorization, and domain execution sequentially:

```
[Incoming HTTP Request]
         │
         ▼
[1. Authentication Middleware]
  - Resolves cookie session token
  - Injects authenticated *domain.User into context
         │
         ▼
[2. Authorization Gate (AuthorizationService)]
  - Evaluates Permission + Resource (IDOR check)
  - Denies unauthenticated (401) or unauthorized (403)
         │
         ▼
[3. Domain Application Service]
  - Executes business invariants
         │
         ▼
[4. Repository Layer]
  - Parameterized database interaction
```

### Route-to-Authorization Specification Matrix

| Endpoint | Method | Required Permission | Resource Scope | Ownership Prerequisite |
|---|---|---|---|---|
| `/businesses/:id` | `PATCH` | `business.edit` | `business_id` | Approved `BusinessManager` or Admin |
| `/businesses/:id/hours` | `PUT` | `business.edit` | `business_id` | Approved `BusinessManager` or Admin |
| `/businesses/:id/menu/categories` | `POST` | `business.manage_menu` | `business_id` | Approved `BusinessManager` or Admin |
| `/businesses/:id/menu/categories/:catId` | `PATCH` | `business.manage_menu` | `business_id` | Approved `BusinessManager` or Admin |
| `/businesses/:id/menu/items` | `POST` | `business.manage_menu` | `business_id` | Approved `BusinessManager` or Admin |
| `/businesses/:id/updates` | `POST` | `business.manage_updates` | `business_id` | Approved `BusinessManager` or Admin |
| `/businesses/:id/reservations/:resId/confirm` | `POST` | `business.manage_reservations` | `business_id` | Approved `BusinessManager` or Admin |
| `/businesses/:id/reservations/:resId/decline` | `POST` | `business.manage_reservations` | `business_id` | Approved `BusinessManager` or Admin |
| `/businesses/:id/reservations/:resId/complete`| `POST` | `business.manage_reservations` | `business_id` | Approved `BusinessManager` or Admin |
| `/admin/claims/:id/approve` | `POST` | `admin.claim_review` | `claim_id` | Platform Admin (`claimant != admin`) |
| `/admin/submissions/:id/approve` | `POST` | `admin.business_submission_review` | `submission_id` | Platform Admin (`submitter != admin`) |
| `/admin/reviews/:id/remove` | `POST` | `admin.review_moderation` | `review_id` | Platform Admin |
| `/admin/business-reports/:id/action` | `POST` | `admin.business_report_moderation` | `report_id` | Platform Admin |
| `/reservations/:id/cancel` | `POST` | N/A (Customer Ownership) | `reservation_id` | Customer owner of reservation |
| `/reviews/:id` | `PATCH`/`DELETE` | N/A (Customer Ownership) | `review_id` | Customer author of review |
| `/users/me/saved/:bizId` | `POST`/`DELETE`| N/A (Customer Ownership) | `user_id` | Authenticated session user |
