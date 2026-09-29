# CAFNET FRONTEND-TO-BACKEND MAPPING SPECIFICATION
**Document ID:** `CAFNET-BACKEND-01-FRONTEND-BACKEND-MAPPING`  
**Status:** Canonical Architecture Specification  
**Target Stack:** Next.js Frontend $\leftrightarrow$ Go Backend (Neon PostgreSQL)  

---

## 1. Feature-to-Backend Domain Mapping Matrix

This table maps every screen, component, and storage module in the feature-complete Next.js frontend to its authoritative backend domain, canonical entity, authorized primary actor, and planned RESTful API contract.

| Frontend UI Screen / Component | Existing Client Storage | Backend Domain | Canonical Entity | Primary Actor | Target Go API Contract |
| :--- | :--- | :--- | :--- | :--- | :--- |
| **Home / Explore / Discovery** (`/explore`, `/cafes`, `/cuisines`, `/drinks`, `/restaurants`) | `mockBusinesses.ts`, `searchBusinesses.ts` | `DISCOVERY` | `Business` | Public / Customer | `GET /api/v1/businesses`<br>`GET /api/v1/categories` |
| **Search Engine** (`/search`, `SearchBar.tsx`) | `searchBusinesses.ts` | `DISCOVERY` | `Business`, `MenuItem` | Public / Customer | `GET /api/v1/search?q={query}` |
| **Business Public Profile** (`/business/[id]`) | `mockBusinesses.ts`, `businessProfileData.ts` | `BUSINESS` | `Business`, `BusinessHours` | Public / Customer | `GET /api/v1/businesses/{id}` |
| **Save Business** (`CustomerProvider.tsx`) | `customerStorage.ts` (`savedBusinessIds`) | `DISCOVERY` | `SavedBusiness` | Authenticated Customer | `POST /api/v1/businesses/{id}/save`<br>`DELETE /api/v1/businesses/{id}/save` |
| **Follow Business** (`CustomerProvider.tsx`) | `customerStorage.ts` (`followedBusinessIds`) | `DISCOVERY` | `FollowedBusiness` | Authenticated Customer | `POST /api/v1/businesses/{id}/follow`<br>`DELETE /api/v1/businesses/{id}/follow` |
| **Customer Account** (`/account`) | `customerStorage.ts` | `IDENTITY` | `User` | Authenticated Customer | `GET /api/v1/users/me`<br>`PATCH /api/v1/users/me` |
| **Request Reservation** (`/reserve/[id]`, `ReservationFlow.tsx`) | `reservationStorage.ts` | `RESERVATIONS` | `Reservation` | Authenticated Customer | `POST /api/v1/businesses/{id}/reservations` |
| **Track Reservations** (`/reservations`, `/reservations/[id]`) | `reservationStorage.ts` | `RESERVATIONS` | `Reservation` | Authenticated Customer | `GET /api/v1/reservations/me`<br>`GET /api/v1/reservations/{id}` |
| **Cancel Reservation** (`ReservationDetailPage.tsx`) | `reservationStorage.ts` (`cancelReservation`) | `RESERVATIONS` | `Reservation` | Authenticated Customer | `POST /api/v1/reservations/{id}/cancel` |
| **Write Verified Review** (`ReviewSubmission.tsx`) | `customerReviewStorage.ts` | `REVIEWS` | `Review` | Customer with completed visit | `POST /api/v1/reservations/{resId}/review` |
| **Edit Own Review** (`CustomerReviewsPage.tsx`) | `customerReviewStorage.ts` (`updateReview`) | `REVIEWS` | `Review` | Customer (Author) | `PATCH /api/v1/reviews/{id}` |
| **Delete Own Review** (`CustomerReviewsPage.tsx`) | `customerReviewStorage.ts` (`deleteReview`) | `REVIEWS` | `Review` | Customer (Author) | `DELETE /api/v1/reviews/{id}` |
| **Vote Review Helpful** (`reviewHelpfulStorage.ts`) | `reviewHelpfulStorage.ts` | `REVIEWS` | `ReviewHelpfulVote` | Authenticated Customer | `POST /api/v1/reviews/{id}/helpful` |
| **Report Problematic Review** (`reviewReportStorage.ts`) | `reviewReportStorage.ts` | `MODERATION` | `ReviewReport` | Authenticated Customer | `POST /api/v1/reviews/{id}/report` |
| **Report Business Info** (`ReportBusinessModal.tsx`) | `businessReportStorage.ts` | `MODERATION` | `BusinessReport` | Authenticated Customer | `POST /api/v1/businesses/{id}/report` |
| **Claim Existing Business** (`/business/claim`, `BusinessClaimFlow.tsx`)| `businessOwnerStorage.ts` (`claims`) | `BUSINESS OWNERSHIP` | `BusinessClaim` | Business Owner | `POST /api/v1/businesses/{id}/claims` |
| **Add New Business** (`/business/add`, `BusinessAddFlow.tsx`) | `businessOwnerStorage.ts` (`submissions`) | `BUSINESS OWNERSHIP` | `BusinessSubmission` | Business Owner | `POST /api/v1/submissions` |
| **Business Center** (`/business/dashboard`, `BusinessCenterPage.tsx`)| `ownerManagedBusinessStorage.ts` | `BUSINESS OWNERSHIP` | `BusinessManager` | Business Manager | `GET /api/v1/business-center/managed` |
| **Listing Profile Management** (`EditBusinessPage.tsx`) | `ownerManagedBusinessStorage.ts` | `BUSINESS` | `Business`, `BusinessHours` | Authorized Manager | `PATCH /api/v1/managed-businesses/{id}` |
| **Menu Category CRUD & Reorder** (`ManageMenuPage.tsx`) | `menuStorage.ts` | `MENU` | `MenuCategory` | Authorized Manager | `POST /api/v1/managed-businesses/{id}/menu/categories`<br>`PATCH /api/v1/menu/categories/{catId}`<br>`DELETE /api/v1/menu/categories/{catId}`<br>`POST /api/v1/menu/categories/{catId}/reorder` |
| **Menu Item CRUD & Reorder** (`ManageMenuPage.tsx`) | `menuStorage.ts` | `MENU` | `MenuItem` | Authorized Manager | `POST /api/v1/menu/categories/{catId}/items`<br>`PATCH /api/v1/menu/items/{itemId}`<br>`DELETE /api/v1/menu/items/{itemId}`<br>`POST /api/v1/menu/items/{itemId}/reorder` |
| **Create / Edit / Delete Updates** (`CreateUpdatePage.tsx`, `EditUpdatePage.tsx`) | `businessUpdateStorage.ts` | `BUSINESS UPDATES` | `BusinessUpdate` | Authorized Manager | `POST /api/v1/managed-businesses/{id}/updates`<br>`PATCH /api/v1/updates/{updateId}`<br>`DELETE /api/v1/updates/{updateId}` |
| **Confirm Reservation** (`BusinessReservationsPage.tsx`) | `reservationStorage.ts` (`confirmReservation`) | `RESERVATIONS` | `Reservation` | Authorized Manager | `POST /api/v1/managed-businesses/{id}/reservations/{resId}/confirm` |
| **Decline Reservation** (`BusinessReservationsPage.tsx`) | `reservationStorage.ts` (`declineReservation`) | `RESERVATIONS` | `Reservation` | Authorized Manager | `POST /api/v1/managed-businesses/{id}/reservations/{resId}/decline` |
| **Mark Visit Completed** (`BusinessReservationsPage.tsx`) | `reservationStorage.ts` (`completeReservation`) | `RESERVATIONS` | `Reservation` | Authorized Manager | `POST /api/v1/managed-businesses/{id}/reservations/{resId}/complete` |
| **Admin Dashboard Overview** (`/admin`, `AdminDashboardOverview.tsx`) | `adminStorage.ts` (`getAdminStats`) | `ADMINISTRATION` | Platform Analytics | Platform Admin | `GET /api/v1/admin/stats` |
| **Admin Submissions Review** (`/admin/submissions`) | `adminStorage.ts` (`approveSubmission`, `rejectSubmission`) | `ADMINISTRATION` | `BusinessSubmission` | Platform Admin | `POST /api/v1/admin/submissions/{id}/approve`<br>`POST /api/v1/admin/submissions/{id}/reject` |
| **Admin Claims Review** (`/admin/claims`) | `adminStorage.ts` (`approveClaim`, `rejectClaim`) | `ADMINISTRATION` | `BusinessClaim` | Platform Admin | `POST /api/v1/admin/claims/{id}/approve`<br>`POST /api/v1/admin/claims/{id}/reject` |
| **Admin Review Moderation** (`/admin/reviews`) | `reviewReportStorage.ts` (`removeReviewByAdmin`, `dismissReport`) | `MODERATION` | `ReviewReport`, `Review` | Platform Admin | `POST /api/v1/admin/review-reports/{id}/dismiss`<br>`POST /api/v1/admin/reviews/{id}/remove` |
| **Admin Business Reports** (`/admin/reports`) | `businessReportStorage.ts` (`dismissBusinessReport`, `actionBusinessReport`) | `MODERATION` | `BusinessReport` | Platform Admin | `POST /api/v1/admin/business-reports/{id}/dismiss`<br>`POST /api/v1/admin/business-reports/{id}/action` |
| **Admin Business Directory & Badging** (`/admin/businesses`) | `adminStorage.ts` (`setBusinessVerification`) | `ADMINISTRATION` | `Business` | Platform Admin | `POST /api/v1/admin/businesses/{id}/verify`<br>`POST /api/v1/admin/businesses/{id}/publication` |

