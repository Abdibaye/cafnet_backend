# CAFNET BACKEND AUTHORIZATION MODEL SPECIFICATION
**Document ID:** `CAFNET-BACKEND-01-AUTHORIZATION-MODEL`  
**Status:** Canonical Architecture Specification  
**Target Stack:** Go 1.23+, Neon PostgreSQL, Go-Native Auth  

---

## 1. Identity & Role Architecture

In Cafnet, every participant interacts with the system through a single canonical `User` identity. A business owner is also a customer: they can discover other restaurants, save places to their personal collections, follow other kitchens, make table reservations, and write verified reviews for dining visits they have personally completed.

```
+-----------------------------------------------------------------------------------+
|                              CANONICAL USER IDENTITY                              |
|                              (Phone Number, Profile)                              |
+-----------------------------------------+-----------------------------------------+
                                          |
                   +----------------------+----------------------+
                   |                                             |
            [PLATFORM ROLE]                             [BUSINESS CONTEXT]
                   |                                             |
         +---------+---------+                     +-------------+-------------+
         |                   |                     |                           |
     CUSTOMER              ADMIN             BUSINESS MANAGER            NO ACCESS
  (Default for all)    (Governance)       (Via BusinessManager M:N)    (Regular User)
```

### 1.1 The Two Authorization Layers
1. **Platform Layer (Role-Based Access Control - RBAC):**
   - Stored directly on `User.role`.
   - `CUSTOMER`: The standard baseline for every registered user.
   - `ADMIN`: Platform staff with authority over verification, moderation, claims, and governance.

2. **Venue Management Layer (Attribute/Relationship-Based Access Control - ReBAC):**
   - Rather than treating "Business Owner" as a global role, management rights are evaluated dynamically through the `BusinessManager` relationship table.
   - A user may manage zero, one, or twenty different businesses.
   - For each business, a user is either an authorized manager or simply a member of the public.

---

## 2. The Core Ownership Authorization Rule

> **CRITICAL DOMAIN INVARIANT:**  
> A user possessing business management privileges on Venue X holds **zero** inherent rights over Venue Y. Management authorization must strictly evaluate the intersection of the current authenticated user and the specific target business.

### Formal Authorization Predicate
A user $U$ is authorized to view private business operations or mutate business $B$ if and only if:

$$\text{can\_manage\_business}(U, B) \iff \Big(U.\text{role} = \text{'ADMIN'}\Big) \lor \Big(\exists M \in \text{BusinessManagers} : M.\text{user\_id} = U.\text{id} \land M.\text{business\_id} = B.\text{id}\Big)$$

If $\text{can\_manage\_business}(U, B)$ evaluates to `false`, the operation MUST be rejected with HTTP `403 Forbidden` at the domain service layer.

---

## 3. Actor Capabilities & Boundaries

### 3.1 CUSTOMER (Authenticated)
- **CAN:**
  - Search and discover any published business.
  - Save businesses to private collection (`SavedBusiness`).
  - Follow businesses for update feeds (`FollowedBusiness`).
  - Submit table booking requests (`Reservation`).
  - View personal reservation history and live statuses.
  - Cancel personal bookings (if currently in `PENDING` or `CONFIRMED` state).
  - Submit a review with rating and comments for personal completed reservations (`Reservation.status = 'COMPLETED'`).
  - Edit or delete personal reviews.
  - Vote reviews as helpful (`ReviewHelpfulVote`).
  - Submit a report on a problematic review (`ReviewReport`).
  - Submit a report on inaccurate business listing details (`BusinessReport`).
  - Submit a claim for an existing unclaimed venue (`BusinessClaim`).
  - Submit a proposal for a new venue (`BusinessSubmission`).
- **CANNOT:**
  - View or alter unpublished businesses.
  - Edit business listing profiles, menus, or photos.
  - Confirm, decline, or complete reservations.
  - Post business updates.
  - Review reservations belonging to other users or reservations not yet completed.
  - Review or dismiss moderation reports.
  - Approve claims or submissions.

### 3.2 BUSINESS MANAGER (Authorized for Venue $B$)
- **CAN (Strictly scoped to Venue $B$):**
  - Access the Business Center and management dashboard for Venue $B$.
  - Edit listing profile (name, description, phone, location, opening hours, tags, price range, website).
  - Manage menu categories (create, edit, delete, reorder).
  - Manage menu items (create, edit, delete, reorder, price in ETB).
  - Publish, edit, and delete business updates (offers, events, announcements).
  - View all incoming and historical reservations for Venue $B$.
  - Confirm pending reservations.
  - Decline pending reservations (with mandatory reason).
  - Mark confirmed visits as completed.
