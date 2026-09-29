# CAFNET BACKEND DOMAIN MODEL SPECIFICATION
**Document ID:** `CAFNET-BACKEND-01-DOMAIN-MODEL`  
**Status:** Canonical Architecture Specification  
**Target Stack:** Go 1.23+, Neon PostgreSQL, Go-Native Auth  

---

## 1. Domain Philosophy & Architecture Overview

Cafnet is a hospitality and dining discovery, reservation, and management platform built specifically for cafes, restaurants, and lounges in Ethiopia (starting with Addis Ababa). 

The backend architecture translates the user-facing Next.js frontend into a clean, modular domain model. It avoids premature microservices while maintaining strict domain boundaries that can be expressed as idiomatic Go packages and enforced via PostgreSQL schema constraints.

### Core Architecture Principles

1. **Explicit Boundaries Over Generic Models:** Avoid single "status" columns or polymorphic tables that conflate business publication, business existence, administrative verification, and ownership claim status.
2. **Authoritative vs. Derived Separation:** Store only authoritative operational facts. Aggregates like average ratings, open/closed flags, and review distributions are strictly derived or transactionally maintained read models.
3. **Multi-Tenant Ownership Ready:** A user can own or manage multiple businesses, and a business can have multiple managers. Ownership is never modeled as a single `owner_id` on the business record.
4. **Verified Trust Pipeline:** A customer review can only be submitted for a completed dining visit (`Reservation.status = 'COMPLETED'`). Community moderation guarantees that platform listings and reviews remain trustworthy.
5. **No Floating-Point Money:** All financial representations (prices on menus, future billings) are stored as integer minor units (ETB cents/santims).

---

## 2. Domain Boundaries

The Cafnet platform is divided into **10 canonical domain boundaries**:

```
+-----------------------------------------------------------------------------------+
|                                     CAFNET PLATFORM                               |
+-----------------------------------------------------------------------------------+
|  [IDENTITY]             [DISCOVERY]           [BUSINESS]     [BUSINESS OWNERSHIP] |
|   Users, Sessions,       Search, Explore,      Directory,     Claims, Managers,   |
|   Roles, Tokens          Saves, Follows        Profile Data   Permissions         |
+-----------------------------------------------------------------------------------+
|  [MENU]                 [BUSINESS UPDATES]    [RESERVATIONS] [REVIEWS]            |
|   Categories, Items,     Offers, Events,       Booking flow,  Ratings, Feedback,  |
|   ETB Prices, Ordering   Announcements         Visit tracking Helpful votes       |
+-----------------------------------------------------------------------------------+
|  [MODERATION & REPORTING]                     [ADMINISTRATION]                    |
|   Community flags (Reviews & Businesses)       Governance, Approvals, Audit Logs  |
+-----------------------------------------------------------------------------------+
```

### 2.1 IDENTITY
- **Responsibility:** User registration, phone-based authentication (OTP), session lifecycle, credentials, and platform security roles (`CUSTOMER`, `ADMIN`).
- **Core Entities:** `User`, `Session`, `UserRole`.
- **Boundary Invariant:** The identity domain knows *who* a user is and what *platform* role they hold, but delegates business-specific authorization to the `BUSINESS OWNERSHIP` domain.

### 2.2 DISCOVERY
- **Responsibility:** Search, category browsing, geolocation/neighborhood distance calculation, popular tag filtering, and personalized customer collections.
- **Core Entities / Relations:** `SavedBusiness`, `FollowedBusiness`, Search indexes / query filters.
- **Boundary Invariant:** Discovery only returns businesses that are active and published. Discovery views are read-only projections.

### 2.3 BUSINESS
- **Responsibility:** Canonical restaurant identity, location, profile metadata, operating hours, amenities/tags, payment methods, contact details, photos, and public listing state.
- **Core Entities:** `Business`, `BusinessHours`, `BusinessImage`.
- **Boundary Invariant:** Modifying public business profile data requires verified management authorization or administrative governance.

