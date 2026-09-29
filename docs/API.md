# Cafnet HTTP / API Specification

## 1. Overview & Architectural Boundaries

The Cafnet HTTP layer acts strictly as a **transport adapter** between external HTTP clients (Next.js web frontend, future mobile clients) and the underlying domain architecture.

### Architectural Invariants:
1. **Handlers contain NO business logic**: Handlers decode JSON requests, enforce body size limits, extract authenticated identity, delegate orchestration to application services (`internal/service/`), and encode JSON envelopes.
2. **Handlers contain NO database access**: No SQL, pgx, or connection pool queries exist in HTTP handlers.
3. **Authorization enforced at service boundary**: HTTP handlers do not invent authorization checks; they invoke services which enforce Platform RBAC and BusinessManager ReBAC.
4. **Zero credential leakage**: Raw session tokens, OTP secrets, and password hashes are NEVER returned in response bodies.

---

## 2. Base URL & Protocol Versioning

All business endpoints are versioned under the `/api/v1` namespace. System monitoring endpoints reside at the root.

```
Root Base URL:     http://localhost:8080
API v1 Prefix:     /api/v1
Format:            application/json (UTF-8)
Date/Time Format:  RFC3339 (UTC)
```

---

## 3. Authentication & Session Model

Cafnet implements Go-native OTP phone authentication with opaque, persistent server-side sessions:

1. **OTP Request (`POST /api/v1/auth/otp/request`)**:
   - Accepts E.164 or local Ethiopian phone numbers (normalized via `domain.NormalizePhone`).
   - Rate limited by configurable cooldown (`OTPRequestCooldown`, default 60s).
   - Generates a high-entropy 6-digit verification code stored as a SHA-256 hash.
   - Returns a `challenge_id` and expiration timestamp. In `dev` mode, returns `dev_otp`.

2. **OTP Verification (`POST /api/v1/auth/otp/verify`)**:
   - Accepts `challenge_id`, `phone`, and `otp_code`.
   - On successful verification, issues an opaque 256-bit cryptographically secure random session token.
   - Emits an `HttpOnly`, `SameSite=Lax`, `Path=/`, `Secure` (in production) session cookie named `cafnet_session`.
   - **Security**: The raw session token is NEVER returned in the JSON response body.

3. **Session Transport**:
   - Browser clients automatically transmit the session cookie on subsequent requests (`credentials: 'include'`).
   - Optional Bearer token authorization header (`Authorization: Bearer <token>`) is supported for programmatic API clients.

4. **Logout (`POST /api/v1/auth/logout`)**:
   - Invalidates the persistent session in PostgreSQL and clears the browser cookie via `Max-Age: -1`.

---

## 4. Cross-Origin Resource Sharing (CORS) Policy

CORS is handled via strict whitelist matching rather than wildcard `*`:

- Whitelisted origins are configured via `APP_ALLOWED_ORIGINS` (defaults: `http://localhost:3000`, `http://127.0.0.1:3000`).
- Preflight (`OPTIONS`) requests receive:
  ```http
  Access-Control-Allow-Origin: <origin-if-whitelisted>
  Access-Control-Allow-Credentials: true
  Access-Control-Allow-Methods: GET, POST, PUT, PATCH, DELETE, OPTIONS
  Access-Control-Allow-Headers: Accept, Authorization, Content-Type, X-Request-ID
  Access-Control-Max-Age: 86400
  ```
- Wildcard `*` CORS with credentials is strictly prohibited.

---

## 5. CSRF Defense Policy

State-mutating requests (`POST`, `PUT`, `PATCH`, `DELETE`) carrying authentication cookies are subject to origin verification:
- Checks the `Origin` header against the allowed origin whitelist.
- If `Origin` is omitted, verifies the parsed `Referer` origin against the whitelist.
- If neither header is present or matches, rejects with `403 Forbidden` (`code: "forbidden"`).

---

## 6. Standard Envelope & Error Format

### 6.1 Success Envelope
Standard single-resource or action response:
```json
{
  "status": "success",
  "data": { ... }
}
```

