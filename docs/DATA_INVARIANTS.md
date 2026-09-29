# CAFNET BACKEND DATA INVARIANTS SPECIFICATION
**Document ID:** `CAFNET-BACKEND-01-DATA-INVARIANTS`  
**Status:** Canonical Architecture Specification  
**Target Stack:** Go 1.23+, Neon PostgreSQL  

---

## 1. Authoritative vs. Derived Data

A core principle of the Cafnet architecture is establishing a single source of truth for every attribute. Derived data must never become an unmanaged shadow state.

| Data Concept | Classification | Authoritative Source | Derived Formula / Derivation Method |
| :--- | :--- | :--- | :--- |
| **Business Name & Profile** | Authoritative | `Business` table | Direct lookup |
| **Menu Item Price** | Authoritative | `MenuItem.price_etb_minor` | Direct lookup (ETB minor units) |
| **Reservation Status** | Authoritative | `Reservation.status` | State machine transition |
| **Review Rating & Text** | Authoritative | `Review.rating`, `Review.review_text` | Direct lookup (where `status = 'VISIBLE'`) |
| **Business Average Rating** | **Derived** | `Review` table | $$\text{avg\_rating} = \frac{\sum \text{rating}}{\text{count(reviews)}} \quad (\text{rounded to 1 decimal})$$ |
| **Business Review Count** | **Derived** | `Review` table | $$\text{review\_count} = \text{COUNT}(Review.id) \quad \text{where status = 'VISIBLE'}$$ |
| **Rating Distribution** | **Derived** | `Review` table | Aggregated count per rating score $[5, 4, 3, 2, 1]$ |
| **Business Open / Closed State** | **Derived** | `BusinessHours` + Local Time | Evaluated dynamically: current local time vs. day-of-week schedule |
| **Review Helpfulness Count** | **Derived** | `ReviewHelpfulVote` | $$\text{count} = \text{COUNT}(ReviewHelpfulVote.customer\_user\_id)$$ |
| **Review Verified Badge** | **Derived / Certified**| `Reservation.status = 'COMPLETED'` | Set during review creation via foreign key verification |
| **Review Eligibility** | **Derived** | `Reservation` & `Review` | $$\text{eligible} \iff (\text{Reservation.status} = \text{'COMPLETED'} \land \neg \exists \text{Review}(\text{res\_id}))$$ |
| **Search Relevance Score** | **Derived** | PostgreSQL Search Expression | Multi-token match across name, category, cuisine, neighborhood, and menu |

### Denormalization & Materialized Cache Strategy
While authoritative rating statistics live in `Review`, sorting hundreds of listings by average rating in search feeds requires efficient querying.
- **Initial Scale:** PostgreSQL indexed subqueries or SQL aggregate views (`SELECT business_id, AVG(rating), COUNT(*) FROM reviews WHERE status = 'VISIBLE' GROUP BY business_id`).
- **Optimization Strategy:** Denormalized columns on `Business` (`cached_rating_avg`, `cached_review_count`) updated atomically inside the same database transaction that inserts, soft-deletes, or moderates a `Review`. The authoritative source remains `Review`.

---

## 2. Hard Deletion vs. Soft Deletion vs. State Transitions

