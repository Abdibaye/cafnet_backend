# Business Updates Management Architecture (Backend 15)

## 1. Overview & Architecture

The Business Updates subsystem provides time-sensitive venue broadcasting across Cafnet. Authenticated and verified business managers can announce promotions, menu additions, events, price updates, and general notices to customers.

```
Next.js Frontend (Business Center / Explore / Followed Feeds)
     ↓ (HTTP / JSON with HttpOnly Cookie)
Go REST API (Backend 15)
     ↓
ReBAC Authorization Layer (`CanPublishUpdate` / `CanManageBusiness`)
     ↓
Neon PostgreSQL (`business_updates` table)
```

## 2. Canonical Update Types

The database and domain model strictly enforce 5 canonical update types:

1. **`new_menu`**: Announcements of new menus, seasonal items, or specialty roasts.
2. **`offer`**: Special promotions or discounts.
   - **Expiration Validation**: Mandatory `valid_until` (format `YYYY-MM-DD` or RFC3339) or `expires_at`. Past dates are rejected with HTTP 400 Bad Request. Automatically derives `expires_at` (end of day UTC). Expired offers are hidden from customer surfaces.
3. **`event`**: Scheduled performances, cupping sessions, or cultural nights.
   - **Date Validation**: Validates `event_date`. Past dates are rejected with HTTP 400. Automatically derives `expires_at` (end of day UTC).
4. **`price_update`**: Price adjustments or revisions (`price_text`).
5. **`announcement`**: General venue updates (e.g., renovations, holiday schedules, new seating).

## 3. ReBAC Authorization & IDOR Defense

Management operations are guarded server-side:

| Operation | Route | Role / Requirement | Protection |
|:---|:---|:---|:---|
| Public Venue Updates | `GET /api/v1/businesses/{id}/updates` | Public / Optional | Returns only active, non-expired, non-deleted updates. |
| Manager Venue Updates | `GET /api/v1/businesses/{id}/updates?include_expired=true` | Manager (ReBAC) | Returns both active and expired updates if actor is authorized manager. Falls back to active-only if unauthorized. |
| Discovery Feed | `GET /api/v1/updates` | Public / Optional | Returns active updates across all published businesses. |
| Following Updates | `GET /api/v1/following/updates` | Authenticated Customer | Returns active updates for businesses followed by current user. |
| Create Update | `POST /api/v1/businesses/{id}/updates` | Manager (ReBAC) | Verifies `CanPublishUpdate(actor, businessID)`. Pending claimants or submitters receive HTTP 403 Forbidden. |
| Update Update | `PATCH /api/v1/businesses/{id}/updates/{updateID}` | Manager (ReBAC) | Verifies `CanPublishUpdate(actor, businessID)` and enforces `update.BusinessID == businessID` (IDOR check). Cross-business modifications rejected with HTTP 403 Forbidden. |
| Delete Update | `DELETE /api/v1/businesses/{id}/updates/{updateID}` | Manager (ReBAC) | Verifies `CanPublishUpdate(actor, businessID)` and enforces `update.BusinessID == businessID`. Soft-deletes record. |

## 4. Frontend Integration

All legacy `localStorage` mock persistence for updates (`cafnet.mock-business-updates`) has been eliminated from the active application lifecycle:

- `frontend/components/UpdateListPage.tsx`: Displays active and expired updates fetched via `listBusinessUpdates(businessId, true)`, with confirmation dialog for soft deletion.
- `frontend/components/CreateUpdatePage.tsx`: Validates bounds (title: 1..120, description: 1..5000), checks date constraints, and publishes via `createBusinessUpdate`.
- `frontend/components/EditUpdatePage.tsx`: Fetches existing update and updates via `updateBusinessUpdate`.
- `frontend/components/sections/ExploreUpdatesFeed.tsx`: Consumes `listActiveUpdates({ limit: 6 })` with authoritative empty states.
- `frontend/components/CustomerCollectionPage.tsx`: Consumes `listFollowingUpdates()` for followed venues.
- `frontend/components/BusinessProfileUpdatesSection.tsx`: Consumes `listBusinessUpdates(business.id)`.
- `frontend/lib/api/adapters.ts`: Provides `apiUpdateToViewModel`, `isUpdateExpired`, `getUpdateTypeLabel`, and `getUpdateTypeIcon`.
