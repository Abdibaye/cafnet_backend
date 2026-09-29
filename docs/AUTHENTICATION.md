# Cafnet Authentication & Session Identity Foundation
**Phase:** BACKEND 04 — Go-Native Authentication + Persistent Session Foundation  
**Target Stack:** Go 1.27+, Neon PostgreSQL 18.6, HTTP-Only Cookie Sessions  
**Document ID:** `CAFNET-BACKEND-04-AUTH`  
**Status:** Canonical Implementation  

---

## 1. Architectural Philosophy: Authentication vs. Authorization

Cafnet enforces a strict architectural boundary between identity and permissions:

```
┌────────────────────────────────────────────────────────┐
│               AUTHENTICATION (Backend 04)              │
│                 "WHO IS THIS USER?"                    │
│                                                        │
│  Phone E.164  ──►  OTP Challenge  ──►  Session Cookie  │
│  (Canonical)       (Cryptographic)     (Opaque Token)  │
└──────────────────────────┬─────────────────────────────┘
                           │
                           ▼ (Resolves Authenticated domain.User)
┌────────────────────────────────────────────────────────┐
│               AUTHORIZATION (Backend 05)               │
│             "WHAT CAN THIS USER ACCESS?"               │
│                                                        │
│  Venue Managers    Admin Role       Resource Ownership │
│  (ReBAC via M:N)   (RBAC via Role)  (Review Author...) │
└────────────────────────────────────────────────────────┘
```

- **Authentication** is strictly concerned with establishing verified identity via cryptographic phone OTP verification and issuing high-entropy opaque session tokens.
- **Authorization** (business management, menu editing, reservation approval, platform moderation) is deferred to Backend 05 and consumes the authenticated `*domain.User` injected into the request context.

---

## 2. Phone Number Normalization & Identity Resolution

A user's phone number is their sole authentication identity. To ensure that formatting variations do not produce duplicate user records, all phone inputs are sanitized into canonical **E.164** format (`+2519XXXXXXXX` or `+2517XXXXXXXX`):

### Normalization Logic (`internal/domain/phone.go`)
1. Strip all non-digit and non-plus characters (` `, `-`, `(`, `)`, `.`).
2. Identify Ethiopian mobile prefixes: `9` (Ethio telecom) or `7` (Safaricom Ethiopia).
3. Transform input variants:
   - `0911223344` / `0711223344` $\to$ `+251911223344` / `+251711223344`
   - `251911223344` $\to$ `+251911223344`
   - `911223344` $\to$ `+251911223344`
   - `+251911223344` $\to$ `+251911223344` (canonical)
4. Reject any input that does not yield a 9-digit subscriber body starting with 9 or 7 with `domain.ErrInvalidPhone`.

---

## 3. OTP Lifecycle & Security

```
      User submits phone
              │
              ▼
      Normalize Phone
              │
              ▼
      Cooldown Check ──────(Within cooldown window)───► Return ErrOTPRequestTooSoon
              │
              ▼ (Cooldown elapsed)
    Invalidate Previous Challenges for Phone
              │
              ▼
    Generate 6-digit Code (crypto/rand or Dev OTP)
              │
              ▼
    Compute Salted Hash: SHA-256(phone + ":" + otp)
              │
              ▼
    Persist Challenge in PostgreSQL (otp_challenges)
              │
              ▼
    Dispatch via SMSProvider (mock / telecom gateway)
```

### Security Measures:
1. **Never Plaintext in DB:** Persistent storage holds only the salted SHA-256 digest (`otp_hash`). Plaintext codes exist exclusively in transit and in ephemeral memory.
2. **Timing-Attack Resistance:** Verification compares hashes using `crypto/subtle.ConstantTimeCompare`.
3. **Attempt Bounding:** Challenges permit at most `AUTH_OTP_MAX_ATTEMPTS` (default: 3). Mismatches increment the counter. Once exhausted, the challenge returns `domain.ErrOTPTooManyAttempts`.
4. **Replay Protection:** Successfully verified challenges are immediately marked `is_consumed = TRUE, consumed_at = NOW()`. Subsequent attempts return `domain.ErrOTPAlreadyUsed`.
5. **Request Cooldown:** Users cannot spam OTP generation. Requests within `AUTH_OTP_REQUEST_COOLDOWN` (default: 60s) return `domain.ErrOTPRequestTooSoon`.
6. **Account Enumeration Resistance:** OTP generation behaves identically regardless of whether the phone number belongs to an existing user or a new visitor.