| Entity | Deletion Strategy | Implementation Mechanism | Architectural Rationale |
| :--- | :--- | :--- | :--- |
| **`Business`** | **Soft Delete** | `deleted_at TIMESTAMPTZ NULL` | Deleting a business must not corrupt customer booking histories, receipts, or audit logs. |
| **`MenuCategory`** | **Soft Delete** | `deleted_at TIMESTAMPTZ NULL` | Preserves historical menus referenced in past analytics. |
| **`MenuItem`** | **Soft Delete** | `deleted_at TIMESTAMPTZ NULL` | Prevents foreign key breakage in past order/item references. |
| **`BusinessUpdate`**| **Soft Delete** | `deleted_at TIMESTAMPTZ NULL` | Allows managers to remove an update while preserving an audit record of broadcasts. |
| **`Reservation`** | **State Transition** | `status = 'CANCELLED' \| 'DECLINED'` | **NEVER DELETED.** Reservation records are binding legal and accounting contracts. |
| **`Review`** | **Soft Delete & Moderation State** | Author: `deleted_at = NOW()`<br>Admin: `status = 'REMOVED_BY_ADMIN'` | Retains moderation history. Customer review deletion hides text from public but preserves audit. |
| **`BusinessClaim`**| **State Transition** | `status = 'REJECTED'` | **NEVER DELETED.** Preserves claim history for fraud detection and ownership disputes. |
| **`BusinessSubmission`**| **State Transition**| `status = 'REJECTED'` | **NEVER DELETED.** Preserves provenance of who submitted each venue. |
| **`ReviewReport`** | **State Transition** | `status = 'DISMISSED' \| 'ACTIONED'` | Reports are immutable audit records of community moderation. |
| **`BusinessReport`**| **State Transition** | `status = 'DISMISSED' \| 'ACTIONED'` | Audit history of venue corrections. |
| **`SavedBusiness`** | **Hard Delete** | `DELETE FROM saved_businesses WHERE ...` | Join records represent ephemeral user preferences; clean deletion is safe. |
| **`FollowedBusiness`**| **Hard Delete**| `DELETE FROM followed_businesses WHERE ...` | Clean join record deletion on unfollow. |
| **`ReviewHelpfulVote`**| **Hard Delete**| `DELETE FROM review_helpful_votes WHERE ...` | Toggling un-votes cleans the join table. |
| **`Session`** | **Hard Delete / TTL**| Expired row cleanup | Ephemeral authentication tokens. |
| **`AdminAction`** | **Immutable Ledger**| Append-only table | Audit records must never be modified or deleted. |

---

## 3. Database Constraints Requirements

Neon PostgreSQL schema constraints must enforce business invariants at the lowest storage layer:

### 3.1 Uniqueness Constraints
1. **One Review per Reservation:**
   ```sql
   ALTER TABLE reviews ADD CONSTRAINT unique_reservation_review UNIQUE (reservation_id);
   ```
2. **One Active Claim per Business:**
   ```sql
   CREATE UNIQUE INDEX idx_unique_pending_business_claim 
   ON business_claims (business_id) 
   WHERE status = 'PENDING';
   ```
3. **No Duplicate Bookmark / Follow:**
   ```sql
   ALTER TABLE saved_businesses ADD PRIMARY KEY (user_id, business_id);
   ALTER TABLE followed_businesses ADD PRIMARY KEY (user_id, business_id);
   ```
4. **No Duplicate Helpful Vote:**
   ```sql
   ALTER TABLE review_helpful_votes ADD PRIMARY KEY (review_id, customer_user_id);
   ```
5. **No Duplicate Community Review Report:**
   ```sql
   ALTER TABLE review_reports ADD CONSTRAINT unique_reporter_review UNIQUE (review_id, reporter_user_id);
   ```
6. **No Duplicate Category Names per Venue:**
   ```sql
   ALTER TABLE menu_categories ADD CONSTRAINT unique_business_category_name UNIQUE (business_id, name);
   ```
7. **Single Phone Number per Account:**
   ```sql
   ALTER TABLE users ADD CONSTRAINT unique_user_phone UNIQUE (phone);
   ```

### 3.2 Check Constraints
1. **Valid Rating Score:**
   ```sql
   ALTER TABLE reviews ADD CONSTRAINT check_valid_rating CHECK (rating >= 1 AND rating <= 5);
   ```
2. **Valid Party Size:**
   ```sql
   ALTER TABLE reservations ADD CONSTRAINT check_valid_party_size CHECK (party_size > 0 AND party_size <= 50);
   ```
3. **Valid ETB Minor Units:**
   ```sql
   ALTER TABLE menu_items ADD CONSTRAINT check_valid_price CHECK (price_etb_minor >= 0);
   ```
4. **Valid Weekly Day of Week:**
   ```sql
   ALTER TABLE business_hours ADD CONSTRAINT check_valid_dow CHECK (day_of_week >= 0 AND day_of_week <= 6);
   ```

---

## 4. Concurrency & Race Condition Scenarios

In a concurrent multi-user system, simultaneous actions can lead to data races if not handled with explicit database locking and transactional boundaries.

