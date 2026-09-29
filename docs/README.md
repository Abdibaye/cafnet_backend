# Cafnet Backend Architecture Documentation
**Phase:** BACKEND 01 — Product & Domain Architecture  
**Target Stack:** Go 1.23+, Neon PostgreSQL, Go-Native Authentication  

This directory contains the canonical domain specifications for the Cafnet backend. These documents define the authoritative entities, relationships, state machines, authorization rules, data invariants, and frontend-to-backend mappings.

---

## Architecture Documents Index

1. **[DOMAIN_MODEL.md](./DOMAIN_MODEL.md)**
   - Domain Philosophy & Boundaries (10 Canonical Domains)
   - Canonical Core Entities & Attribute Types
   - Architectural Standards (UTC Timestamps, ETB Minor Unit Pricing, Media References)
   - Canonical Visit Model Evaluation

2. **[ENTITY_RELATIONSHIPS.md](./ENTITY_RELATIONSHIPS.md)**
   - Visual Entity-Relationship Diagram (ERD)
   - Cardinality Matrix & Referential Semantics
   - Data Ownership Matrix (Creator, Owner, Manager, Admin Authority)
   - Foreign Key Cascading vs. Restrict Policies

3. **[STATE_MACHINES.md](./STATE_MACHINES.md)**
   - Business Lifecycle & Pathways (New Submissions vs. Claims)
   - Orthogonal Business Dimensions (Existence, Publication, Ownership, Verification)
   - Reservation Lifecycle State Machine (5-state controlled flow)
   - Claim & Submission State Machines
   - Review & Business Report Moderation State Machines
   - Business Update Expiration Lifecycle

4. **[AUTHORIZATION_MODEL.md](./AUTHORIZATION_MODEL.md)**
   - Single Canonical User Identity with Dual-Layer Access (RBAC + Venue ReBAC)
   - The Core Ownership Authorization Rule (`can_manage_business`)
   - Actor Capabilities (Customer, Business Manager, Platform Admin)
   - Fine-Grained Operation Permission Matrix
   - Authentication Boundary vs. Domain Service Boundary

5. **[FRONTEND_BACKEND_MAPPING.md](./FRONTEND_BACKEND_MAPPING.md)**
   - Screen-by-Screen Mapping Matrix (UI Components to Domain Entities & Go API Contracts)
   - Complete 20-Point Feature Coverage Verification Checklist

6. **[DATA_INVARIANTS.md](./DATA_INVARIANTS.md)**
   - Authoritative vs. Derived Data Formulas (Ratings, Counts, Schedules)
   - Deletion Strategies (Hard Delete vs. Soft Delete vs. Terminal State Transitions)
   - Neon PostgreSQL Constraints (Unique indexes, checks, foreign keys)
   - Concurrency Scenarios & Race Condition Locking Strategies
   - PostgreSQL Full-Text Search Strategy
   - Future Domain Events Pipeline (For async notifications)
