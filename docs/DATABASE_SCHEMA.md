# CAFNET DATABASE SCHEMA SPECIFICATION
**Document ID:** `CAFNET-BACKEND-02-DATABASE-SCHEMA`  
**Status:** Canonical PostgreSQL Schema Specification  
**Target Engine:** Neon Serverless PostgreSQL 16+ (PostgreSQL 18.6 Verified)  
**Database Name:** `neondb`  

---

## 1. Schema Architecture & Design Principles

The Cafnet database schema directly translates the domain boundaries defined in **Backend 01** into relational structures. The database enforces core platform invariants at the lowest storage layer using PostgreSQL check constraints, foreign keys, unique indexes, and procedural triggers.

### Architectural Rules Enforced in PostgreSQL:
1. **Separation of Platform Dimensions:** Business existence, publication state, verification status, and claim status are stored as distinct, independent columns.
2. **Verified Dining Feedback:** Reviews cannot be created for unconfirmed or pending reservations. Database triggers guarantee that reviews can only be attached to reservations with `status = 'COMPLETED'`.
3. **No Duplicate Bookmarks or Votes:** Composite primary keys prevent duplicate saves, follows, and helpful review votes.
4. **Controlled State Machines:** Procedural triggers prevent illegal transitions in the reservation lifecycle (e.g. attempting to cancel or alter a completed dining record).
5. **Exact Financial Minor Units:** Menu prices are stored strictly as `BIGINT` representing ETB Santims ($1 \text{ ETB} = 100 \text{ Santims}$).
6. **Soft Deletion with Referential Stability:** Customer review deletions and venue unpublishing do not corrupt historical booking contracts or audit records.

---

## 2. Table Specifications

### 2.1 `users`
**Purpose:** Canonical identity for customers, business owners, and administrators.
- **Columns:**
  - `id` (`UUID`, PK, `DEFAULT gen_random_uuid()`): Unique user identifier.
  - `phone` (`VARCHAR(20)`, `NOT NULL`, `UNIQUE`): Canonical E.164 phone number (e.g., `+251911234567`).
  - `full_name` (`VARCHAR(100)`, `NOT NULL`): Customer or manager name.
  - `avatar_url` (`TEXT`): Profile image reference.
  - `role` (`VARCHAR(20)`, `NOT NULL`, `DEFAULT 'CUSTOMER'`): Platform role (`CUSTOMER`, `ADMIN`).
  - `is_active` (`BOOLEAN`, `NOT NULL`, `DEFAULT TRUE`): Account status flag.
  - `created_at` (`TIMESTAMPTZ`, `NOT NULL`, `DEFAULT NOW()`): Creation timestamp (UTC).
  - `updated_at` (`TIMESTAMPTZ`, `NOT NULL`, `DEFAULT NOW()`): Automatic update timestamp (UTC).
  - `deleted_at` (`TIMESTAMPTZ`): Soft delete timestamp.
- **Constraints:**
  - `unique_user_phone UNIQUE (phone)`
  - `check_user_role CHECK (role IN ('CUSTOMER', 'ADMIN'))`
- **Triggers:** `trg_users_updated_at` (maintains `updated_at`).

### 2.2 `sessions`
**Purpose:** Ephemeral authentication sessions and token storage.
- **Columns:**
  - `id` (`UUID`, PK, `DEFAULT gen_random_uuid()`)
  - `user_id` (`UUID`, `NOT NULL`, `REFERENCES users(id) ON DELETE CASCADE`)
  - `token_hash` (`VARCHAR(64)`, `NOT NULL`, `UNIQUE`): SHA-256 hash of session token.
  - `user_agent` (`TEXT`): Client browser / device details.
  - `ip_address` (`VARCHAR(45)`): IPv4 or IPv6 address.
  - `expires_at` (`TIMESTAMPTZ`, `NOT NULL`): Session expiration timestamp.
  - `created_at` (`TIMESTAMPTZ`, `NOT NULL`, `DEFAULT NOW()`)