### 2.4 BUSINESS OWNERSHIP
- **Responsibility:** Management access control, ownership verification workflows, claim submissions, and multi-manager relationships.
- **Core Entities:** `BusinessManager`, `BusinessClaim`, `BusinessSubmission`.
- **Boundary Invariant:** A user cannot manage a business listing until an administrative claim or submission approval has explicitly created an active `BusinessManager` relationship.

### 2.5 MENU
- **Responsibility:** Menu catalog management, categorized menu structures, ETB pricing, descriptions, and explicit owner-defined display ordering.
- **Core Entities:** `MenuCategory`, `MenuItem`.
- **Boundary Invariant:** Every item belongs to a category; display order is explicitly controlled by managers via sort order integers, never assumed to be alphabetical.

### 2.6 BUSINESS UPDATES
- **Responsibility:** Time-sensitive owner broadcasts to customers, including new menu items, special offers, live events, price updates, and general announcements.
- **Core Entities:** `BusinessUpdate`.
- **Boundary Invariant:** Updates can only be posted by verified managers of a published business. Expired updates automatically drop from public feeds.

### 2.7 RESERVATIONS
- **Responsibility:** Table booking lifecycle, customer booking requests, party size, reservation status progression, cancellation rules, and visit completion tracking.
- **Core Entities:** `Reservation`.
- **Boundary Invariant:** Terminal reservation states (`COMPLETED`, `DECLINED`, `CANCELLED`) cannot be modified. Only `CONFIRMED` reservations can be marked `COMPLETED`.

### 2.8 REVIEWS
- **Responsibility:** Customer rating submissions, verified dining feedback, review edits, customer deletions, and community helpfulness votes.
- **Core Entities:** `Review`, `ReviewHelpfulVote`.
- **Boundary Invariant:** A review requires a completed reservation (`Reservation.status = 'COMPLETED'`). Exactly one review is permitted per reservation.

### 2.9 MODERATION & REPORTING
- **Responsibility:** Quality control and anti-abuse mechanisms. Manages customer reports on misleading reviews and inaccurate business profile information.
- **Core Entities:** `ReviewReport`, `BusinessReport`, `ReviewModerationState`.
- **Boundary Invariant:** Customer reports never automatically alter or remove public data; they enter a triage queue requiring administrative review.

### 2.10 ADMINISTRATION
- **Responsibility:** Platform oversight, new business submission approval/rejection, claim approval/rejection, review takedowns, and immutable audit logs.
- **Core Entities:** `AdminAction` (audit log).
- **Boundary Invariant:** All destructive or access-granting administrative decisions record an immutable audit entry identifying the administrator, target entity, timestamp, and rationale.

---

## 3. Canonical Core Entities

| Entity Name | Domain | Storage Classification | Primary Responsibility |
| :--- | :--- | :--- | :--- |
| **`User`** | Identity | Persistent Entity | Single canonical identity for customers, owners, and administrators. |
| **`Session`** | Identity | Authentication Data | Active authentication session, token hash, device/IP, expiry. |
| **`Business`** | Business | Persistent Entity | Authoritative root for a venue's public listing and administrative state. |
| **`BusinessHours`** | Business | Persistent Child Entity | Weekly structured opening and closing schedules per day. |
| **`BusinessImage`** | Business | Persistent Child Entity | Media references and display ordering for listing galleries. |
| **`BusinessManager`** | Business Ownership | Persistent Relationship | Explicit M:N mapping granting a user management rights over a business. |
| **`BusinessClaim`** | Business Ownership | Persistent Workflow Entity | Formal petition by a user to claim management rights for an existing business. |
| **`BusinessSubmission`** | Business Ownership | Persistent Workflow Entity | Formal petition by a user to register and publish a new business on Cafnet. |
| **`MenuCategory`** | Menu | Persistent Entity | Grouping header for menu items with owner-defined sort order. |
| **`MenuItem`** | Menu | Persistent Entity | Food or beverage offering with name, description, ETB price, and order. |
| **`BusinessUpdate`** | Business Updates | Persistent Entity | Broadcast post (offer, event, new menu) with scheduling and expiry. |
| **`Reservation`** | Reservations | Persistent Entity | Table booking contract between customer and business with visit tracking. |
| **`Review`** | Reviews | Persistent Entity | 1-5 star rating and feedback tied to a verified reservation visit. |
| **`ReviewHelpfulVote`** | Reviews | Persistent Relationship | Unique upvote mapping between a customer and a review. |
| **`ReviewReport`** | Moderation | Persistent Workflow Entity | Flag raised against a review for spam, offensive content, or dishonesty. |
| **`BusinessReport`** | Moderation | Persistent Workflow Entity | Flag raised against inaccurate business details (hours, phone, closed). |
| **`SavedBusiness`** | Discovery | Persistent Relationship | User bookmark for a business. |
| **`FollowedBusiness`** | Discovery | Persistent Relationship | User subscription to updates from a business. |
| **`AdminAction`** | Administration | Audit Record (Append-only) | Immutable log of administrative decisions (approvals, dismissals, bans). |

