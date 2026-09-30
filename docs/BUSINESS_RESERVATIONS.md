# Business Center: Reservation Management API Migration (Backend 16)

## 1. Overview
Backend 16 completes the canonical migration of the Business Center Reservation Management experience from legacy mock/localStorage persistence (`data/reservationStorage.ts`) to the production Go REST API backed by Neon PostgreSQL.

Managed business owners and authorized managers can now:
- Inspect pending reservation requests and view alerts on the business dashboard.
- Access the dedicated reservation manager at `/business/dashboard/business/[id]/reservations`.
- Inspect detailed guest information (party size, requested date, 12/24-hour time, contact phone, customer name, special requests, and audit timestamps).
- Filter reservations by lifecycle status tab: `Pending`, `Confirmed`, `Completed`, `Declined`, `Cancelled`, and `All`.
- Confirm pending reservations (`POST /api/v1/businesses/{id}/reservations/{resID}/confirm`).
- Decline pending reservations with an explicit decline reason (`POST /api/v1/businesses/{id}/reservations/{resID}/decline`).
- Mark confirmed dining visits as completed (`POST /api/v1/businesses/{id}/reservations/{resID}/complete`), which officially unlocks verified review eligibility for the diner.
- Rely on strict backend-enforced ReBAC (`CanManageReservations`), preventing cross-business access and IDOR vulnerabilities.
- Respect the business capability flag (`reservation_available`), ensuring businesses without table bookings do not display clutter or actionable booking interfaces.

---

## 2. Reservation Lifecycle & State Machine
The reservation lifecycle retains its canonical 5 states across the database, domain layer, and API:

```
                  ┌──────────────┐
                  │   PENDING    │
                  └──────┬───────┘
                         │
        ┌────────────────┼────────────────┐
        │ (confirm)      │ (decline)      │ (cancel by customer)
        ▼                ▼                ▼
┌───────────────┐ ┌──────────────┐ ┌──────────────┐
│   CONFIRMED   │ │   DECLINED   │ │  CANCELLED   │
└───────┬───────┘ └──────────────┘ └──────────────┘
        │ (complete visit)
        ▼
┌───────────────┐
│   COMPLETED   │ ──► Unlocks Verified Review Eligibility
└───────────────┘
```

### Transition Enforcement Rules:
1. `PENDING` -> `CONFIRMED`: Authorized manager confirms table. Sets `confirmed_at = NOW()`.
2. `PENDING` -> `DECLINED`: Authorized manager declines booking. Requires non-empty reason (`decline_reason`), sets `declined_at = NOW()`.
3. `PENDING` -> `CANCELLED`: Customer cancels before arrival. Sets `cancelled_at = NOW()`.
4. `CONFIRMED` -> `COMPLETED`: Authorized manager marks dining party visit finished. Sets `completed_at = NOW()`.
5. Invalid transitions (e.g., `PENDING` -> `COMPLETED`, `DECLINED` -> `COMPLETED`, `CONFIRMED` -> `DECLINED`, or duplicate mutations) return HTTP 409 Conflict.

---

## 3. API Endpoints

| Method | Endpoint | Auth & Permissions | Description |
| :--- | :--- | :--- | :--- |
| `GET` | `/api/v1/businesses/{id}/reservations` | Authenticated Manager (`CanManageReservations`) | List all reservations for the business with optional `status`, `date`, `limit`, `offset` filters. |
| `GET` | `/api/v1/businesses/{id}/reservations/{resID}` | Authenticated Manager (`CanManageReservations`) | Retrieve single reservation details. Verifies reservation belongs to specified business. |
| `POST` | `/api/v1/businesses/{id}/reservations/{resID}/confirm` | Authenticated Manager (`CanManageReservations`) | Transitions status from `PENDING` to `CONFIRMED`. |
| `POST` | `/api/v1/businesses/{id}/reservations/{resID}/decline` | Authenticated Manager (`CanManageReservations`) | Transitions status from `PENDING` to `DECLINED`. Requires JSON body `{"reason": "..."}`. |
| `POST` | `/api/v1/businesses/{id}/reservations/{resID}/complete` | Authenticated Manager (`CanManageReservations`) | Transitions status from `CONFIRMED` to `COMPLETED`. Unlocks customer review eligibility. |

---

## 4. ReBAC Security & IDOR Isolation
All manager endpoints verify three strict criteria server-side:
1. **Authenticated Session**: Session cookie must map to a valid, active user.
2. **Business Management Relationship**: User must be registered as an active manager or owner of `business_id` in `business_managers`.
3. **Resource Ownership**: The target reservation (`resID`) must match `business_id = {id}`. Cross-business manipulations (e.g. Manager of Business B calling actions on Business A's reservations) are rejected with HTTP 403 Forbidden.

---

## 5. Verification & Testing
- **Backend Integration Tests**: `TestHTTP_BusinessReservationManagement` in `backend/tests/http_test.go` exercises unauthenticated access (403), unauthorized manager access (403), cross-business IDOR isolation (403), state machine transitions (200), invalid state machine jumps (409), decline validation (400 on empty reason), and customer visibility of decline reasons.
- **Frontend Unit & Integration Tests**: `reservationsApi handles manager business reservations` in `frontend/tests/api_integration.test.ts` validates list, get, confirm, decline, and complete client methods.
- **Next.js Production Compilation**: Verified via `npm run build` with zero TypeScript errors across all dynamic routes.