- **CANNOT:**
  - Access the dashboard or mutate any data for Venue $C$ (where $C \ne B$).
  - Self-verify their own business listing badge (requires Admin).
  - Approve or reject ownership claims (requires Admin).
  - Moderate, hide, or delete negative customer reviews (must use the standard reporting flow).
  - Bypass moderation or administrative oversight.

### 3.3 PLATFORM ADMIN
- **CAN:**
  - Review, approve, or reject new business submissions (`BusinessSubmission`).
  - Review, approve, or reject business ownership claims (`BusinessClaim`).
  - Toggle official platform verification badges (`Business.verification_status`).
  - Publish or unpublish any business listing.
  - Moderate reported customer reviews (dismiss report or action takedown).
  - Review reported business information errors (dismiss or apply authoritative corrections).
  - Inspect system-wide operational metrics and audit logs (`AdminAction`).
- **CANNOT:**
  - Post reviews masquerading as a customer without a completed reservation.
  - Alter audit trail logs (append-only ledger).

---

## 4. Operation Permission Matrix

| Operation | Anonymous Visitor | Authenticated Customer | Business Manager (Target Venue) | Business Manager (Other Venue) | Platform Admin |
| :--- | :---: | :---: | :---: | :---: | :---: |
| **Browse / Search published venues** | ALLOW | ALLOW | ALLOW | ALLOW | ALLOW |
| **View business menu & updates** | ALLOW | ALLOW | ALLOW | ALLOW | ALLOW |
| **Save / Follow business** | DENY (Auth req) | ALLOW | ALLOW | ALLOW | ALLOW |
| **Request table reservation** | DENY (Auth req) | ALLOW | ALLOW | ALLOW | ALLOW |
| **Track own reservation** | DENY | ALLOW (Own only) | ALLOW (Own only) | ALLOW (Own only) | ALLOW |
| **Cancel own reservation** | DENY | ALLOW (Own only) | ALLOW (Own only) | ALLOW (Own only) | ALLOW |
| **Confirm / Decline reservation** | DENY | DENY | **ALLOW** | **DENY** | ALLOW |
| **Mark visit completed** | DENY | DENY | **ALLOW** | **DENY** | ALLOW |
| **Write review for completed visit** | DENY | ALLOW (Own only) | ALLOW (Own only)| ALLOW (Own only) | DENY (Must have visit) |
| **Edit / Delete own review** | DENY | ALLOW (Own only) | ALLOW (Own only)| ALLOW (Own only) | ALLOW (Delete/Remove) |
| **Vote review helpful** | DENY | ALLOW | ALLOW | ALLOW | ALLOW |
| **Report review / business** | DENY | ALLOW | ALLOW | ALLOW | ALLOW |
| **Submit claim on business** | DENY | ALLOW | ALLOW | ALLOW | ALLOW |
| **Submit new business** | DENY | ALLOW | ALLOW | ALLOW | ALLOW |
| **Edit business listing profile** | DENY | DENY | **ALLOW** | **DENY** | ALLOW |
| **Manage menu categories & items** | DENY | DENY | **ALLOW** | **DENY** | ALLOW |
| **Publish business updates** | DENY | DENY | **ALLOW** | **DENY** | ALLOW |
| **Approve / Reject claim** | DENY | DENY | DENY | DENY | **ALLOW** |
| **Approve / Reject submission** | DENY | DENY | DENY | DENY | **ALLOW** |
| **Remove review via moderation** | DENY | DENY | DENY | DENY | **ALLOW** |
| **Action business report** | DENY | DENY | DENY | DENY | **ALLOW** |

---

## 5. Authentication vs. Domain Authorization Boundaries

To ensure clean Go architecture, the backend cleanly separates the **Authentication Boundary** from the **Domain Authorization Boundary**:

```
[HTTP Request]
       |
       v
+---------------------------------------------------------------+
| AUTHENTICATION BOUNDARY (Middleware / Go Native Auth)         |
| 1. Extracts session token / cookie / Bearer                   |
| 2. Validates cryptographic signature & expiration             |
| 3. Resolves User ID & User.role (CUSTOMER, ADMIN)             |
| 4. Injects `UserContext` into Go `context.Context`            |
+---------------------------------------------------------------+
       |
       v
+---------------------------------------------------------------+
| DOMAIN AUTHORIZATION BOUNDARY (Application Services)          |
| 1. Service receives command: `UpdateMenu(ctx, businessID, ...)|
| 2. Queries relationship: `can_manage_business(user, business)`|
| 3. Evaluates business invariants (e.g. is business published?)|
| 4. Executes transaction or rejects with Domain Error          |
+---------------------------------------------------------------+
```

### Invariants:
1. Authentication middleware NEVER queries business ownership or menu structures; it strictly authenticates identity.
2. Domain services NEVER inspect raw headers, cookies, or JWT claims; they receive typed Go context representations (`auth.Principal`).
3. Administrative override paths must be explicitly logged to the `AdminAction` audit ledger.