---

## 4. Atomic Transaction Boundary (Verify & Issue)

When a user submits an OTP code, verification, identity resolution, user creation (if new), and session issuance occur inside an explicit ACID transaction:

```sql
BEGIN;

-- 1. Lock challenge row to serialize concurrent verification requests
SELECT * FROM otp_challenges WHERE id = $1 FOR UPDATE;

-- 2. Validate unconsumed, unexpired, attempts < max_attempts, hash match
-- 3. Mark consumed
UPDATE otp_challenges SET is_consumed = TRUE, consumed_at = NOW() WHERE id = $1;

-- 4. Lookup or create User
INSERT INTO users (phone, full_name, role) VALUES (...) ON CONFLICT DO NOTHING;

-- 5. Issue session
INSERT INTO sessions (user_id, token_hash, expires_at) VALUES (...);

COMMIT;
```

If any step fails, the transaction rolls back completely, ensuring no orphan users or sessions are created. If the verification fails due to a wrong code, the attempt counter is incremented persistently outside the aborted transaction.

---

## 5. Persistent Session Model & Security

Cafnet avoids stateless JWTs in favor of server-side revocable session records:

1. **High-Entropy Token:** Generated using 32 cryptographically secure random bytes from `crypto/rand`, hex-encoded to 64 characters (256 bits of entropy).
2. **Server-Side Token Hash:** The database stores `token_hash = sha256(raw_token)`. If the database is compromised, active session credentials cannot be extracted.
3. **Revocation Support:** Sessions can be revoked individually (`Logout`) or across all devices (`LogoutAll`) by updating `revoked_at = NOW()`.
4. **Audit & Activity Tracking:** Tracks `created_at`, `expires_at`, `last_used_at`, `user_agent`, and `ip_address`.
5. **Garbage Collection:** Expired sessions are purged via `SessionRepository.DeleteExpired()`.

---

## 6. HTTP Cookie Strategy & Client Storage

When the Go backend is wired to HTTP transport (Backend 06), session tokens are transmitted via modern security-hardened HTTP cookies:

```http
Set-Cookie: cafnet_session=<raw_64_char_hex_token>; 
            Path=/; 
            HttpOnly; 
            Secure; 
            SameSite=Lax; 
            Max-Age=2592000
```

### Strict Client Storage Rules:
- **`HttpOnly`:** Inaccessible to JavaScript (`document.cookie`), neutralizing Cross-Site Scripting (XSS) token theft.
- **`Secure`:** Sent only over HTTPS connections in production (`AUTH_COOKIE_SECURE=true`).
- **`SameSite=Lax`:** Prevents cross-site cookie transmission on third-party cross-origin subrequests while permitting top-level navigation.
- **Forbidden Storage:** Session credentials must NEVER be placed in `localStorage`, `sessionStorage`, URL parameters, or client state stores.

---

## 7. CSRF & CORS Deployment Architecture

### Deployment Arrangement
- **Frontend:** Next.js (Node / Edge runtime) running on domain e.g. `https://cafnet.et` (or `http://localhost:3000` in dev).
- **Backend:** Go API service running on `https://api.cafnet.et` (or `http://localhost:8080` in dev).

### Cross-Origin Request Policy (CORS):
- `Access-Control-Allow-Origin` is dynamically matched against configured origins in `AUTH_ALLOWED_ORIGINS`.
- **Wildcard origins (`*`) are strictly prohibited** because `Access-Control-Allow-Credentials: true` is required for cookies.