- **Indexes:**
  - `idx_sessions_user_id (user_id)`
  - `idx_sessions_expires_at (expires_at)`

### 2.3 `businesses`
**Purpose:** Authoritative directory listing and platform administrative state.
- **Columns:**
  - `id` (`BIGINT`, PK, `GENERATED ALWAYS AS IDENTITY`)
  - `name` (`VARCHAR(150)`, `NOT NULL`): Venue name.
  - `slug` (`VARCHAR(160)`, `NOT NULL`, `UNIQUE`): URL-friendly unique identifier.
  - `category` (`VARCHAR(30)`, `NOT NULL`): `'restaurant'`, `'cafe'`, or `'drinks'`.
  - `cuisine` (`VARCHAR(80)`, `NOT NULL`): Specific dining style (e.g. `'Ethiopian'`, `'Italian'`).
  - `description` (`TEXT`, `NOT NULL`, `DEFAULT ''`): Long profile overview.
  - `short_description` (`VARCHAR(200)`, `NOT NULL`, `DEFAULT ''`): Snippet for cards.
  - `location_name` (`VARCHAR(100)`, `NOT NULL`): Landmark or area (e.g. `'Bole Medhanialem'`).
  - `neighborhood` (`VARCHAR(80)`, `NOT NULL`): Geographic search zone (e.g. `'Bole'`).
  - `address_line` (`TEXT`): Street address details.
  - `latitude` / `longitude` (`DECIMAL(10, 7)`): Geospatial coordinates.
  - `phone` (`VARCHAR(30)`, `NOT NULL`, `DEFAULT ''`): Venue contact phone.
  - `website` (`VARCHAR(255)`): Official website URL.
  - `price_range` (`VARCHAR(10)`, `NOT NULL`, `DEFAULT '$$'`): `'$'`, `'$$'`, `'$$$'`, `'$$$$'`.
  - `tags` (`TEXT[]`, `NOT NULL`, `DEFAULT '{}'`): Amenities and features.
  - `payment_methods` (`TEXT[]`, `NOT NULL`, `DEFAULT '{"Cash"}'`): Accepted payments.
  - `reservation_available` (`BOOLEAN`, `NOT NULL`, `DEFAULT FALSE`): Table booking flag.
  - `publication_status` (`VARCHAR(20)`, `NOT NULL`, `DEFAULT 'UNPUBLISHED'`): `'PUBLISHED'`, `'UNPUBLISHED'`.
  - `verification_status` (`VARCHAR(20)`, `NOT NULL`, `DEFAULT 'UNVERIFIED'`): `'VERIFIED'`, `'UNVERIFIED'`.
  - `claim_status` (`VARCHAR(20)`, `NOT NULL`, `DEFAULT 'UNCLAIMED'`): `'UNCLAIMED'`, `'CLAIMED'`.
  - `is_featured` (`BOOLEAN`, `NOT NULL`, `DEFAULT FALSE`): Editorial highlight.
  - `created_by_user_id` (`UUID`, `REFERENCES users(id) ON DELETE SET NULL`): Provenance.
  - `created_at` / `updated_at` (`TIMESTAMPTZ`, `NOT NULL`, `DEFAULT NOW()`)
  - `deleted_at` (`TIMESTAMPTZ`): Soft delete timestamp.
  - `search_vector` (`tsvector`, Generated Stored): Weighted multi-field full-text search document.
- **Constraints:**
  - `unique_business_slug UNIQUE (slug)`
  - `check_business_category CHECK (category IN ('restaurant', 'cafe', 'drinks'))`
  - `check_business_price_range CHECK (price_range IN ('$', '$$', '$$$', '$$$$'))`
  - `check_business_publication_status CHECK (publication_status IN ('PUBLISHED', 'UNPUBLISHED'))`
  - `check_business_verification_status CHECK (verification_status IN ('VERIFIED', 'UNVERIFIED'))`
  - `check_business_claim_status CHECK (claim_status IN ('UNCLAIMED', 'CLAIMED'))`