---

## 4. Entity Attribute Specifications

### 4.1 `User`
```
User {
  id:                 UUID [PK]
  phone:              VARCHAR(20) [UNIQUE, NOT NULL]  -- Canonical E.164 (e.g., +251911234567)
  full_name:          VARCHAR(100) [NOT NULL]
  avatar_url:         TEXT [NULLABLE]
  role:               VARCHAR(20) [NOT NULL, DEFAULT 'CUSTOMER'] -- CUSTOMER, ADMIN
  is_active:          BOOLEAN [NOT NULL, DEFAULT TRUE]
  created_at:         TIMESTAMPTZ [NOT NULL, DEFAULT NOW()]
  updated_at:         TIMESTAMPTZ [NOT NULL, DEFAULT NOW()]
  deleted_at:         TIMESTAMPTZ [NULLABLE]
}
```

### 4.2 `Business`
```
Business {
  -- Public Data
  id:                     BIGINT [PK, GENERATED ALWAYS AS IDENTITY]
  name:                   VARCHAR(150) [NOT NULL]
  slug:                   VARCHAR(160) [UNIQUE, NOT NULL]
  category:               VARCHAR(30) [NOT NULL] -- 'restaurant', 'cafe', 'drinks'
  cuisine:                VARCHAR(80) [NOT NULL] -- e.g., 'Ethiopian', 'Coffee & Bakery', 'Italian'
  description:            TEXT [NOT NULL]
  short_description:      VARCHAR(200) [NOT NULL]
  location_name:          VARCHAR(100) [NOT NULL] -- Neighborhood/area (e.g., 'Bole Medhanialem')
  neighborhood:           VARCHAR(80) [NOT NULL]  -- Searchable zone (e.g., 'Bole')
  address_line:           TEXT [NULLABLE]
  latitude:               DECIMAL(10, 7) [NULLABLE]
  longitude:              DECIMAL(10, 7) [NULLABLE]
  phone:                  VARCHAR(30) [NOT NULL]
  website:                VARCHAR(255) [NULLABLE]
  price_range:            VARCHAR(10) [NOT NULL] -- '$', '$$', '$$$', '$$$$'
  tags:                   TEXT[] [NOT NULL, DEFAULT '{}']
  payment_methods:        TEXT[] [NOT NULL, DEFAULT '{"Cash"}']
  reservation_available:  BOOLEAN [NOT NULL, DEFAULT FALSE]
  
  -- Administrative State (Separated)
  publication_status:     VARCHAR(20) [NOT NULL, DEFAULT 'UNPUBLISHED'] -- 'PUBLISHED', 'UNPUBLISHED'
  verification_status:    VARCHAR(20) [NOT NULL, DEFAULT 'UNVERIFIED']  -- 'VERIFIED', 'UNVERIFIED'
  claim_status:           VARCHAR(20) [NOT NULL, DEFAULT 'UNCLAIMED']   -- 'UNCLAIMED', 'CLAIMED'
  is_featured:            BOOLEAN [NOT NULL, DEFAULT FALSE]
  
  created_by_user_id:     UUID [NULLABLE, FK -> User.id]
  created_at:             TIMESTAMPTZ [NOT NULL, DEFAULT NOW()]
  updated_at:             TIMESTAMPTZ [NOT NULL, DEFAULT NOW()]
  deleted_at:             TIMESTAMPTZ [NULLABLE]
}
```

