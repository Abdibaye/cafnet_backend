# Business Center Menu Management API Migration (Backend 14)

## 1. Overview & Architecture

Backend 14 migrates the Cafnet Business Center menu management subsystem from legacy mock/localStorage persistence to the canonical Go REST API backed by Neon PostgreSQL.

### Architecture Overview
```
Vercel (Next.js Client)
  ↓
Next.js App Router (`/business/dashboard/business/[id]/menu`)
  ↓
Typed Frontend API Client (`lib/api/menus.ts`)
  ↓
Go REST API on Render (`internal/http/handler/menu.go`)
  ↓
Application Service Layer (`internal/service/menu_service.go`)
  ↓ ReBAC Verification & IDOR Defense (`internal/service/authorization_service.go`)
Neon PostgreSQL (`menu_categories`, `menu_items`)
```

---

## 2. Security & ReBAC Authorization (IDOR Defense)

A fundamental security requirement is authoritative, server-side access control:
- Managing Business A grants **ZERO** permissions over Business B.
- URL parameters, route business IDs, or frontend state are never trusted.
- Every mutation must pass `authzService.CanManageMenu(ctx, actor, businessID)`.
- **IDOR Protections**:
  - Updating a category verifies `cat.BusinessID == businessID`.
  - Updating or deleting an item verifies `item.BusinessID == businessID`.
  - Creating an item verifies the target category belongs to that business (`cat.BusinessID == businessID`).
  - Reordering categories or items validates venue ownership inside an atomic PostgreSQL transaction.

---

## 3. Money Representation & Precision

All monetary prices are stored and transmitted in **Ethiopian Santims** as integer minor units (`int64`, 1 ETB = 100 Santims):
- `125 ETB` $\to$ `12500` santims
- `125.50 ETB` $\to$ `12550` santims
- `0 ETB` $\to$ `0` santims
- Floating-point representations (`float64`) are strictly prevented in database models and mutations.
- Bounds validation enforces `0 <= price <= 100,000,000` santims (`ETB 1,000,000`).
- The frontend adapter `parsePriceToMinorSantims` provides resilient user input parsing (stripping currency tags, whitespace, commas) and formats output with `formatPriceEtb`.

---

## 4. Canonical REST API Endpoints

### 4.1 Menu Read
- **`GET /api/v1/businesses/{id}/menu`**
  - **Auth**: Public / Optional
  - **Description**: Returns all categories and contained items for public customer venue profiles and manager dashboards.

### 4.2 Category Operations
- **`POST /api/v1/businesses/{id}/menu/categories`**
  - **Auth**: Manager (ReBAC)
  - **Payload**: `{"name": "Breakfast", "sort_order": 0}`
- **`PATCH /api/v1/businesses/{id}/menu/categories/{categoryID}`**
  - **Auth**: Manager (ReBAC + IDOR)
  - **Payload**: `{"name": "All-Day Breakfast"}`
- **`DELETE /api/v1/businesses/{id}/menu/categories/{categoryID}`**
  - **Auth**: Manager (ReBAC + IDOR)
  - **Behavior**: Cascading soft-delete of category and contained items inside a transaction.
- **`PUT /api/v1/businesses/{id}/menu/categories/reorder`**
  - **Auth**: Manager (ReBAC)
  - **Payload**: `{"category_ids": ["uuid-2", "uuid-1"]}`

### 4.3 Item Operations
- **`POST /api/v1/businesses/{id}/menu/items`**
  - **Auth**: Manager (ReBAC + IDOR)
  - **Payload**:
    ```json
    {
      "category_id": "uuid-cat-1",
      "name": "Buna",
      "description": "Single-origin organic roast",
      "price_etb_minor": 4500,
      "is_available": true,
      "sort_order": 0
    }
    ```
- **`PATCH /api/v1/businesses/{id}/menu/items/{itemID}`**
  - **Auth**: Manager (ReBAC + IDOR)
  - **Payload**: `{"price_etb_minor": 5000, "description": "Updated roast"}`
- **`DELETE /api/v1/businesses/{id}/menu/items/{itemID}`**
  - **Auth**: Manager (ReBAC + IDOR)
- **`PUT /api/v1/businesses/{id}/menu/categories/{categoryID}/items/reorder`**
  - **Auth**: Manager (ReBAC + IDOR)
  - **Payload**: `{"item_ids": ["uuid-item-2", "uuid-item-1"]}`

---

## 5. Frontend Migration & Cleanup

1. **`frontend/lib/api/menus.ts`**:
   - Implements typed async functions for all menu CRUD and reorder endpoints with standard timeout and credentials handling.
2. **`frontend/lib/api/adapters.ts`**:
   - Added `parsePriceToMinorSantims` for converting customer/owner inputs to santims.
   - Added `apiMenuToViewModel` and `computeMenuSummary`.
3. **`frontend/components/ManageMenuPage.tsx`**:
   - Replaced all calls to `menuStorage.ts` with real API calls.
   - Added ReBAC 403 Forbidden access-restricted state.
   - Added loading skeleton, error retry banners, and clean empty states.
4. **`frontend/components/BusinessProfileMenuSection.tsx`**:
   - Removed mock fallback; displays real API menu data.
5. **`frontend/components/ManagedBusinessPage.tsx`**:
   - Loads menu summary via `getBusinessMenu` and `computeMenuSummary` dynamically.
6. **Obsolete Mock Removal**:
   - Disconnected `data/menuStorage.ts` from all owner and customer flows.

---

## 6. Verification & Test Results

- **Backend Integration Tests (`backend/tests/http_test.go`)**:
  - `TestMenuManagementHTTP` (PASS, 57.97s):
    - Public menu read $\to$ 200
    - Customer mutation on Biz A $\to$ 403 Forbidden
    - Biz A manager mutation on Biz B $\to$ 403 Forbidden
    - Category create $\to$ update $\to$ reorder
    - Item create with santim units $\to$ negative price 400 rejection $\to$ update $\to$ reorder
    - Delete item $\to$ Delete category
    - Final state verification.
  - `TestBusinessAndMenuService` (PASS, 18.72s).
  - `TestMenuRepository` (PASS, 11.96s).
- **Frontend Integration Tests (`frontend/tests/api_integration.test.ts`)**:
  - 23 of 23 test suites passed.
- **Frontend Build (`npm run build`)**:
  - All 27 static and dynamic routes compiled cleanly.