- **Indexes:**
  - `idx_businesses_search_vector ON businesses USING GIN (search_vector)`
  - `idx_businesses_name_trgm ON businesses USING GIN (name gin_trgm_ops)`
  - `idx_businesses_neighborhood_trgm ON businesses USING GIN (neighborhood gin_trgm_ops)`
  - `idx_businesses_publication ON businesses(publication_status) WHERE deleted_at IS NULL`
  - `idx_businesses_category ON businesses(category) WHERE publication_status = 'PUBLISHED' AND deleted_at IS NULL`
  - `idx_businesses_location ON businesses(location_name, neighborhood)`

### 2.4 `business_hours`
**Purpose:** Weekly structured opening and closing schedule.
- **Columns:**
  - `id` (`UUID`, PK, `DEFAULT gen_random_uuid()`)
  - `business_id` (`BIGINT`, `NOT NULL`, `REFERENCES businesses(id) ON DELETE CASCADE`)
  - `day_of_week` (`SMALLINT`, `NOT NULL`): 0=Sunday, 1=Monday, ..., 6=Saturday.
  - `opens_at` (`TIME`), `closes_at` (`TIME`)
  - `is_closed` (`BOOLEAN`, `NOT NULL`, `DEFAULT FALSE`)
- **Constraints:**
  - `unique_business_day UNIQUE (business_id, day_of_week)`
  - `check_business_hours_dow CHECK (day_of_week >= 0 AND day_of_week <= 6)`

### 2.5 `business_images`
**Purpose:** Listing photo gallery references and display ordering.
- **Columns:**
  - `id` (`UUID`, PK, `DEFAULT gen_random_uuid()`)
  - `business_id` (`BIGINT`, `NOT NULL`, `REFERENCES businesses(id) ON DELETE CASCADE`)
  - `image_url` (`TEXT`, `NOT NULL`): CDN / cloud storage URI.
  - `alt_text` (`VARCHAR(200)`): Accessibility text.
  - `sort_order` (`INT`, `NOT NULL`, `DEFAULT 0`): Explicit gallery display order.
  - `created_at` (`TIMESTAMPTZ`, `NOT NULL`, `DEFAULT NOW()`)
- **Indexes:**
  - `idx_business_images_business_order (business_id, sort_order)`

### 2.6 `business_managers`
**Purpose:** Multi-tenant ownership and management relationship.
- **Columns:**
  - `id` (`UUID`, PK, `DEFAULT gen_random_uuid()`)
  - `business_id` (`BIGINT`, `NOT NULL`, `REFERENCES businesses(id) ON DELETE CASCADE`)
  - `user_id` (`UUID`, `NOT NULL`, `REFERENCES users(id) ON DELETE CASCADE`)
  - `manager_role` (`VARCHAR(20)`, `NOT NULL`, `DEFAULT 'MANAGER'`): `'PRIMARY_OWNER'`, `'MANAGER'`.
  - `granted_by` (`UUID`, `REFERENCES users(id) ON DELETE SET NULL`)
  - `created_at` (`TIMESTAMPTZ`, `NOT NULL`, `DEFAULT NOW()`)
- **Constraints:**
  - `unique_business_manager UNIQUE (business_id, user_id)`
  - `check_manager_role CHECK (manager_role IN ('PRIMARY_OWNER', 'MANAGER'))`
- **Indexes:**
  - `idx_business_managers_user (user_id)`
  - `idx_business_managers_business (business_id)`

