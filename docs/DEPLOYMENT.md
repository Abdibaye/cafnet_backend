# Cafnet Production Deployment & Operational Hardening (Backend 09)

**Document ID:** `CAFNET-BACKEND-09-DEPLOYMENT`  
**Status:** Canonical Production Runbook  
**Topology:** Vercel (Next.js 16) → Render (Go 1.24 API) → Neon (PostgreSQL 16)  

---

## 1. Production Topology & Trust Boundaries

```
                      END USER BROWSER
                             │
                ┌────────────┴────────────┐
                │                         │
             HTTPS                     HTTPS
       (credentials: "include")           │
                │                         │
                ▼                         ▼
         VERCEL PLATFORM            RENDER CLOUD
    Next.js UI (Edge / Server)       Go REST API
        (cafnet-web.vercel.app)     (cafnet-api.onrender.com)
                │                         │
                │ (Public REST API)       │ (Direct TLS Connection)
                └────────────────────────►│
                                          ▼
                                   NEON POSTGRESQL
                                (AWS us-east-2 Pooler)
```

### Critical Security Boundaries
1. **No Direct Database Access from Frontend**: Next.js and the client browser never connect to Neon PostgreSQL. All database access is mediated by the Go API.
2. **Zero Database Credentials in Client**: The client browser never receives `DATABASE_URL` or database credentials in HTML, scripts, or network responses.
3. **Zero Token Storage in JavaScript**: The opaque session token is transmitted exclusively via an `HttpOnly`, `Secure`, `SameSite=None` cookie (`cafnet_session`). It is never placed in `localStorage`, `sessionStorage`, or JavaScript memory.
4. **Isolated Deployments**: Deploying the Next.js frontend on Vercel or the Go API on Render does not drop, recreate, or reseed the Neon database.

---

## 2. Render Web Service Configuration