### 4.3 `BusinessHours`
```
BusinessHours {
  id:           UUID [PK]
  business_id:  BIGINT [NOT NULL, FK -> Business.id ON DELETE CASCADE]
  day_of_week:  SMALLINT [NOT NULL] -- 0=Sunday, 1=Monday, ..., 6=Saturday
  opens_at:     TIME [NULLABLE]      -- e.g. 08:00:00
  closes_at:    TIME [NULLABLE]      -- e.g. 22:00:00
  is_closed:    BOOLEAN [NOT NULL, DEFAULT FALSE]
  
  CONSTRAINT unique_business_day UNIQUE (business_id, day_of_week)
}
```

### 4.4 `BusinessManager` (Ownership Relationship)
```
BusinessManager {
  id:           UUID [PK]
  business_id:  BIGINT [NOT NULL, FK -> Business.id ON DELETE CASCADE]
  user_id:      UUID [NOT NULL, FK -> User.id ON DELETE CASCADE]
  manager_role: VARCHAR(20) [NOT NULL, DEFAULT 'MANAGER'] -- 'PRIMARY_OWNER', 'MANAGER'
  granted_by:   UUID [NULLABLE, FK -> User.id] -- Admin or Primary Owner
  created_at:   TIMESTAMPTZ [NOT NULL, DEFAULT NOW()]
  
  CONSTRAINT unique_business_manager UNIQUE (business_id, user_id)
}
```

### 4.5 `BusinessClaim`
```
BusinessClaim {
  id:                 UUID [PK]
  business_id:        BIGINT [NOT NULL, FK -> Business.id]
  claimant_user_id:   UUID [NOT NULL, FK -> User.id]
  status:             VARCHAR(20) [NOT NULL, DEFAULT 'PENDING'] -- 'PENDING', 'APPROVED', 'REJECTED'
  submitted_at:       TIMESTAMPTZ [NOT NULL, DEFAULT NOW()]
  reviewed_at:        TIMESTAMPTZ [NULLABLE]
  reviewed_by:        UUID [NULLABLE, FK -> User.id]
  rejection_reason:   TEXT [NULLABLE]
  created_at:         TIMESTAMPTZ [NOT NULL, DEFAULT NOW()]
  updated_at:         TIMESTAMPTZ [NOT NULL, DEFAULT NOW()]
}
```

### 4.6 `BusinessSubmission`
```
BusinessSubmission {
  id:                     UUID [PK]
  submitter_user_id:      UUID [NOT NULL, FK -> User.id]
  business_name:          VARCHAR(150) [NOT NULL]
  category:               VARCHAR(30) [NOT NULL] -- 'restaurant', 'cafe', 'drinks'
  area:                   VARCHAR(100) [NOT NULL]
  submitted_payload:      JSONB [NOT NULL, DEFAULT '{}'] -- Preserves original draft data
  status:                 VARCHAR(20) [NOT NULL, DEFAULT 'PENDING'] -- 'PENDING', 'APPROVED', 'REJECTED'
  created_business_id:    BIGINT [NULLABLE, FK -> Business.id] -- Populated upon approval
  submitted_at:           TIMESTAMPTZ [NOT NULL, DEFAULT NOW()]
  reviewed_at:            TIMESTAMPTZ [NULLABLE]
  reviewed_by:            UUID [NULLABLE, FK -> User.id]
  rejection_reason:       TEXT [NULLABLE]
}
```