### 2.7 `business_claims`
**Purpose:** Administrative workflow for claiming management rights of an existing business.
- **Columns:**
  - `id` (`UUID`, PK, `DEFAULT gen_random_uuid()`)
  - `business_id` (`BIGINT`, `NOT NULL`, `REFERENCES businesses(id) ON DELETE RESTRICT`)
  - `claimant_user_id` (`UUID`, `NOT NULL`, `REFERENCES users(id) ON DELETE RESTRICT`)
  - `status` (`VARCHAR(20)`, `NOT NULL`, `DEFAULT 'PENDING'`): `'PENDING'`, `'APPROVED'`, `'REJECTED'`.
  - `submitted_at` (`TIMESTAMPTZ`, `NOT NULL`, `DEFAULT NOW()`)
  - `reviewed_at` (`TIMESTAMPTZ`), `reviewed_by` (`UUID`, `REFERENCES users(id) ON DELETE SET NULL`)
  - `rejection_reason` (`TEXT`)
  - `created_at` / `updated_at` (`TIMESTAMPTZ`, `NOT NULL`, `DEFAULT NOW()`)
- **Constraints & Partial Indexes:**
  - `check_claim_status CHECK (status IN ('PENDING', 'APPROVED', 'REJECTED'))`
  - `CREATE UNIQUE INDEX idx_unique_pending_business_claim ON business_claims(business_id) WHERE status = 'PENDING'` (Prevents conflicting active claims on the same business).

### 2.8 `business_submissions`
**Purpose:** Administrative workflow for community/owner submission of new venues.
- **Columns:**
  - `id` (`UUID`, PK, `DEFAULT gen_random_uuid()`)
  - `submitter_user_id` (`UUID`, `NOT NULL`, `REFERENCES users(id) ON DELETE RESTRICT`)
  - `business_name` (`VARCHAR(150)`, `NOT NULL`)
  - `category` (`VARCHAR(30)`, `NOT NULL`): `'restaurant'`, `'cafe'`, `'drinks'`.
  - `area` (`VARCHAR(100)`, `NOT NULL`)
  - `submitted_payload` (`JSONB`, `NOT NULL`, `DEFAULT '{}'`): Preserves original raw submission.
  - `status` (`VARCHAR(20)`, `NOT NULL`, `DEFAULT 'PENDING'`): `'PENDING'`, `'APPROVED'`, `'REJECTED'`.
  - `created_business_id` (`BIGINT`, `REFERENCES businesses(id) ON DELETE SET NULL`)
  - `submitted_at` (`TIMESTAMPTZ`, `NOT NULL`, `DEFAULT NOW()`)
  - `reviewed_at` (`TIMESTAMPTZ`), `reviewed_by` (`UUID`, `REFERENCES users(id) ON DELETE SET NULL`)
  - `rejection_reason` (`TEXT`)
  - `created_at` / `updated_at` (`TIMESTAMPTZ`, `NOT NULL`, `DEFAULT NOW()`)

### 2.9 `menu_categories`
**Purpose:** Owner-controlled menu section headers.
- **Columns:**
  - `id` (`UUID`, PK, `DEFAULT gen_random_uuid()`)
  - `business_id` (`BIGINT`, `NOT NULL`, `REFERENCES businesses(id) ON DELETE CASCADE`)
  - `name` (`VARCHAR(80)`, `NOT NULL`)
  - `sort_order` (`INT`, `NOT NULL`, `DEFAULT 0`): Manual display ordering.
  - `created_at` / `updated_at` (`TIMESTAMPTZ`, `NOT NULL`, `DEFAULT NOW()`)
  - `deleted_at` (`TIMESTAMPTZ`): Soft delete.
- **Constraints:**
  - `unique_business_category_name UNIQUE (business_id, name)`

### 2.10 `menu_items`
**Purpose:** Individual dishes and beverages with exact minor-unit pricing.
- **Columns:**
  - `id` (`UUID`, PK, `DEFAULT gen_random_uuid()`)
  - `category_id` (`UUID`, `NOT NULL`, `REFERENCES menu_categories(id) ON DELETE CASCADE`)
  - `business_id` (`BIGINT`, `NOT NULL`, `REFERENCES businesses(id) ON DELETE CASCADE`)
  - `name` (`VARCHAR(100)`, `NOT NULL`)
  - `description` (`TEXT`, `NOT NULL`, `DEFAULT ''`)
  - `price_etb_minor` (`BIGINT`, `NOT NULL`): Price in Santims (e.g. `25000` = 250 ETB).
  - `sort_order` (`INT`, `NOT NULL`, `DEFAULT 0`): Manual display ordering.
  - `is_available` (`BOOLEAN`, `NOT NULL`, `DEFAULT TRUE`)
  - `created_at` / `updated_at` (`TIMESTAMPTZ`, `NOT NULL`, `DEFAULT NOW()`)
  - `deleted_at` (`TIMESTAMPTZ`): Soft delete.