### 6.2 Paginated Success Envelope
List endpoints return a uniform pagination metadata envelope:
```json
{
  "status": "success",
  "data": [ ... ],
  "pagination": {
    "limit": 20,
    "offset": 0,
    "has_more": false
  }
}
```

### 6.3 Standard Error Envelope
All application and domain errors map to a machine-readable structure:
```json
{
  "error": {
    "code": "invalid_input",
    "message": "Human-readable explanation of error",
    "details": null
  }
}
```

### 6.4 Canonical Error Code Mapping
| HTTP Status | Domain Error Code | Description |
|---|---|---|
| 400 Bad Request | `invalid_input` | Missing fields, malformed JSON, invalid query types |
| 400 Bad Request | `invalid_otp` | Incorrect verification code |
| 400 Bad Request | `otp_expired` | Expired verification challenge |
| 401 Unauthorized | `unauthenticated` | Missing or invalid session cookie |
| 401 Unauthorized | `session_expired` | Session TTL has elapsed |
| 401 Unauthorized | `session_revoked` | Session explicitly invalidated |
| 403 Forbidden | `forbidden` | Insufficient platform role or permission |
| 403 Forbidden | `not_business_manager` | User does not manage the target venue (IDOR defense) |
| 403 Forbidden | `self_approval_forbidden` | Admin attempting self-approval |
| 404 Not Found | `not_found` | Resource does not exist |
| 409 Conflict | `conflict` | Unique constraint violation (slug, phone, etc.) |
| 409 Conflict | `illegal_state_transition` | Invalid lifecycle transition (e.g. reviewing uncompleted visit) |
| 429 Too Many Requests | `rate_limited` | OTP request cooldown active |
| 429 Too Many Requests | `otp_too_many_attempts` | Verification attempt limit reached |
| 500 Internal Error | `internal_error` | Unexpected runtime error (details hidden from client) |
| 503 Unavailable | `database_unavailable` | Database connection lost or ping timeout |

---

## 7. Endpoint Catalog

### 7.1 Health & Readiness
| Method | Route | Auth | Description |
|---|---|---|---|
| `GET` | `/health` | Public | Liveness probe returning `{"status":"ok"}` |
| `GET` | `/ready` | Public | Readiness probe performing a bounded 5s ping to Neon PostgreSQL |

### 7.2 Authentication
| Method | Route | Auth | Description |
|---|---|---|---|
| `POST` | `/api/v1/auth/otp/request` | Public | Request SMS verification code (`{"phone":"+2519..."}`) |
| `POST` | `/api/v1/auth/otp/verify` | Public | Verify code, issue session, set `HttpOnly` cookie |
| `POST` | `/api/v1/auth/logout` | Public/Auth | Invalidate session and clear session cookie |
| `GET` | `/api/v1/auth/me` | Required | Retrieve current authenticated user profile and platform role |

### 7.3 Public Discovery
| Method | Route | Auth | Description |
|---|---|---|---|
| `GET` | `/api/v1/businesses` | Optional | Browse published venues with filters (`category`, `neighborhood`, `limit`, `offset`) |
| `GET` | `/api/v1/businesses/{id}` | Optional | Retrieve public business profile by numeric ID or slug |
| `GET` | `/api/v1/slugs/{slug}` | Optional | Retrieve public business profile by unique URL slug |
| `GET` | `/api/v1/search` | Optional | Full-text directory search (`?q=...&limit=...`) |

### 7.4 Customer Interactions
| Method | Route | Auth | Description |
|---|---|---|---|
| `POST` | `/api/v1/saved/{businessID}` | Required | Save venue to customer bookmark list |
| `DELETE` | `/api/v1/saved/{businessID}` | Required | Remove venue from customer bookmarks |
| `GET` | `/api/v1/saved` | Required | List all venues bookmarked by authenticated customer |
| `POST` | `/api/v1/following/{businessID}` | Required | Subscribe to venue broadcasts |
| `DELETE` | `/api/v1/following/{businessID}` | Required | Unsubscribe from venue broadcasts |
| `GET` | `/api/v1/following` | Required | List all venues followed by customer |
| `POST` | `/api/v1/claims` | Required | Submit venue ownership claim (`{"business_id": ...}`) |
| `GET` | `/api/v1/claims` | Required | List claims submitted by authenticated user |
| `GET` | `/api/v1/claims/{id}` | Required | Get details of a single user claim |
| `POST` | `/api/v1/submissions` | Required | Submit venue suggestion for admin review |
| `GET` | `/api/v1/submissions` | Required | List venue suggestions submitted by user |
| `GET` | `/api/v1/submissions/{id}` | Required | Get details of a single user submission |