| Scenario | Potential Race Condition | Prevention & Locking Strategy |
| :--- | :--- | :--- |
| **1. Simultaneous Claims on Same Venue** | Two users submit a claim for an unclaimed business at the exact same millisecond. | **Partial Unique Index:** `WHERE status = 'PENDING'` ensures the second insert immediately aborts with unique violation error (`23505`). |
| **2. Concurrent Review Submissions** | A customer double-clicks "Submit Review", firing two parallel HTTP requests. | **Unique Constraint:** `UNIQUE (reservation_id)` guarantees only one review row can commit. Second transaction fails safely. |
| **3. Manager Confirms while Customer Cancels** | Manager clicks "Confirm" at the exact second customer clicks "Cancel". | **Row-Level Pessimistic Locking:** `SELECT * FROM reservations WHERE id = $1 FOR UPDATE`. First transaction acquires the lock and transitions state; second transaction reads updated state and is rejected by the transition matrix. |
| **4. Two Admins Moderate Same Review** | Admin A clicks "Dismiss" while Admin B clicks "Remove Review". | **Optimistic Concurrency / Status Check:** `UPDATE review_reports SET status = $1 WHERE id = $2 AND status = 'PENDING'`. Only one admin's update affects rows; the other receives 0 affected rows. |
| **5. Simultaneous Helpful Upvote Toggle** | Rapid tapping on "Helpful" button. | **Upsert / Idempotent Query:** `INSERT ... ON CONFLICT DO NOTHING` or atomic toggle logic. |
| **6. Menu Reordering Race** | Two managers reordering categories simultaneously. | **Transaction with Business Lock:** `SELECT pg_advisory_xact_lock(business_id)` during batch reorder operations. |

---

## 5. PostgreSQL Search & Discovery Strategy

Cafnet's search spans business names, descriptions, categories, cuisines, neighborhoods, amenities/tags, and menu items.

### Recommended PostgreSQL Architecture
Rather than adding an external Elasticsearch or Typesense dependency prematurely, PostgreSQL natively provides high-performance search capabilities sufficient for Cafnet:

1. **Full-Text Search (`tsvector` & `tsquery`):**
   ```sql
   ALTER TABLE businesses ADD COLUMN search_vector tsvector 
   GENERATED ALWAYS AS (
     setweight(to_tsvector('english', coalesce(name, '')), 'A') ||
     setweight(to_tsvector('english', coalesce(category, '') || ' ' || coalesce(cuisine, '')), 'B') ||
     setweight(to_tsvector('english', coalesce(neighborhood, '') || ' ' || coalesce(location_name, '')), 'B') ||
     setweight(to_tsvector('english', coalesce(description, '')), 'C')
   ) STORED;

   CREATE INDEX idx_businesses_search ON businesses USING GIN (search_vector);
   ```

2. **Fuzzy Matching (`pg_trgm` extension):**
   - For handling misspellings (e.g., "Kategna" vs "Kategina", "Kazanchis" vs "Kazanchise").
   - GIN trigram index on `name` and `neighborhood`.

3. **Menu Item Search Aggregation:**
   - Join with `menu_items` and `menu_categories` during search queries, or maintain an aggregated `menu_search_vector` on the `businesses` record.

---

## 6. Future Domain Events (Notification Pipeline)

Although notification delivery (SMS/Push) is deferred to future backend updates, domain operations will emit typed **Domain Events** to decouple business logic from notification dispatchers:

```
[Domain Service] ---> (Event Bus / Outbox Table) ---> [Notification Handlers]
```

### Planned Domain Events
1. `event.business.submitted` $\rightarrow$ Alert platform administrators to review new venue.
2. `event.business.approved` $\rightarrow$ Notify submitter that listing is live and management access is granted.
3. `event.business.rejected` $\rightarrow$ Notify submitter with rejection reason.
4. `event.claim.submitted` $\rightarrow$ Alert platform administrators.
5. `event.claim.approved` $\rightarrow$ Notify claimant that ownership has been verified.
6. `event.claim.rejected` $\rightarrow$ Notify claimant with rejection reason.
7. `event.reservation.created` $\rightarrow$ Notify restaurant managers of pending table request.
8. `event.reservation.confirmed` $\rightarrow$ Notify customer with booking confirmation code.
9. `event.reservation.declined` $\rightarrow$ Notify customer with restaurant decline reason.
10. `event.reservation.completed` $\rightarrow$ Prompt customer to leave a verified review.
11. `event.review.reported` $\rightarrow$ Queue review for administrator moderation triage.
12. `event.business.reported` $\rightarrow$ Queue listing for administrator factual verification.