- **Constraints:**
  - `check_menu_item_price CHECK (price_etb_minor >= 0)`
- **Indexes:**
  - `idx_menu_items_name_trgm ON menu_items USING GIN (name gin_trgm_ops)`

### 2.11 `business_updates`
**Purpose:** Time-sensitive broadcast posts by venue managers.
- **Columns:**
  - `id` (`UUID`, PK, `DEFAULT gen_random_uuid()`)
  - `business_id` (`BIGINT`, `NOT NULL`, `REFERENCES businesses(id) ON DELETE CASCADE`)
  - `author_user_id` (`UUID`, `NOT NULL`, `REFERENCES users(id) ON DELETE RESTRICT`)
  - `update_type` (`VARCHAR(30)`, `NOT NULL`): `'new_menu'`, `'offer'`, `'event'`, `'price_update'`, `'announcement'`.
  - `title` (`VARCHAR(120)`, `NOT NULL`)
  - `description` (`TEXT`, `NOT NULL`)
  - `status` (`VARCHAR(20)`, `NOT NULL`, `DEFAULT 'PUBLISHED'`): `'DRAFT'`, `'PUBLISHED'`, `'EXPIRED'`.
  - `event_date` (`VARCHAR(50)`), `event_time` (`VARCHAR(30)`), `price_text` (`VARCHAR(50)`), `offer_details` (`VARCHAR(100)`), `valid_until` (`VARCHAR(50)`)
  - `expires_at` (`TIMESTAMPTZ`): Automatic expiration timestamp.
  - `published_at` (`TIMESTAMPTZ`, `NOT NULL`, `DEFAULT NOW()`)
  - `created_at` / `updated_at` (`TIMESTAMPTZ`, `NOT NULL`, `DEFAULT NOW()`)
  - `deleted_at` (`TIMESTAMPTZ`)
- **Constraints:**
  - `check_update_type CHECK (update_type IN ('new_menu', 'offer', 'event', 'price_update', 'announcement'))`
  - `check_update_status CHECK (status IN ('DRAFT', 'PUBLISHED', 'EXPIRED'))`

### 2.12 `reservations`
**Purpose:** Table booking contract between customer and restaurant.
- **Columns:**
  - `id` (`UUID`, PK, `DEFAULT gen_random_uuid()`)
  - `reservation_code` (`VARCHAR(20)`, `NOT NULL`, `UNIQUE`): Reference code (e.g. `'CAF-1001'`).
  - `business_id` (`BIGINT`, `NOT NULL`, `REFERENCES businesses(id) ON DELETE RESTRICT`)
  - `customer_user_id` (`UUID`, `NOT NULL`, `REFERENCES users(id) ON DELETE RESTRICT`)
  - `customer_name` (`VARCHAR(100)`, `NOT NULL`), `customer_phone` (`VARCHAR(30)`, `NOT NULL`)
  - `reservation_date` (`DATE`, `NOT NULL`), `reservation_time` (`TIME`, `NOT NULL`)
  - `party_size` (`INT`, `NOT NULL`)
  - `special_request` (`TEXT`, `NOT NULL`, `DEFAULT ''`)
  - `status` (`VARCHAR(20)`, `NOT NULL`, `DEFAULT 'PENDING'`): `'PENDING'`, `'CONFIRMED'`, `'DECLINED'`, `'CANCELLED'`, `'COMPLETED'`.
  - `decline_reason` (`TEXT`): Mandatory when declining.
  - `confirmed_at`, `declined_at`, `completed_at`, `cancelled_at` (`TIMESTAMPTZ`)
  - `reviewed_by_user` (`UUID`, `REFERENCES users(id) ON DELETE SET NULL`)
  - `created_at` / `updated_at` (`TIMESTAMPTZ`, `NOT NULL`, `DEFAULT NOW()`)
