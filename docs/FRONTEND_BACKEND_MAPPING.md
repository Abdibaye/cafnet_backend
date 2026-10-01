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
| **Customer Account & Onboarding** (`/account`, `/account/settings`, `/onboarding`) | Real Session Auth (`CustomerProvider.tsx`) | `IDENTITY` | `User` | Authenticated Customer | `POST /api/v1/auth/signup`<br>`POST /api/v1/auth/signin`<br>`POST /api/v1/auth/logout`<br>`GET /api/v1/auth/me`<br>`GET /api/v1/account/profile`<br>`PATCH /api/v1/account/profile` |
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
| **Admin Submissions Review** (`/admin/submissions`, `/admin/submissions/[id]`) | `adminStorage.ts` (`approveSubmission`, `rejectSubmission`) | `ADMINISTRATION` | `BusinessSubmission` | Platform Admin | `GET /api/v1/admin/submissions`<br>`GET /api/v1/admin/submissions/{id}`<br>`POST /api/v1/admin/submissions/{id}/approve`<br>`POST /api/v1/admin/submissions/{id}/reject` |
| **Admin Claims Review** (`/admin/claims`, `/admin/claims/[id]`) | `adminStorage.ts` (`approveClaim`, `rejectClaim`) | `ADMINISTRATION` | `BusinessClaim` | Platform Admin | `GET /api/v1/admin/claims`<br>`GET /api/v1/admin/claims/{id}`<br>`POST /api/v1/admin/claims/{id}/approve`<br>`POST /api/v1/admin/claims/{id}/reject` |
| **Admin Review Moderation** (`/admin/reviews`, `/admin/reviews/[reviewId]`) | `reviewReportStorage.ts` (`removeReviewByAdmin`, `dismissReport`) | `MODERATION` | `ReviewReport`, `Review` | Platform Admin | `GET /api/v1/admin/reviews/reports`<br>`GET /api/v1/admin/reviews/reports/{id}`<br>`POST /api/v1/admin/reviews/reports/{id}/resolve`<br>`GET /api/v1/admin/reviews/{id}`<br>`POST /api/v1/admin/reviews/{id}/moderate` |
| **Admin Business Reports** (`/admin/reports`, `/admin/reports/[reportId]`) | `businessReportStorage.ts` (`dismissBusinessReport`, `actionBusinessReport`) | `MODERATION` | `BusinessReport` | Platform Admin | `GET /api/v1/admin/business-reports`<br>`GET /api/v1/admin/business-reports/{id}`<br>`POST /api/v1/admin/business-reports/{id}/resolve` |
| **Admin Business Directory & Badging** (`/admin/businesses`, `/admin/businesses/[id]`) | `adminStorage.ts` (`setBusinessVerification`) | `ADMINISTRATION` | `Business` | Platform Admin | `POST /api/v1/admin/businesses/{id}/verification`<br>`POST /api/v1/admin/businesses/{id}/publication` |
| **Admin Audit Trail** | Database audit table | `ADMINISTRATION` | `AdminAction` | Platform Admin | `GET /api/v1/admin/actions` |

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

The frontend uses server-side cookie sessions (`cafnet_session`) backed by PostgreSQL, completely replacing client-side mock identity:

| Frontend Auth Action | Frontend Mechanism | Go Backend Route | Payload / Headers |
|---|---|---|---|
| **Enter Phone Number** | `CustomerProvider.tsx` (`requestOtp`) | `POST /api/v1/auth/otp/request` | `{ "phone": string }` $\to$ `{ "challenge_id": UUID, "expires_at": ISO8601, "cooldown_seconds": 60 }` |
| **Verify OTP Code** | `CustomerProvider.tsx` (`verifyOtp`) | `POST /api/v1/auth/otp/verify` | `{ "challenge_id": UUID, "phone": string, "otp_code": string }` $\to$ Sets `cafnet_session` cookie; returns `user`, `is_new_user`, `profile_completed` |
| **Complete Onboarding / Update Profile** | `CustomerProvider.tsx` (`updateProfile`) | `PATCH /api/v1/account/profile` | Cookie `cafnet_session`, `{ "full_name": string, "avatar_url"?: string }` $\to$ Returns updated `*domain.User` |
| **Check Active Identity** | `CustomerProvider.tsx` (`refresh`) | `GET /api/v1/auth/me` | Cookie `cafnet_session` $\to$ Returns canonical `*domain.User` |
| **Sign Out** | `CustomerProvider.tsx` (`logout`) | `POST /api/v1/auth/logout` | Revokes server session record; expires `cafnet_session` cookie |

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

---

## 6. HTTP Transport Endpoint Mapping (Backend 07)

The table below maps each frontend route / screen to the Backend 07 HTTP REST API endpoints:

| Frontend Route / Screen | HTTP Method | Endpoint Path | Auth Policy | Description |
|:---|:---|:---|:---|:---|
| `/explore`, `/cafes`, `/restaurants`, `/drinks` | `GET` | `/api/v1/businesses` | Public / Optional | List published venues with category, neighborhood, limit, and offset filters |
| `/business/[id]` | `GET` | `/api/v1/businesses/{id}` | Public / Optional | Aggregated public business profile (hours, menu, updates, verified reviews) by ID or slug |
| `/business/[id]` | `GET` | `/api/v1/slugs/{slug}` | Public / Optional | Public business profile lookup by unique URL slug |
| `/search` | `GET` | `/api/v1/search` | Public / Optional | Full-text directory search with tsvector ranking |
| `/login`, `/account` | `POST` | `/api/v1/auth/otp/request` | Public | Request SMS verification code for Ethiopian phone number |
| `/login`, `/account` | `POST` | `/api/v1/auth/otp/verify` | Public | Verify OTP code and receive `HttpOnly` `cafnet_session` cookie |
| `/account` | `GET` | `/api/v1/auth/me` | Required | Retrieve current authenticated user profile and platform role |
| `/account` | `POST` | `/api/v1/auth/logout` | Public/Auth | Invalidate session in DB and clear session cookie |
| `/saved` | `GET` | `/api/v1/saved` | Required | List user's bookmarked favorite places |
| `/business/[id]` (Save) | `POST` | `/api/v1/saved/{businessID}` | Required | Bookmark venue |
| `/business/[id]` (Unsave) | `DELETE` | `/api/v1/saved/{businessID}` | Required | Remove venue bookmark |
| `/following` | `GET` | `/api/v1/following` | Required | List user's followed venues |
| `/business/[id]` (Follow) | `POST` | `/api/v1/following/{businessID}` | Required | Follow venue for broadcast updates |
| `/business/[id]` (Unfollow) | `DELETE` | `/api/v1/following/{businessID}` | Required | Unfollow venue |
| `/business/claim` | `POST` | `/api/v1/claims` | Required | Submit venue ownership claim |
| `/business/dashboard/claim/[id]` | `GET` | `/api/v1/claims/{id}` | Required | View status of submitted claim |
| `/business/add` | `POST` | `/api/v1/submissions` | Required | Submit venue recommendation |
| `/business/dashboard/submission/[id]` | `GET` | `/api/v1/submissions/{id}` | Required | View status of submitted venue suggestion |
| `/business/dashboard` | `GET` | `/api/v1/businesses/managed` | Required | List venues managed by current user |
| `/business/dashboard/business/[id]/edit` | `PATCH` | `/api/v1/businesses/{id}` | Manager (ReBAC) | Update venue profile info |
| `/business/dashboard/business/[id]/edit` | `PUT` | `/api/v1/businesses/{id}/hours` | Manager (ReBAC) | Update daily operating hours |
| `/business/dashboard/business/[id]/edit` | `PUT` | `/api/v1/businesses/{id}/images` | Manager (ReBAC) | Update venue photo gallery |
| `/business/dashboard/business/[id]/menu` | `GET` | `/api/v1/businesses/{id}/menu` | Public / Optional | Retrieve full menu hierarchy |
| `/business/dashboard/business/[id]/menu` | `POST` | `/api/v1/businesses/{id}/menu/categories` | Manager (ReBAC) | Create menu category |
| `/business/dashboard/business/[id]/menu` | `POST` | `/api/v1/businesses/{id}/menu/items` | Manager (ReBAC) | Create menu item with ETB minor units |
| `/business/dashboard/business/[id]/updates` | `GET` | `/api/v1/businesses/{id}/updates` | Public / Optional | List venue announcements and offers |
| `/business/dashboard/business/[id]/updates/create` | `POST` | `/api/v1/businesses/{id}/updates` | Manager (ReBAC) | Publish broadcast update post |
| `/reserve/[id]` | `POST` | `/api/v1/reservations` | Required | Book a table reservation |
| `/reservations` | `GET` | `/api/v1/reservations` | Required | List customer's reservations |
| `/reservations/[id]` | `POST` | `/api/v1/reservations/{id}/cancel` | Required | Cancel customer reservation |
| `/business/dashboard/business/[id]/reservations` | `GET` | `/api/v1/businesses/{id}/reservations` | Manager (ReBAC) | List venue reservations |
| `/business/dashboard/business/[id]/reservations` | `POST` | `/api/v1/businesses/{id}/reservations/{resID}/confirm` | Manager (ReBAC) | Confirm table reservation |
| `/business/dashboard/business/[id]/reservations` | `POST` | `/api/v1/businesses/{id}/reservations/{resID}/decline` | Manager (ReBAC) | Decline table reservation with reason |
| `/business/dashboard/business/[id]/reservations` | `POST` | `/api/v1/businesses/{id}/reservations/{resID}/complete` | Manager (ReBAC) | Mark visit completed (unlocks review) |
| `/business/[id]` (Review) | `POST` | `/api/v1/reviews` | Required | Create verified review (completed visit required) |
| `/reviews` | `GET` | `/api/v1/reviews/mine` | Required | List user's reviews |
| `/business/[id]` (Helpful) | `POST` | `/api/v1/reviews/{id}/helpful` | Required | Upvote review helpfulness |
| `/business/[id]` (Report Review) | `POST` | `/api/v1/reviews/{id}/reports` | Required | Flag review for moderation |
| `/business/[id]` (Report Business) | `POST` | `/api/v1/businesses/{id}/reports` | Required | Flag inaccurate venue listing |
| `/admin/submissions` | `GET` | `/api/v1/admin/submissions` | Admin (RBAC) | List pending venue submissions |
| `/admin/submissions/[id]` | `POST` | `/api/v1/admin/submissions/{id}/approve` | Admin (RBAC) | Approve submission into business |
| `/admin/claims` | `GET` | `/api/v1/admin/claims` | Admin (RBAC) | List pending ownership claims |
| `/admin/claims/[id]` | `POST` | `/api/v1/admin/claims/{id}/approve` | Admin (RBAC) | Approve claim and assign manager |
| `/admin/reviews` | `GET` | `/api/v1/admin/reviews/reports` | Admin (RBAC) | List reported reviews |
| `/admin/reviews/[reviewId]` | `POST` | `/api/v1/admin/reviews/{id}/moderate` | Admin (RBAC) | Moderate review (approved, flagged, rejected) |
| `/admin/reports` | `GET` | `/api/v1/admin/business-reports` | Admin (RBAC) | List business reports |
| `/admin` (Audit) | `GET` | `/api/v1/admin/actions` | Admin (RBAC) | Query immutable audit log actions |