### 4.7 `MenuCategory` & `MenuItem`
```
MenuCategory {
  id:           UUID [PK]
  business_id:  BIGINT [NOT NULL, FK -> Business.id ON DELETE CASCADE]
  name:         VARCHAR(80) [NOT NULL]
  sort_order:   INT [NOT NULL, DEFAULT 0]
  created_at:   TIMESTAMPTZ [NOT NULL, DEFAULT NOW()]
  updated_at:   TIMESTAMPTZ [NOT NULL, DEFAULT NOW()]
  deleted_at:   TIMESTAMPTZ [NULLABLE]
  
  CONSTRAINT unique_business_category_name UNIQUE (business_id, name)
}

MenuItem {
  id:               UUID [PK]
  category_id:      UUID [NOT NULL, FK -> MenuCategory.id ON DELETE CASCADE]
  business_id:      BIGINT [NOT NULL, FK -> Business.id ON DELETE CASCADE]
  name:             VARCHAR(100) [NOT NULL]
  description:      TEXT [NOT NULL, DEFAULT '']
  price_etb_minor:  BIGINT [NOT NULL] -- ETB Minor units (1 ETB = 100 units). E.g. 250 ETB = 25000
  sort_order:       INT [NOT NULL, DEFAULT 0]
  is_available:     BOOLEAN [NOT NULL, DEFAULT TRUE]
  created_at:       TIMESTAMPTZ [NOT NULL, DEFAULT NOW()]
  updated_at:       TIMESTAMPTZ [NOT NULL, DEFAULT NOW()]
  deleted_at:       TIMESTAMPTZ [NULLABLE]
}
```

### 4.8 `BusinessUpdate`
```
BusinessUpdate {
  id:             UUID [PK]
  business_id:    BIGINT [NOT NULL, FK -> Business.id ON DELETE CASCADE]
  author_user_id: UUID [NOT NULL, FK -> User.id]
  update_type:    VARCHAR(30) [NOT NULL] -- 'new_menu', 'offer', 'event', 'price_update', 'announcement'
  title:          VARCHAR(120) [NOT NULL]
  description:    TEXT [NOT NULL]
  status:         VARCHAR(20) [NOT NULL, DEFAULT 'PUBLISHED'] -- 'DRAFT', 'PUBLISHED', 'EXPIRED'
  event_date:     VARCHAR(50) [NULLABLE]
  event_time:     VARCHAR(30) [NULLABLE]
  price_text:     VARCHAR(50) [NULLABLE]
  offer_details:  VARCHAR(100) [NULLABLE]
  valid_until:    VARCHAR(50) [NULLABLE]
  expires_at:     TIMESTAMPTZ [NULLABLE]
  published_at:   TIMESTAMPTZ [NOT NULL, DEFAULT NOW()]
  created_at:     TIMESTAMPTZ [NOT NULL, DEFAULT NOW()]
  updated_at:     TIMESTAMPTZ [NOT NULL, DEFAULT NOW()]
  deleted_at:     TIMESTAMPTZ [NULLABLE]
}
```

### 4.9 `Reservation`
```
Reservation {
  id:                 UUID [PK]
  reservation_code:   VARCHAR(20) [UNIQUE, NOT NULL] -- Customer reference (e.g., 'CAF-1001')
  business_id:        BIGINT [NOT NULL, FK -> Business.id]
  customer_user_id:   UUID [NOT NULL, FK -> User.id]
  customer_name:      VARCHAR(100) [NOT NULL]
  customer_phone:     VARCHAR(30) [NOT NULL]
  reservation_date:   DATE [NOT NULL]
  reservation_time:   TIME [NOT NULL]
  party_size:         INT [NOT NULL] -- CHECK (party_size > 0 AND party_size <= 50)
  special_request:    TEXT [NOT NULL, DEFAULT '']
  status:             VARCHAR(20) [NOT NULL, DEFAULT 'PENDING'] -- PENDING, CONFIRMED, DECLINED, CANCELLED, COMPLETED
  decline_reason:     TEXT [NULLABLE]
  created_at:         TIMESTAMPTZ [NOT NULL, DEFAULT NOW()]
  confirmed_at:       TIMESTAMPTZ [NULLABLE]
  declined_at:        TIMESTAMPTZ [NULLABLE]
  completed_at:       TIMESTAMPTZ [NULLABLE]
  cancelled_at:       TIMESTAMPTZ [NULLABLE]
  reviewed_by_user:   UUID [NULLABLE, FK -> User.id] -- Manager who confirmed/declined/completed
  updated_at:         TIMESTAMPTZ [NOT NULL, DEFAULT NOW()]
}
```