- **Constraints:**
  - `unique_reservation_code UNIQUE (reservation_code)`
  - `check_reservation_party_size CHECK (party_size > 0 AND party_size <= 50)`
  - `check_reservation_status CHECK (status IN ('PENDING', 'CONFIRMED', 'DECLINED', 'CANCELLED', 'COMPLETED'))`
- **Triggers:**
  - `trg_reservations_status_guard`: Enforces legal state transitions and decline reason requirements.

### 2.13 `reviews`
**Purpose:** Customer rating and written feedback verified by completed visit.
- **Columns:**
  - `id` (`UUID`, PK, `DEFAULT gen_random_uuid()`)
  - `business_id` (`BIGINT`, `NOT NULL`, `REFERENCES businesses(id) ON DELETE CASCADE`)
  - `customer_user_id` (`UUID`, `NOT NULL`, `REFERENCES users(id) ON DELETE RESTRICT`)
  - `reservation_id` (`UUID`, `NOT NULL`, `UNIQUE`, `REFERENCES reservations(id) ON DELETE RESTRICT`)
  - `rating` (`SMALLINT`, `NOT NULL`): Star rating (1 to 5).
  - `review_text` (`TEXT`, `NOT NULL`, `DEFAULT ''`)
  - `is_verified_visit` (`BOOLEAN`, `NOT NULL`, `DEFAULT TRUE`)
  - `status` (`VARCHAR(20)`, `NOT NULL`, `DEFAULT 'VISIBLE'`): `'VISIBLE'`, `'REMOVED_BY_ADMIN'`.
  - `created_at` / `updated_at` (`TIMESTAMPTZ`, `NOT NULL`, `DEFAULT NOW()`)
  - `deleted_at` (`TIMESTAMPTZ`): Customer soft delete.
- **Constraints:**
  - `unique_reservation_review UNIQUE (reservation_id)`
  - `check_review_rating CHECK (rating >= 1 AND rating <= 5)`
  - `check_review_status CHECK (status IN ('VISIBLE', 'REMOVED_BY_ADMIN'))`
- **Triggers:**
  - `trg_reviews_reservation_completed`: Verifies that `reservation_id` references a completed reservation before allowing insert or update.

### 2.14 `review_helpful_votes`
**Purpose:** Upvotes on customer reviews.
- **Columns:**
  - `review_id` (`UUID`, `NOT NULL`, `REFERENCES reviews(id) ON DELETE CASCADE`)
  - `customer_user_id` (`UUID`, `NOT NULL`, `REFERENCES users(id) ON DELETE CASCADE`)
  - `created_at` (`TIMESTAMPTZ`, `NOT NULL`, `DEFAULT NOW()`)
- **Constraints:**
  - `PRIMARY KEY (review_id, customer_user_id)`

### 2.15 `review_reports`
**Purpose:** Community abuse flagging on reviews.
- **Columns:**
  - `id` (`UUID`, PK, `DEFAULT gen_random_uuid()`)
  - `review_id` (`UUID`, `NOT NULL`, `REFERENCES reviews(id) ON DELETE CASCADE`)
  - `reporter_user_id` (`UUID`, `NOT NULL`, `REFERENCES users(id) ON DELETE RESTRICT`)
  - `reason` (`VARCHAR(30)`, `NOT NULL`): `'spam'`, `'offensive'`, `'fake'`, `'other'`.
  - `explanation` (`TEXT`, `NOT NULL`, `DEFAULT ''`)
  - `status` (`VARCHAR(20)`, `NOT NULL`, `DEFAULT 'PENDING'`): `'PENDING'`, `'DISMISSED'`, `'ACTIONED'`.
  - `reviewed_at` (`TIMESTAMPTZ`), `reviewed_by` (`UUID`, `REFERENCES users(id) ON DELETE SET NULL`)
  - `admin_action` (`VARCHAR(20)`): `'dismissed'`, `'removed'`.
  - `created_at` / `updated_at` (`TIMESTAMPTZ`, `NOT NULL`, `DEFAULT NOW()`)