---

## 2. Feature Coverage Verification Checklist

The proposed domain architecture fully supports 100% of all frontend features without gaps:

- [x] **Customer Discovery:** Handled via `Business` entity and `DISCOVERY` read queries.
- [x] **Search:** Handled via PostgreSQL Full-Text Search over business names, descriptions, tags, and menu items.
- [x] **Save Businesses:** Handled via `SavedBusiness` M:N join table.
- [x] **Follow Businesses:** Handled via `FollowedBusiness` M:N join table.
- [x] **Business Profiles:** Handled via canonical `Business`, `BusinessHours`, `BusinessImage`.
- [x] **Add Business Flow:** Handled via `BusinessSubmission` with approval/rejection audit trail.
- [x] **Claim Business Flow:** Handled via `BusinessClaim` with approval/rejection audit trail.
- [x] **Business Approval:** Handled via transactional submission approval creating `Business` + `BusinessManager`.
- [x] **Business Management:** Handled via `BusinessManager` relationship authorizing edits.
- [x] **Menu Management:** Handled via `MenuCategory` and `MenuItem` with strict `sort_order` and ETB minor unit prices.
- [x] **Business Updates:** Handled via `BusinessUpdate` with typed categorization, scheduling, and expiration logic.
- [x] **Reservations:** Handled via `Reservation` entity with 5-state lifecycle (`PENDING`, `CONFIRMED`, `DECLINED`, `CANCELLED`, `COMPLETED`).
- [x] **Reservation Approval:** Handled via owner confirmation action on `Reservation`.
- [x] **Visit Completion:** Handled via owner marking confirmed reservation `COMPLETED`.
- [x] **Verified Reviews:** Handled via `Review` requiring `Reservation.status = 'COMPLETED'`.
- [x] **Review Editing:** Handled via author patch on `Review`.
- [x] **Review Deletion:** Handled via author soft-delete on `Review`.
- [x] **Helpful Votes:** Handled via `ReviewHelpfulVote` unique toggle.
- [x] **Review Reports:** Handled via `ReviewReport` community flagging triage.
- [x] **Business Reports:** Handled via `BusinessReport` community inaccuracy reporting.
- [x] **Admin Moderation:** Handled via `AdminAction` and report resolution endpoints.