### 4.10 `Review` & `ReviewHelpfulVote`
```
Review {
  id:                 UUID [PK]
  business_id:        BIGINT [NOT NULL, FK -> Business.id ON DELETE CASCADE]
  customer_user_id:   UUID [NOT NULL, FK -> User.id]
  reservation_id:     UUID [UNIQUE, NOT NULL, FK -> Reservation.id] -- Invariant: 1 review per reservation
  rating:             SMALLINT [NOT NULL] -- CHECK (rating >= 1 AND rating <= 5)
  review_text:        TEXT [NOT NULL, DEFAULT ''] -- Max 1000 characters
  is_verified_visit:  BOOLEAN [NOT NULL, DEFAULT TRUE] -- Set true via completed reservation constraint
  status:             VARCHAR(20) [NOT NULL, DEFAULT 'VISIBLE'] -- 'VISIBLE', 'REMOVED_BY_ADMIN'
  created_at:         TIMESTAMPTZ [NOT NULL, DEFAULT NOW()]
  updated_at:         TIMESTAMPTZ [NOT NULL, DEFAULT NOW()]
  deleted_at:         TIMESTAMPTZ [NULLABLE] -- Customer soft delete
}

ReviewHelpfulVote {
  review_id:          UUID [NOT NULL, FK -> Review.id ON DELETE CASCADE]
  customer_user_id:   UUID [NOT NULL, FK -> User.id ON DELETE CASCADE]
  created_at:         TIMESTAMPTZ [NOT NULL, DEFAULT NOW()]
  
  PRIMARY KEY (review_id, customer_user_id)
}
```

### 4.11 `ReviewReport` & `BusinessReport`
```
ReviewReport {
  id:                 UUID [PK]
  review_id:          UUID [NOT NULL, FK -> Review.id ON DELETE CASCADE]
  reporter_user_id:   UUID [NOT NULL, FK -> User.id]
  reason:             VARCHAR(30) [NOT NULL] -- 'spam', 'offensive', 'fake', 'other'
  explanation:        TEXT [NOT NULL, DEFAULT '']
  status:             VARCHAR(20) [NOT NULL, DEFAULT 'PENDING'] -- 'PENDING', 'DISMISSED', 'ACTIONED'
  reviewed_at:        TIMESTAMPTZ [NULLABLE]
  reviewed_by:        UUID [NULLABLE, FK -> User.id]
  admin_action:       VARCHAR(20) [NULLABLE] -- 'dismissed', 'removed'
  created_at:         TIMESTAMPTZ [NOT NULL, DEFAULT NOW()]
  updated_at:         TIMESTAMPTZ [NOT NULL, DEFAULT NOW()]
  
  CONSTRAINT unique_reporter_review UNIQUE (review_id, reporter_user_id)
}

BusinessReport {
  id:                 UUID [PK]
  business_id:        BIGINT [NOT NULL, FK -> Business.id ON DELETE CASCADE]
  reporter_user_id:   UUID [NOT NULL, FK -> User.id]
  reason:             VARCHAR(30) [NOT NULL] -- 'WRONG_HOURS', 'WRONG_PHONE', 'WRONG_LOCATION', 'WRONG_INFORMATION', 'CLOSED', 'DUPLICATE', 'OTHER'
  details:            TEXT [NULLABLE]
  status:             VARCHAR(20) [NOT NULL, DEFAULT 'PENDING'] -- 'PENDING', 'DISMISSED', 'ACTIONED'
  reviewed_at:        TIMESTAMPTZ [NULLABLE]
  reviewed_by:        UUID [NULLABLE, FK -> User.id]
  admin_notes:        TEXT [NULLABLE]
  created_at:         TIMESTAMPTZ [NOT NULL, DEFAULT NOW()]
  updated_at:         TIMESTAMPTZ [NOT NULL, DEFAULT NOW()]
}
```

