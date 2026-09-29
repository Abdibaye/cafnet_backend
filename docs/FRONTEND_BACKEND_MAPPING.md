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