| Setting | Value | Rationale |
| :--- | :--- | :--- |
| **Service Type** | Web Service | Long-running HTTP/REST API service |
| **Environment / Runtime** | Docker | Uses multi-stage [Dockerfile](file:///home/abdi/cafnetweb/backend/Dockerfile) with Go compiler and minimal Alpine runtime |
| **Alternative (Native Go)** | Go | Build: `go build -tags netgo -ldflags "-s -w" -o app ./cmd/server` <br/> Start: `./app` |
| **Region** | Ohio / US East (`us-east-2`) | Co-located with Neon AWS us-east-2 pooler for sub-millisecond query latency |
| **Branch** | `main` | Automatic deployments triggered on git push |
| **Health Check Path** | `/health` | Minimal liveness check returning `{"status":"ok"}` without exposing internal state |
| **PORT Handling** | Read dynamically | Server reads `os.Getenv("PORT")` and listens on `0.0.0.0:$PORT` |

---

## 3. Required Environment Variables

### Backend (Render Web Service)

| Environment Variable | Production Value | Staging / Test Value | Description |
| :--- | :--- | :--- | :--- |
| `DATABASE_URL` | `postgresql://...aws.neon.tech/neondb?sslmode=require` | Same or isolated branch | Neon PostgreSQL connection string with SSL enabled |
| `APP_ENV` | `production` | `staging` | Environment mode; disables dev OTP in production |
| `AUTH_DEV_MODE` | `false` | `true` | Must be `false` in production to prevent deterministic OTP codes |
| `AUTH_DETERMINISTIC_OTP` | *Omitted* | `123456` | Test verification code (ignored if `AUTH_DEV_MODE=false`) |
| `AUTH_ALLOWED_ORIGINS` | `https://cafnet-web.vercel.app` | `https://cafnet-web.vercel.app,http://localhost:3000` | Explicit CORS allowed origins (no wildcards) |
| `AUTH_COOKIE_SECURE` | `true` | `true` | Enforces `Secure` attribute on HTTPS session cookies |
| `AUTH_COOKIE_SAME_SITE` | `None` | `None` | Required for cross-domain cookie exchange between Vercel and Render |
| `AUTH_SESSION_COOKIE_NAME` | `cafnet_session` | `cafnet_session` | Name of the HttpOnly session cookie |
| `AUTH_SESSION_TTL` | `720h` | `720h` | 30-day session lifetime |
| `AUTH_OTP_TTL` | `5m` | `5m` | 5-minute OTP challenge expiration |
| `AUTH_OTP_MAX_ATTEMPTS` | `3` | `3` | Max verification attempts before challenge invalidation |
| `AUTH_OTP_REQUEST_COOLDOWN`| `60s` | `60s` | Minimum cooldown between OTP requests for the same phone |
| `DB_MAX_CONNS` | `10` | `10` | Conservative pool limit respecting Neon serverless bounds |
| `DB_MIN_CONNS` | `2` | `2` | Minimum idle connections ready in pool |
| `DB_MAX_CONN_LIFETIME` | `30m` | `30m` | Connection recycle duration |
| `DB_MAX_CONN_IDLE_TIME` | `5m` | `5m` | Idle connection teardown threshold |

### Frontend (Vercel Project)

| Environment Variable | Production Value | Description |
| :--- | :--- | :--- |
| `NEXT_PUBLIC_API_URL` | `https://<your-api>.onrender.com` | Public HTTPS base URL of the Go REST API (no trailing slash) |

---

## 4. Health, Liveness, and Readiness

The server exposes two distinct probe endpoints:

1. **Liveness Check** (`GET /health` or `GET /api/v1/health`):
   - **Purpose**: Used by Render health checks and container orchestrators to detect if the HTTP process is alive.
   - **Payload**:
     ```json
     {
       "data": {
         "status": "ok"
       }
     }
     ```
   - **Security**: Returns HTTP 200 without touching database pools or exposing environment internals.

2. **Readiness Check** (`GET /ready` or `GET /api/v1/ready`):
   - **Purpose**: Verifies that the Neon PostgreSQL database pool is connected and responsive.
   - **Payload**:
     ```json
     {
       "data": {
         "status": "ready",
         "database": "connected"
       }
     }
     ```
   - **Failure Mode**: Returns HTTP 503 Service Unavailable if database ping fails within 5 seconds.

---

## 5. Security & Invariant Policies

### 5.1 CORS Policy
- Configured via `AUTH_ALLOWED_ORIGINS`.
- Wildcards (`*`) are strictly disallowed when `Access-Control-Allow-Credentials: true` is enabled.
- Preflight (`OPTIONS`) requests receive `204 No Content` with explicit allowed headers (`Content-Type`, `Authorization`, `X-Request-ID`, `X-CSRF-Token`) and a `Vary: Origin` header.

### 5.2 Session Cookie Hardening
- **Cookie Name**: `cafnet_session`
- **Attributes**: `HttpOnly; Secure; SameSite=None; Path=/; Max-Age=2592000`
- Because the frontend is hosted on `vercel.app` and the API is on `onrender.com`, `SameSite=None` together with `Secure=true` is mandatory for modern browsers (Chrome, Safari, Firefox) to persist and transmit cookies cross-site on fetch requests.

### 5.3 CSRF Protection
- All state-mutating requests (`POST`, `PUT`, `PATCH`, `DELETE`) with an active session cookie are validated against the `Origin` or `Referer` headers.
- If the origin does not match the whitelist, the server returns `403 Forbidden` (`cross-origin request forbidden by CSRF policy`).

### 5.4 Defensive Security Headers
Every HTTP response automatically includes:
- `X-Content-Type-Options: nosniff`
- `X-Frame-Options: DENY`
- `Referrer-Policy: strict-origin-when-cross-origin`
- `Content-Security-Policy: default-src 'none'; frame-ancestors 'none'`
- `X-XSS-Protection: 0`

### 5.5 Panic Recovery
- Catches any unhandled panic during request processing.
- Logs the error and stack trace to stderr along with the request ID.
- Returns a standardized JSON error envelope: `{"error":{"code":"internal_server_error","message":"An unexpected server error occurred","request_id":"..."}}`.

---

## 6. Neon Database Migration Procedure

Migrations are executed deliberately and never run automatically during web service startup. This prevents race conditions in multi-instance setups and avoids accidental schema corruption.

### 6.1 Inspect Migration Status
```bash
cd backend
go run cmd/migrate/main.go status
```

### 6.2 Apply Pending Migrations (`up`)
```bash
cd backend
go run cmd/migrate/main.go up
```
Every migration executes inside an atomic database transaction (`BEGIN ... COMMIT`). If an error occurs, the entire transaction rolls back cleanly.

### 6.3 Revert Last Migration (`down`)
```bash
cd backend
go run cmd/migrate/main.go down
```

---

## 7. Graceful Shutdown & Signal Handling

On `SIGTERM` or `SIGINT` (such as during a Render redeployment):
1. The server stops accepting new incoming HTTP connections.
2. A 15-second grace period is granted to allow in-flight requests to complete.
3. The server closes the listener.
4. The database connection pool (`pool.Close()`) is cleanly flushed and closed.
5. The process exits with code 0.

---

## 8. Production Deployment Checklist

- [ ] `DATABASE_URL` is set in Render environment variables with `sslmode=require`.
- [ ] `AUTH_ALLOWED_ORIGINS` in Render matches the exact Vercel frontend URL (e.g. `https://cafnet-web.vercel.app`).
- [ ] `AUTH_COOKIE_SAME_SITE` is set to `None` and `AUTH_COOKIE_SECURE` is `true`.
- [ ] `NEXT_PUBLIC_API_URL` in Vercel points to the HTTPS Render API domain without a trailing slash.
- [ ] Health check probe passes at `/health`.
- [ ] Database readiness check passes at `/ready`.
- [ ] Guest exploration and search load venues from Neon PostgreSQL.
- [ ] OTP sign-in sets `cafnet_session` cookie cross-domain.
- [ ] Customer save/follow mutations update database state and persist across refreshes.
- [ ] Sign-out revokes session in Neon and clears the cookie.