### 4.12 `SavedBusiness` & `FollowedBusiness`
```
SavedBusiness {
  user_id:            UUID [NOT NULL, FK -> User.id ON DELETE CASCADE]
  business_id:        BIGINT [NOT NULL, FK -> Business.id ON DELETE CASCADE]
  created_at:         TIMESTAMPTZ [NOT NULL, DEFAULT NOW()]
  
  PRIMARY KEY (user_id, business_id)
}

FollowedBusiness {
  user_id:            UUID [NOT NULL, FK -> User.id ON DELETE CASCADE]
  business_id:        BIGINT [NOT NULL, FK -> Business.id ON DELETE CASCADE]
  created_at:         TIMESTAMPTZ [NOT NULL, DEFAULT NOW()]
  
  PRIMARY KEY (user_id, business_id)
}
```

### 4.13 `AdminAction` (Audit Trail)
```
AdminAction {
  id:                 UUID [PK]
  admin_user_id:      UUID [NOT NULL, FK -> User.id]
  action_type:        VARCHAR(50) [NOT NULL] -- 'APPROVE_SUBMISSION', 'REJECT_SUBMISSION', 'APPROVE_CLAIM', 'REJECT_CLAIM', 'REMOVE_REVIEW', 'DISMISS_REVIEW_REPORT', 'ACTION_BUSINESS_REPORT', 'DISMISS_BUSINESS_REPORT'
  target_entity_type: VARCHAR(30) [NOT NULL] -- 'Business', 'BusinessClaim', 'BusinessSubmission', 'Review', 'BusinessReport', 'ReviewReport'
  target_entity_id:   VARCHAR(64) [NOT NULL]
  metadata:           JSONB [NOT NULL, DEFAULT '{}']
  created_at:         TIMESTAMPTZ [NOT NULL, DEFAULT NOW()]
}
```

---

## 5. Architectural Standards

### 5.1 Time and Date Standard
- **Database Storage:** All dates and timestamps are stored in UTC (`TIMESTAMPTZ` in PostgreSQL).
- **Go Type:** Mapped directly to `time.Time`.
- **Target Timezone:** Business hours and user bookings evaluate according to the local timezone of the venue (`Africa/Addis_Ababa`, UTC+3). However, timestamps must never be converted or stored as local naive dates.

### 5.2 Monetary Representation (ETB)
- **Problem:** Floating-point types (`float32`, `float64`, `REAL`) cause precision loss and rounding errors.
- **Decision:** All prices are stored as **integer minor units** (ETB Santims, where $1 \text{ ETB} = 100 \text{ Santims}$) using PostgreSQL `BIGINT` and Go `int64`.
- **Frontend Compatibility:** The API formats minor units for frontend display:
  $$\text{display\_string} = \frac{\text{price\_etb\_minor}}{100} + \text{" ETB"}$$
  Example: `25000` minor units becomes `"250 ETB"`.

### 5.3 Media and Images
- Images are decoupled from binary database storage.
- The `Business` entity stores references to image URIs (or an ordered relation `BusinessImage`).
- Storage URLs point to cloud object storage (Cloudflare R2 / AWS S3 / Google Cloud Storage) with CDN delivery.

---

## 6. The "Visit" Model Evaluation

In Section 11 of the backend requirements, we must answer:
> *Does "Visit" need to be a separate persistent entity or is a COMPLETED reservation sufficient?*

### Decision: COMPLETED Reservation is Authoritative
1. **Current Product Scope:** The frontend currently triggers verified review eligibility directly from `Reservation.status === "completed"`.
2. **Entity Redundancy:** A separate `Visit` table would introduce a 1:1 shadow record with duplicate timestamps (`completedAt`), foreign keys (`customerId`, `businessId`), and synchronization risks.
3. **Canonical Truth:** The `Reservation` table holds the authoritative reservation lifecycle. When a business manager marks a guest as served via the `completeBooking` action, `status` becomes `'COMPLETED'` and `completed_at` is stamped.
4. **Foreign Key Integrity:** `Review.reservation_id` points directly to `Reservation.id`. A database check ensures:
   ```sql
   CHECK (
     EXISTS (
       SELECT 1 FROM reservations 
       WHERE reservations.id = reservation_id 
       AND reservations.status = 'COMPLETED'
     )
   )
   ```
5. **Future Proofing:** If walk-in visits or physical QR-code check-ins without prior reservations are introduced in future product phases, a unified `Visit` entity can be introduced then. For Backend 01, keeping it within `Reservation` avoids gratuitous complexity.