### 7.5 Business Management (ReBAC Protected)
| Method | Route | Auth | Description |
|---|---|---|---|
| `GET` | `/api/v1/businesses/managed` | Required | List all venues managed by authenticated manager |
| `GET` | `/api/v1/businesses/{id}/manage` | Manager | Retrieve private management profile for venue |
| `PATCH` | `/api/v1/businesses/{id}` | Manager | Update venue details (name, cuisine, descriptions, phone, etc.) |
| `GET` | `/api/v1/businesses/{id}/hours` | Manager | Retrieve operating hours schedule |
| `PUT` | `/api/v1/businesses/{id}/hours` | Manager | Set 7-day operating hours schedule |
| `GET` | `/api/v1/businesses/{id}/images` | Manager | Retrieve gallery photos |
| `PUT` | `/api/v1/businesses/{id}/images` | Manager | Set gallery photos with ordering |

### 7.6 Menu Management (ReBAC Protected)
| Method | Route | Auth | Description |
|---|---|---|---|
| `GET` | `/api/v1/businesses/{id}/menu` | Optional | Retrieve public full menu (categories + items) |
| `POST` | `/api/v1/businesses/{id}/menu/categories` | Manager | Create menu category |
| `PATCH` | `/api/v1/businesses/{id}/menu/categories/{categoryID}` | Manager | Update category name/description |
| `DELETE` | `/api/v1/businesses/{id}/menu/categories/{categoryID}` | Manager | Delete category and its items |
| `PUT` | `/api/v1/businesses/{id}/menu/categories/reorder` | Manager | Reorder category display hierarchy |
| `POST` | `/api/v1/businesses/{id}/menu/items` | Manager | Add item to category |
| `PATCH` | `/api/v1/businesses/{id}/menu/items/{itemID}` | Manager | Update item price, availability, details |
| `DELETE` | `/api/v1/businesses/{id}/menu/items/{itemID}` | Manager | Remove menu item |
| `PUT` | `/api/v1/businesses/{id}/menu/categories/{categoryID}/items/reorder` | Manager | Reorder items within category |

### 7.7 Venue Broadcast Updates
| Method | Route | Auth | Description |
|---|---|---|---|
| `GET` | `/api/v1/updates` | Optional | List active broadcasts across all published venues (discovery/Explore) |
| `GET` | `/api/v1/following/updates` | Required | List active broadcasts from venues followed by user |
| `GET` | `/api/v1/businesses/{id}/updates` | Optional | List public broadcast updates for venue |
| `POST` | `/api/v1/businesses/{id}/updates` | Manager | Publish new broadcast update |
| `PATCH` | `/api/v1/businesses/{id}/updates/{updateID}` | Manager | Update existing broadcast post |
| `DELETE` | `/api/v1/businesses/{id}/updates/{updateID}` | Manager | Delete broadcast post |

### 7.8 Table Reservations
| Method | Route | Auth | Description |
|---|---|---|---|
| `POST` | `/api/v1/reservations` | Required | Customer creates table reservation |
| `GET` | `/api/v1/reservations` | Required | Customer lists their upcoming/past reservations |
| `GET` | `/api/v1/reservations/{id}` | Required | Customer views single reservation |
| `POST` | `/api/v1/reservations/{id}/cancel` | Required | Customer cancels pending reservation |
| `GET` | `/api/v1/businesses/{id}/reservations` | Manager | Manager lists venue reservations |
| `GET` | `/api/v1/businesses/{id}/reservations/{resID}` | Manager | Manager views single venue reservation |
| `POST` | `/api/v1/businesses/{id}/reservations/{resID}/confirm` | Manager | Confirm pending reservation |
| `POST` | `/api/v1/businesses/{id}/reservations/{resID}/decline` | Manager | Decline reservation with reason |
| `POST` | `/api/v1/businesses/{id}/reservations/{resID}/complete` | Manager | Mark reservation as completed visit |