### CSRF Protection:
1. `SameSite=Lax` cookies block default cross-origin POST/PATCH/DELETE requests initiated by third-party origins.
2. For state-changing mutations (`POST`, `PUT`, `PATCH`, `DELETE`), the Go backend validates the `Origin` and `Referer` headers against `AUTH_ALLOWED_ORIGINS`.
3. In cross-subdomain architectures where Next.js proxies API calls (e.g. Next.js Route Handlers `/api/*`), requests terminate on the same origin, further reducing attack surface.

---

## 8. Development Mode & Deterministic OTP

To streamline automated integration testing and local frontend development without telecom SMS costs:
- When `AUTH_DEV_MODE=true`, `RequestOTP` supports a deterministic verification code (`AUTH_DETERMINISTIC_OTP`, default: `"123456"`).
- **Production Guard:** `AuthConfig.Validate()` enforces that `AUTH_DEV_MODE` cannot be enabled if `APP_ENV=production`. If attempted, the application fails to boot.
- Production systems never log or expose OTP verification codes.

---

## 9. Centralized Configuration Reference

| Environment Variable | Type | Default | Description |
|---|---|---|---|
| `AUTH_OTP_TTL` | Duration | `5m` | Lifetime of an OTP challenge |
| `AUTH_OTP_MAX_ATTEMPTS` | Integer | `3` | Maximum incorrect verification attempts permitted |
| `AUTH_OTP_REQUEST_COOLDOWN` | Duration | `60s` | Minimum delay between consecutive OTP requests for a phone |
| `AUTH_SESSION_TTL` | Duration | `720h` (30 days) | Session validity duration before expiration |
| `AUTH_SESSION_COOKIE_NAME` | String | `cafnet_session` | Name of the authentication cookie |
| `AUTH_ALLOWED_ORIGINS` | Comma-separated | `http://localhost:3000` | Trusted frontend origins for CORS & CSRF validation |
| `AUTH_DEV_MODE` | Boolean | `false` | Enables deterministic dev OTP (forbidden in production) |
| `AUTH_DETERMINISTIC_OTP` | String | `123456` | Deterministic verification code for development |
| `AUTH_COOKIE_SECURE` | Boolean | `true` in prod | Restricts cookie transmission to HTTPS |
| `AUTH_COOKIE_SAME_SITE` | String | `Lax` | Cookie SameSite enforcement mode |

---

## 10. Future HTTP API Endpoint Specifications (Backend 06)

These planned endpoint contracts define the external HTTP boundary to be implemented in Backend 06:

### `POST /auth/otp/request`
- **Request Body:**
  ```json
  { "phone": "0911223344" }
  ```
- **Response `200 OK`:**
  ```json
  {
    "challenge_id": "b3e0c039-4482-4c22-b5e1-8848d7be88bb",
    "expires_at": "2026-09-29T12:05:00Z",
    "cooldown_seconds": 60
  }
  ```

### `POST /auth/otp/verify`
- **Request Body:**
  ```json
  {
    "challenge_id": "b3e0c039-4482-4c22-b5e1-8848d7be88bb",
    "phone": "0911223344",
    "otp_code": "123456"
  }
  ```
- **Response `200 OK`:**
  - Headers: `Set-Cookie: cafnet_session=...; HttpOnly; Secure; SameSite=Lax`
  - Body:
    ```json
    {
      "user": {
        "id": "c1f7b0f3-8eb4-47b2-9d33-d9487c69992a",
        "phone": "+251911223344",
        "full_name": "Customer 3344",
        "role": "CUSTOMER",
        "is_active": true
      },
      "is_new_user": true
    }
    ```

### `POST /auth/logout`
- **Headers:** `Cookie: cafnet_session=...`
- **Response `200 OK`:**
  - Headers: `Set-Cookie: cafnet_session=; Max-Age=0`
  - Body:
    ```json
    { "success": true }
    ```

### `GET /auth/me`
- **Headers:** `Cookie: cafnet_session=...`
- **Response `200 OK`:**
  ```json
  {
    "user": {
      "id": "c1f7b0f3-8eb4-47b2-9d33-d9487c69992a",
      "phone": "+251911223344",
      "full_name": "Customer 3344",
      "role": "CUSTOMER",
      "is_active": true
    }
  }
  ```