- **Constraints:**
  - `unique_reporter_review UNIQUE (review_id, reporter_user_id)`
  - `check_review_report_reason CHECK (reason IN ('spam', 'offensive', 'fake', 'other'))`
  - `check_review_report_status CHECK (status IN ('PENDING', 'DISMISSED', 'ACTIONED'))`

### 2.16 `business_reports`
**Purpose:** Community flagging of inaccurate venue information.
- **Columns:**
  - `id` (`UUID`, PK, `DEFAULT gen_random_uuid()`)
  - `business_id` (`BIGINT`, `NOT NULL`, `REFERENCES businesses(id) ON DELETE CASCADE`)
  - `reporter_user_id` (`UUID`, `NOT NULL`, `REFERENCES users(id) ON DELETE RESTRICT`)
  - `reason` (`VARCHAR(30)`, `NOT NULL`): `'WRONG_HOURS'`, `'WRONG_PHONE'`, `'WRONG_LOCATION'`, `'WRONG_INFORMATION'`, `'CLOSED'`, `'DUPLICATE'`, `'OTHER'`.
  - `details` (`TEXT`)
  - `status` (`VARCHAR(20)`, `NOT NULL`, `DEFAULT 'PENDING'`): `'PENDING'`, `'DISMISSED'`, `'ACTIONED'`.
  - `reviewed_at` (`TIMESTAMPTZ`), `reviewed_by` (`UUID`, `REFERENCES users(id) ON DELETE SET NULL`)
  - `admin_notes` (`TEXT`)
  - `created_at` / `updated_at` (`TIMESTAMPTZ`, `NOT NULL`, `DEFAULT NOW()`)
- **Constraints:**
  - `check_business_report_reason CHECK (reason IN ('WRONG_HOURS', 'WRONG_PHONE', 'WRONG_LOCATION', 'WRONG_INFORMATION', 'CLOSED', 'DUPLICATE', 'OTHER'))`
  - `check_business_report_status CHECK (status IN ('PENDING', 'DISMISSED', 'ACTIONED'))`

### 2.17 `saved_businesses` & `followed_businesses`
**Purpose:** Customer bookmarks and update subscriptions.
- **Columns:**
  - `user_id` (`UUID`, `NOT NULL`, `REFERENCES users(id) ON DELETE CASCADE`)
  - `business_id` (`BIGINT`, `NOT NULL`, `REFERENCES businesses(id) ON DELETE CASCADE`)
  - `created_at` (`TIMESTAMPTZ`, `NOT NULL`, `DEFAULT NOW()`)
- **Constraints:**
  - `PRIMARY KEY (user_id, business_id)`

### 2.18 `admin_actions`
**Purpose:** Append-only audit trail for administrative accountability.
- **Columns:**
  - `id` (`UUID`, PK, `DEFAULT gen_random_uuid()`)
  - `admin_user_id` (`UUID`, `NOT NULL`, `REFERENCES users(id) ON DELETE RESTRICT`)
  - `action_type` (`VARCHAR(50)`, `NOT NULL`)
  - `target_entity_type` (`VARCHAR(30)`, `NOT NULL`)
  - `target_entity_id` (`VARCHAR(64)`, `NOT NULL`)
  - `metadata` (`JSONB`, `NOT NULL`, `DEFAULT '{}'`)
  - `created_at` (`TIMESTAMPTZ`, `NOT NULL`, `DEFAULT NOW()`)