### 7.9 Verified Dining Reviews & Social Feedback
| Method | Route | Auth | Description |
|---|---|---|---|
| `GET` | `/api/v1/businesses/{id}/reviews` | Optional | List verified reviews for venue |
| `GET` | `/api/v1/businesses/{id}/reviews/summary` | Optional | Aggregated rating & review score breakdown |
| `POST` | `/api/v1/reviews` | Required | Submit verified review (**requires completed reservation**) |
| `PATCH` | `/api/v1/reviews/{id}` | Author | Update review rating or commentary |
| `DELETE` | `/api/v1/reviews/{id}` | Author | Remove review |
| `POST` | `/api/v1/reviews/{id}/helpful` | Required | Vote review helpful |
| `DELETE` | `/api/v1/reviews/{id}/helpful` | Required | Remove helpful vote |
| `GET` | `/api/v1/reviews/mine` | Required | List all reviews written by authenticated user |

### 7.10 Community Reports
| Method | Route | Auth | Description |
|---|---|---|---|
| `POST` | `/api/v1/reviews/{id}/reports` | Required | Flag inappropriate review for moderation |
| `POST` | `/api/v1/businesses/{id}/reports` | Required | Flag venue listing inaccuracy or policy violation |

### 7.11 Platform Administration (Admin Role Required)
| Method | Route | Auth | Description |
|---|---|---|---|
| `GET` | `/api/v1/admin/submissions` | Admin | List pending venue recommendation submissions |
| `POST` | `/api/v1/admin/submissions/{id}/approve` | Admin | Approve submission and generate business listing |
| `POST` | `/api/v1/admin/submissions/{id}/reject` | Admin | Reject submission with note |
| `GET` | `/api/v1/admin/claims` | Admin | List pending business ownership claims |
| `POST` | `/api/v1/admin/claims/{id}/approve` | Admin | Approve claim and assign manager role |
| `POST` | `/api/v1/admin/claims/{id}/reject` | Admin | Reject claim with note |
| `GET` | `/api/v1/admin/reviews/reports` | Admin | List pending review moderation reports |
| `POST` | `/api/v1/admin/reviews/reports/{id}/resolve`| Admin | Resolve review report |
| `POST` | `/api/v1/admin/reviews/{id}/moderate` | Admin | Moderate review (approved, rejected, flagged) |
| `GET` | `/api/v1/admin/business-reports` | Admin | List pending venue reports |
| `POST` | `/api/v1/admin/business-reports/{id}/resolve`| Admin | Resolve venue report |
| `POST` | `/api/v1/admin/businesses/{id}/publication` | Admin | Toggle published/unpublished visibility |
| `POST` | `/api/v1/admin/businesses/{id}/verification`| Admin | Grant/revoke verified authenticity badge |
| `GET` | `/api/v1/admin/actions` | Admin | Query immutable administrative audit trail logs |

---

## 8. Security Guarantees & IDOR Defenses

1. **Manager IDOR Protection**: A manager holding credentials for Business A who attempts to mutate Business B (`PATCH /api/v1/businesses/{id_B}`, menu categories, menu items, or reservation status) is strictly halted with `403 Forbidden` (`code: "not_business_manager"`).
2. **Review Integrity**: Customers cannot submit a review unless they have a completed reservation (`status: "COMPLETED"`) for that business. Any attempt prior to completion results in `409 Conflict` (`code: "illegal_state_transition"`).
3. **Admin Self-Approval Prohibition**: Platform administrators who submit a claim or venue suggestion cannot approve their own requests.
4. **Panic Recovery**: Any runtime panic in an HTTP handler is caught by the recovery middleware and gracefully converted to a `500 Internal Server Error` JSON response without crashing the server or leaking stack traces.
5. **Request Bounds**: Inbound JSON payloads are bounded with `http.MaxBytesReader` to prevent memory exhaustion DoS attacks.