---

## 3. Authentication & Session Integration Boundary

The frontend currently uses mock OTP and client-side simulation. The future backend integration boundary replaces client-side mock identity with server-side cookie sessions:

| Frontend Auth Action | Current Mock Mechanism | Future Go Backend Route | Payload / Headers |
|---|---|---|---|
| **Enter Phone Number** | `CustomerProvider.tsx` (`requestOtp`) | `POST /auth/otp/request` | `{ "phone": string }` $\to$ `{ "challenge_id": UUID, "expires_at": ISO8601, "cooldown_seconds": 60 }` |
| **Verify OTP Code** | Compares with hardcoded `'123456'` | `POST /auth/otp/verify` | `{ "challenge_id": UUID, "phone": string, "otp_code": string }` $\to$ Sets `cafnet_session` HTTP-only cookie |
| **Check Active Identity** | Reads mock user from `localStorage` | `GET /auth/me` | Cookie `cafnet_session` $\to$ Returns canonical `*domain.User` |
| **Sign Out** | Clears mock user state in memory | `POST /auth/logout` | Revokes server session record; expires `cafnet_session` cookie |

---

## 4. Authorization & Ownership Enforcement Boundary

The frontend currently uses client-side role simulation. The authoritative backend boundary enforces server-side ReBAC and RBAC via `AuthorizationService`:

| Frontend Operational Feature | Client Security Assumption | Authoritative Go Backend Enforcement | Enforcement Predicate |
|---|---|---|---|
| **Edit Listing Profile** | User clicked "Edit" in dashboard | `AuthorizationService.CanEditBusiness` | `can_manage_business(U, B)` (ReBAC via `business_managers`) |
| **Manage Menu Items & Categories** | UI displays menu management page | `AuthorizationService.CanManageMenu` | `can_manage_business(U, B)` |
| **Broadcast Updates (New menu/event)**| UI displays create update form | `AuthorizationService.CanPublishUpdate` | `can_manage_business(U, B)` |
| **Confirm / Decline Reservation** | UI displays reservation action buttons | `AuthorizationService.CanManageReservations` | `can_manage_business(U, B)` |
| **Approve Business Claim** | Admin review screen button | `OwnershipWorkflowService.ApproveClaim` | `U.role == 'ADMIN' && claimant_id != U.id` (Atomic transaction) |
| **Approve Business Submission** | Admin review screen button | `OwnershipWorkflowService.ApproveSubmission` | `U.role == 'ADMIN' && submitter_id != U.id` (Atomic transaction) |
| **Delete / Edit Customer Review** | UI shows author controls | `AuthorizationService.AuthorizeReviewModification` | `review.customer_user_id == U.id` |
| **Cancel Dining Reservation** | UI displays "Cancel" button | `AuthorizationService.AuthorizeReservationCancellation` | `reservation.customer_user_id == U.id && status IN ('PENDING', 'CONFIRMED')` |

---

## 5. Application Service Orchestration Mapping (Backend 06)

The table below maps each frontend user journey directly to the Backend 06 Application Services that orchestrate the use case between future HTTP handlers and the PostgreSQL domain layer:

| Frontend UI Component / Flow | User Intent / Action | Backend 06 Application Service | Service Method & Signature | Invariant & Security Enforcement |
|:---|:---|:---|:---|:---|
| **Venue Details Page** (`/business/[id]`) | Load comprehensive public profile | `DiscoveryService` | `GetPublicBusinessProfile(ctx, id)` | Filters `PUBLISHED` only; aggregates hours, images, menu, active updates, reviews, and summary |
| **Search Bar & Explore** | Discover venues by name, cuisine, tag | `DiscoveryService` | `SearchBusinesses(ctx, query, limit, offset)` | Full-text search via PostgreSQL GIN indexes, enforces public visibility |
| **Save / Bookmark Button** | Save or bookmark favorite venue | `CustomerService` | `SaveBusiness(ctx, actor, businessID)` | Idempotent duplicate save, customer ownership check |
| **Follow Venue Button** | Follow venue for notifications/feed | `CustomerService` | `FollowBusiness(ctx, actor, businessID)` | Idempotent duplicate follow, customer ownership check |
| **Claim Venue Modal** | Business owner claims listing | `CustomerService` | `SubmitClaim(ctx, actor, businessID)` | Verifies business exists, rejects already claimed or pending claim conflicts |
| **Add Venue Submission Flow** | Suggest a new cafe or restaurant | `CustomerService` | `CreateSubmission(ctx, actor, input)` | Records pending submission; does not create public listing until admin approval |
| **Venue Management Dashboard** | Edit profile info, phone, website | `BusinessService` | `UpdateBusiness(ctx, actor, businessID, input)` | ReBAC authorization check (`CanEditBusiness`), protects platform badge fields |
| **Manage Opening Hours** | Update daily opening/closing times | `BusinessService` | `SetHours(ctx, actor, businessID, hours)` | ReBAC check, validates day of week (0-6) and time format |
| **Manage Menu Screen** | Create/edit categories & items | `MenuService` | `CreateCategory`, `CreateItem`, `ReorderItems` | ReBAC check (`CanManageMenu`), IDOR prevention across business categories, ETB minor-unit pricing |
| **Broadcast Updates Screen** | Post announcement, event, or offer | `UpdatesService` | `CreateUpdate(ctx, actor, businessID, input)` | ReBAC check (`CanPublishUpdate`), validates update type, expiry bounds |
| **Customer Reserve Flow** | Book a table at a cafe/restaurant | `ReservationService` | `CreateReservation(ctx, actor, input)` | Generates confirmation code, party size bounds (1-50), status `PENDING` |
| **Customer Cancel Reservation** | Cancel upcoming reservation | `ReservationService` | `CancelReservation(ctx, actor, id)` | Customer ownership check, validates state is `PENDING` or `CONFIRMED` |
| **Manager Reservations Tab** | Confirm pending reservation | `ReservationService` | `ConfirmReservation(ctx, actor, businessID, id)` | ReBAC check (`CanManageReservations`), row lock, state transition `PENDING` $\to$ `CONFIRMED` |
| **Manager Reservations Tab** | Complete dining visit | `ReservationService` | `CompleteReservation(ctx, actor, businessID, id)` | ReBAC check, state transition `CONFIRMED` $\to$ `COMPLETED`; unlocks verified dining review |
| **Submit Verified Review** | Review after dining at venue | `ReviewService` | `CreateReview(ctx, actor, input)` | Authoritative verification: requires `COMPLETED` reservation; duplicate review prevention |
| **Helpful Vote on Review** | Upvote helpful customer review | `ReviewService` | `MarkHelpful(ctx, actor, reviewID)` | Idempotent upvote, customer identity required |
| **Report Review Modal** | Flag abusive or fake review | `ReportService` | `CreateReviewReport(ctx, actor, input)` | Records review report with status `PENDING` |
| **Report Business Modal** | Flag incorrect hours or location | `ReportService` | `CreateBusinessReport(ctx, actor, input)` | Records business report with status `PENDING` |
| **Admin Submissions Review** | Approve venue suggestion | `AdminService` | `ApproveSubmission(ctx, adminUser, id)` | RBAC check, delegates to `OwnershipWorkflowService`, creates `Business` + `BusinessManager`, logs audit action |
| **Admin Claims Review** | Approve business owner claim | `AdminService` | `ApproveClaim(ctx, adminUser, id, notes)` | RBAC check, delegates to `OwnershipWorkflowService`, grants manager role, updates claim status, logs audit action |
| **Admin Review Moderation** | Take down inappropriate review | `AdminService` | `ModerateReview(ctx, adminUser, id, status, reason)` | RBAC check, updates review status, logs immutable `AdminAction` audit trail |
| **Admin Reports Resolution** | Resolve/dismiss community reports | `AdminService` | `ResolveReviewReport`, `ResolveBusinessReport` | RBAC check, updates report status, logs immutable `AdminAction` audit trail |



