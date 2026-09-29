# Cafnet Backend Architecture Documentation
**Phase:** BACKEND 01 & 02 — Product & Domain Architecture, Neon PostgreSQL Schema & Migrations  
**Target Stack:** Go 1.23+, Neon PostgreSQL, Go-Native Authentication  

This directory contains the canonical specifications for the Cafnet backend. These documents define the authoritative entities, relationships, state machines, authorization rules, data invariants, database schema, and migration workflows.

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

7. **[DATABASE_SCHEMA.md](./DATABASE_SCHEMA.md)**
   - Complete Table-by-Table Schema Specification
   - Primary Keys, Foreign Keys, Unique & Check Constraints
   - Procedural State Machine Triggers (`check_reservation_status_transition`, `check_review_reservation_completed`)
   - Composite Unique Indexes & Full-Text Search Vectors

8. **[MIGRATIONS.md](./MIGRATIONS.md)**
   - Go Migration Runner Architecture (`cmd/migrate/main.go`)
   - CLI Usage (`up`, `down`, `status`, `version`)
   - Neon Development Workflow & `.env.example`
   - Reversibility & Rollback Guarantees

9. **[REPOSITORY_LAYER.md](./REPOSITORY_LAYER.md)**
   - Domain Model & Repository Layer Architecture
   - Context-Injected Transaction Management (`TxManager`, `DBTX`)
   - PostgreSQL Error Mapping to Domain Sentinels
   - Complete 12 Repository Contracts & PostgreSQL Implementations
   - Automated Invariant & Transaction Verification Suite

10. **[AUTHENTICATION.md](./AUTHENTICATION.md)**
    - Go-Native Authentication Architecture & Session Foundation
    - Phone Number E.164 Deterministic Normalization
    - Cryptographic OTP Lifecycle, Security, Rate Limiting & Cooldown
    - Opaque Random Session Tokens, SHA-256 Storage & Multi-Device Revocation
    - HTTP-Only Cookie Strategy, CSRF & CORS Deployment Specifications

11. **[AUTHORIZATION.md](./AUTHORIZATION.md)**
    - Platform Roles vs. Dynamic Venue Management ReBAC
    - Formal Authorization Predicate (`can_manage_business`) & IDOR Protection
    - Cross-Business Security Isolation & Multi-Venue Management Verification
    - Atomic Approval Workflows (`ApproveClaim`, `ApproveSubmission`) & Self-Escalation Guards
    - Server-Side Resource Ownership Policies & Route-to-Permission Matrix
