-- CAFNET INITIAL SCHEMA MIGRATION (UP)
-- Version: 000001
-- Description: Core schema, constraints, state machines, and indexes for Cafnet

-- ============================================================================
-- 1. EXTENSIONS
-- ============================================================================
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
CREATE EXTENSION IF NOT EXISTS "pg_trgm";

-- ============================================================================
-- 2. UTILITY FUNCTIONS & PROCEDURAL TRIGGERS
-- ============================================================================

-- Automatically updates updated_at timestamp on row update
CREATE OR REPLACE FUNCTION set_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Enforces canonical reservation status transition rules and decline reason
CREATE OR REPLACE FUNCTION check_reservation_status_transition()
RETURNS TRIGGER AS $$
BEGIN
    -- Terminal states cannot be changed under any circumstance
    IF OLD.status IN ('COMPLETED', 'DECLINED', 'CANCELLED') AND NEW.status <> OLD.status THEN
        RAISE EXCEPTION 'Cannot transition reservation % from terminal status % to %', OLD.id, OLD.status, NEW.status
            USING ERRCODE = 'check_violation';
    END IF;

    -- PENDING transitions
    IF OLD.status = 'PENDING' AND NEW.status NOT IN ('PENDING', 'CONFIRMED', 'DECLINED', 'CANCELLED') THEN
        RAISE EXCEPTION 'Illegal reservation status transition from PENDING to %', NEW.status
            USING ERRCODE = 'check_violation';
    END IF;

    -- CONFIRMED transitions
    IF OLD.status = 'CONFIRMED' AND NEW.status NOT IN ('CONFIRMED', 'COMPLETED', 'CANCELLED') THEN
        RAISE EXCEPTION 'Illegal reservation status transition from CONFIRMED to %', NEW.status
            USING ERRCODE = 'check_violation';
    END IF;

    -- Decline requires a non-empty decline reason
    IF NEW.status = 'DECLINED' AND (NEW.decline_reason IS NULL OR TRIM(NEW.decline_reason) = '') THEN
        RAISE EXCEPTION 'A non-empty decline reason is required when declining reservation %', NEW.id
            USING ERRCODE = 'check_violation';
    END IF;

    -- Timestamp management for transitions
    IF OLD.status <> 'CONFIRMED' AND NEW.status = 'CONFIRMED' AND NEW.confirmed_at IS NULL THEN
        NEW.confirmed_at = NOW();
    END IF;

    IF OLD.status <> 'DECLINED' AND NEW.status = 'DECLINED' AND NEW.declined_at IS NULL THEN
        NEW.declined_at = NOW();
    END IF;

    IF OLD.status <> 'COMPLETED' AND NEW.status = 'COMPLETED' AND NEW.completed_at IS NULL THEN
        NEW.completed_at = NOW();
    END IF;

    IF OLD.status <> 'CANCELLED' AND NEW.status = 'CANCELLED' AND NEW.cancelled_at IS NULL THEN
        NEW.cancelled_at = NOW();
    END IF;

    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Enforces that a review can ONLY be created for a COMPLETED reservation
CREATE OR REPLACE FUNCTION check_review_reservation_completed()
RETURNS TRIGGER AS $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM reservations 
        WHERE id = NEW.reservation_id AND status = 'COMPLETED'
    ) THEN
        RAISE EXCEPTION 'A review can only be submitted for a COMPLETED reservation (reservation_id: %)', NEW.reservation_id
            USING ERRCODE = 'check_violation';
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- ============================================================================
-- 3. CORE ENTITIES
-- ============================================================================

-- ----------------------------------------------------------------------------
-- 3.1 USERS
-- ----------------------------------------------------------------------------
CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    phone VARCHAR(20) NOT NULL,
    full_name VARCHAR(100) NOT NULL,
    avatar_url TEXT,
    role VARCHAR(20) NOT NULL DEFAULT 'CUSTOMER',
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ,

    CONSTRAINT unique_user_phone UNIQUE (phone),
    CONSTRAINT check_user_role CHECK (role IN ('CUSTOMER', 'ADMIN'))
);

CREATE TRIGGER trg_users_updated_at
BEFORE UPDATE ON users
FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- ----------------------------------------------------------------------------
-- 3.2 SESSIONS
-- ----------------------------------------------------------------------------
CREATE TABLE sessions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash VARCHAR(64) NOT NULL,
    user_agent TEXT,
    ip_address VARCHAR(45),
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT unique_session_token UNIQUE (token_hash)
);

CREATE INDEX idx_sessions_user_id ON sessions(user_id);
CREATE INDEX idx_sessions_expires_at ON sessions(expires_at);

-- ----------------------------------------------------------------------------
-- 3.3 BUSINESSES
-- ----------------------------------------------------------------------------
CREATE TABLE businesses (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    name VARCHAR(150) NOT NULL,
    slug VARCHAR(160) NOT NULL,
    category VARCHAR(30) NOT NULL,
    cuisine VARCHAR(80) NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    short_description VARCHAR(200) NOT NULL DEFAULT '',
    location_name VARCHAR(100) NOT NULL,
    neighborhood VARCHAR(80) NOT NULL,
    address_line TEXT,
    latitude DECIMAL(10, 7),
    longitude DECIMAL(10, 7),
    phone VARCHAR(30) NOT NULL DEFAULT '',
    website VARCHAR(255),
    price_range VARCHAR(10) NOT NULL DEFAULT '$$',
    tags TEXT[] NOT NULL DEFAULT '{}',
    payment_methods TEXT[] NOT NULL DEFAULT '{"Cash"}',
    reservation_available BOOLEAN NOT NULL DEFAULT FALSE,

    -- Administrative & Platform States (Strictly Separated)
    publication_status VARCHAR(20) NOT NULL DEFAULT 'UNPUBLISHED',
    verification_status VARCHAR(20) NOT NULL DEFAULT 'UNVERIFIED',
    claim_status VARCHAR(20) NOT NULL DEFAULT 'UNCLAIMED',
    is_featured BOOLEAN NOT NULL DEFAULT FALSE,

    created_by_user_id UUID REFERENCES users(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ,

    -- Search vector (Deterministic, stored full-text vector)
    search_vector tsvector GENERATED ALWAYS AS (
        setweight(to_tsvector('simple', COALESCE(name, '')), 'A') ||
        setweight(to_tsvector('simple', COALESCE(category, '') || ' ' || COALESCE(cuisine, '')), 'B') ||
        setweight(to_tsvector('simple', COALESCE(neighborhood, '') || ' ' || COALESCE(location_name, '')), 'B') ||
        setweight(to_tsvector('simple', COALESCE(description, '')), 'C')
    ) STORED,

    CONSTRAINT unique_business_slug UNIQUE (slug),
    CONSTRAINT check_business_category CHECK (category IN ('restaurant', 'cafe', 'drinks')),
    CONSTRAINT check_business_price_range CHECK (price_range IN ('$', '$$', '$$$', '$$$$')),
    CONSTRAINT check_business_publication_status CHECK (publication_status IN ('PUBLISHED', 'UNPUBLISHED')),
    CONSTRAINT check_business_verification_status CHECK (verification_status IN ('VERIFIED', 'UNVERIFIED')),
    CONSTRAINT check_business_claim_status CHECK (claim_status IN ('UNCLAIMED', 'CLAIMED'))
);

CREATE TRIGGER trg_businesses_updated_at
BEFORE UPDATE ON businesses
FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE INDEX idx_businesses_search_vector ON businesses USING GIN (search_vector);
CREATE INDEX idx_businesses_name_trgm ON businesses USING GIN (name gin_trgm_ops);
CREATE INDEX idx_businesses_neighborhood_trgm ON businesses USING GIN (neighborhood gin_trgm_ops);
CREATE INDEX idx_businesses_publication ON businesses(publication_status) WHERE deleted_at IS NULL;
CREATE INDEX idx_businesses_category ON businesses(category) WHERE publication_status = 'PUBLISHED' AND deleted_at IS NULL;
CREATE INDEX idx_businesses_location ON businesses(location_name, neighborhood);

-- ----------------------------------------------------------------------------
-- 3.4 BUSINESS HOURS
-- ----------------------------------------------------------------------------
CREATE TABLE business_hours (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    business_id BIGINT NOT NULL REFERENCES businesses(id) ON DELETE CASCADE,
    day_of_week SMALLINT NOT NULL, -- 0=Sunday, 1=Monday, ..., 6=Saturday
    opens_at TIME,
    closes_at TIME,
    is_closed BOOLEAN NOT NULL DEFAULT FALSE,

    CONSTRAINT unique_business_day UNIQUE (business_id, day_of_week),
    CONSTRAINT check_business_hours_dow CHECK (day_of_week >= 0 AND day_of_week <= 6)
);

CREATE INDEX idx_business_hours_business_id ON business_hours(business_id);

-- ----------------------------------------------------------------------------
-- 3.5 BUSINESS IMAGES
-- ----------------------------------------------------------------------------
CREATE TABLE business_images (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    business_id BIGINT NOT NULL REFERENCES businesses(id) ON DELETE CASCADE,
    image_url TEXT NOT NULL,
    alt_text VARCHAR(200),
    sort_order INT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_business_images_business_order ON business_images(business_id, sort_order);

-- ----------------------------------------------------------------------------
-- 3.6 BUSINESS MANAGERS (Ownership Relationship)
-- ----------------------------------------------------------------------------
CREATE TABLE business_managers (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    business_id BIGINT NOT NULL REFERENCES businesses(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    manager_role VARCHAR(20) NOT NULL DEFAULT 'MANAGER',
    granted_by UUID REFERENCES users(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT unique_business_manager UNIQUE (business_id, user_id),
    CONSTRAINT check_manager_role CHECK (manager_role IN ('PRIMARY_OWNER', 'MANAGER'))
);

CREATE INDEX idx_business_managers_user ON business_managers(user_id);
CREATE INDEX idx_business_managers_business ON business_managers(business_id);

-- ----------------------------------------------------------------------------
-- 3.7 BUSINESS CLAIMS
-- ----------------------------------------------------------------------------
CREATE TABLE business_claims (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    business_id BIGINT NOT NULL REFERENCES businesses(id) ON DELETE RESTRICT,
    claimant_user_id UUID NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    status VARCHAR(20) NOT NULL DEFAULT 'PENDING',
    submitted_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    reviewed_at TIMESTAMPTZ,
    reviewed_by UUID REFERENCES users(id) ON DELETE SET NULL,
    rejection_reason TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT check_claim_status CHECK (status IN ('PENDING', 'APPROVED', 'REJECTED'))
);

CREATE TRIGGER trg_business_claims_updated_at
BEFORE UPDATE ON business_claims
FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- Prevents conflicting active pending claims on the same business
CREATE UNIQUE INDEX idx_unique_pending_business_claim ON business_claims(business_id) WHERE status = 'PENDING';
CREATE INDEX idx_business_claims_claimant ON business_claims(claimant_user_id);
CREATE INDEX idx_business_claims_status ON business_claims(status);

-- ----------------------------------------------------------------------------
-- 3.8 BUSINESS SUBMISSIONS
-- ----------------------------------------------------------------------------
CREATE TABLE business_submissions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    submitter_user_id UUID NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    business_name VARCHAR(150) NOT NULL,
    category VARCHAR(30) NOT NULL,
    area VARCHAR(100) NOT NULL,
    submitted_payload JSONB NOT NULL DEFAULT '{}',
    status VARCHAR(20) NOT NULL DEFAULT 'PENDING',
    created_business_id BIGINT REFERENCES businesses(id) ON DELETE SET NULL,
    submitted_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    reviewed_at TIMESTAMPTZ,
    reviewed_by UUID REFERENCES users(id) ON DELETE SET NULL,
    rejection_reason TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT check_submission_category CHECK (category IN ('restaurant', 'cafe', 'drinks')),
    CONSTRAINT check_submission_status CHECK (status IN ('PENDING', 'APPROVED', 'REJECTED'))
);

CREATE TRIGGER trg_business_submissions_updated_at
BEFORE UPDATE ON business_submissions
FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE INDEX idx_business_submissions_submitter ON business_submissions(submitter_user_id);
CREATE INDEX idx_business_submissions_status ON business_submissions(status);

-- ----------------------------------------------------------------------------
-- 3.9 MENU CATEGORIES
-- ----------------------------------------------------------------------------
CREATE TABLE menu_categories (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    business_id BIGINT NOT NULL REFERENCES businesses(id) ON DELETE CASCADE,
    name VARCHAR(80) NOT NULL,
    sort_order INT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ,

    CONSTRAINT unique_business_category_name UNIQUE (business_id, name)
);

CREATE TRIGGER trg_menu_categories_updated_at
BEFORE UPDATE ON menu_categories
FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE INDEX idx_menu_categories_business_sort ON menu_categories(business_id, sort_order) WHERE deleted_at IS NULL;

-- ----------------------------------------------------------------------------
-- 3.10 MENU ITEMS
-- ----------------------------------------------------------------------------
CREATE TABLE menu_items (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    category_id UUID NOT NULL REFERENCES menu_categories(id) ON DELETE CASCADE,
    business_id BIGINT NOT NULL REFERENCES businesses(id) ON DELETE CASCADE,
    name VARCHAR(100) NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    price_etb_minor BIGINT NOT NULL, -- Integer minor units (e.g., 250 ETB = 25000 Santims)
    sort_order INT NOT NULL DEFAULT 0,
    is_available BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ,

    CONSTRAINT check_menu_item_price CHECK (price_etb_minor >= 0)
);

CREATE TRIGGER trg_menu_items_updated_at
BEFORE UPDATE ON menu_items
FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE INDEX idx_menu_items_category_sort ON menu_items(category_id, sort_order) WHERE deleted_at IS NULL;
CREATE INDEX idx_menu_items_business ON menu_items(business_id) WHERE deleted_at IS NULL;
CREATE INDEX idx_menu_items_name_trgm ON menu_items USING GIN (name gin_trgm_ops);

-- ----------------------------------------------------------------------------
-- 3.11 BUSINESS UPDATES
-- ----------------------------------------------------------------------------
CREATE TABLE business_updates (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    business_id BIGINT NOT NULL REFERENCES businesses(id) ON DELETE CASCADE,
    author_user_id UUID NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    update_type VARCHAR(30) NOT NULL,
    title VARCHAR(120) NOT NULL,
    description TEXT NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'PUBLISHED',
    event_date VARCHAR(50),
    event_time VARCHAR(30),
    price_text VARCHAR(50),
    offer_details VARCHAR(100),
    valid_until VARCHAR(50),
    expires_at TIMESTAMPTZ,
    published_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ,

    CONSTRAINT check_update_type CHECK (update_type IN ('new_menu', 'offer', 'event', 'price_update', 'announcement')),
    CONSTRAINT check_update_status CHECK (status IN ('DRAFT', 'PUBLISHED', 'EXPIRED'))
);

CREATE TRIGGER trg_business_updates_updated_at
BEFORE UPDATE ON business_updates
FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE INDEX idx_business_updates_business_pub ON business_updates(business_id, published_at DESC) WHERE deleted_at IS NULL;
CREATE INDEX idx_business_updates_active ON business_updates(status, expires_at) WHERE status = 'PUBLISHED' AND deleted_at IS NULL;

-- ----------------------------------------------------------------------------
-- 3.12 RESERVATIONS
-- ----------------------------------------------------------------------------
CREATE TABLE reservations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    reservation_code VARCHAR(20) NOT NULL,
    business_id BIGINT NOT NULL REFERENCES businesses(id) ON DELETE RESTRICT,
    customer_user_id UUID NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    customer_name VARCHAR(100) NOT NULL,
    customer_phone VARCHAR(30) NOT NULL,
    reservation_date DATE NOT NULL,
    reservation_time TIME NOT NULL,
    party_size INT NOT NULL,
    special_request TEXT NOT NULL DEFAULT '',
    status VARCHAR(20) NOT NULL DEFAULT 'PENDING',
    decline_reason TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    confirmed_at TIMESTAMPTZ,
    declined_at TIMESTAMPTZ,
    completed_at TIMESTAMPTZ,
    cancelled_at TIMESTAMPTZ,
    reviewed_by_user UUID REFERENCES users(id) ON DELETE SET NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT unique_reservation_code UNIQUE (reservation_code),
    CONSTRAINT check_reservation_party_size CHECK (party_size > 0 AND party_size <= 50),
    CONSTRAINT check_reservation_status CHECK (status IN ('PENDING', 'CONFIRMED', 'DECLINED', 'CANCELLED', 'COMPLETED'))
);

CREATE TRIGGER trg_reservations_updated_at
BEFORE UPDATE ON reservations
FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TRIGGER trg_reservations_status_guard
BEFORE UPDATE OF status, decline_reason ON reservations
FOR EACH ROW EXECUTE FUNCTION check_reservation_status_transition();

CREATE INDEX idx_reservations_business_date ON reservations(business_id, reservation_date, reservation_time);
CREATE INDEX idx_reservations_customer ON reservations(customer_user_id, created_at DESC);
CREATE INDEX idx_reservations_status ON reservations(status);

-- ----------------------------------------------------------------------------
-- 3.13 REVIEWS
-- ----------------------------------------------------------------------------
CREATE TABLE reviews (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    business_id BIGINT NOT NULL REFERENCES businesses(id) ON DELETE CASCADE,
    customer_user_id UUID NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    reservation_id UUID NOT NULL,
    rating SMALLINT NOT NULL,
    review_text TEXT NOT NULL DEFAULT '',
    is_verified_visit BOOLEAN NOT NULL DEFAULT TRUE,
    status VARCHAR(20) NOT NULL DEFAULT 'VISIBLE',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ,

    CONSTRAINT unique_reservation_review UNIQUE (reservation_id),
    CONSTRAINT fk_reviews_reservation FOREIGN KEY (reservation_id) REFERENCES reservations(id) ON DELETE RESTRICT,
    CONSTRAINT check_review_rating CHECK (rating >= 1 AND rating <= 5),
    CONSTRAINT check_review_status CHECK (status IN ('VISIBLE', 'REMOVED_BY_ADMIN'))
);

CREATE TRIGGER trg_reviews_updated_at
BEFORE UPDATE ON reviews
FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- Enforces that a review CAN ONLY BE CREATED for a COMPLETED reservation
CREATE TRIGGER trg_reviews_reservation_completed
BEFORE INSERT OR UPDATE OF reservation_id ON reviews
FOR EACH ROW EXECUTE FUNCTION check_review_reservation_completed();

CREATE INDEX idx_reviews_business_status ON reviews(business_id, status, created_at DESC) WHERE deleted_at IS NULL;
CREATE INDEX idx_reviews_customer ON reviews(customer_user_id);

-- ----------------------------------------------------------------------------
-- 3.14 REVIEW HELPFUL VOTES
-- ----------------------------------------------------------------------------
CREATE TABLE review_helpful_votes (
    review_id UUID NOT NULL REFERENCES reviews(id) ON DELETE CASCADE,
    customer_user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    PRIMARY KEY (review_id, customer_user_id)
);

CREATE INDEX idx_review_helpful_votes_user ON review_helpful_votes(customer_user_id);

-- ----------------------------------------------------------------------------
-- 3.15 REVIEW REPORTS
-- ----------------------------------------------------------------------------
CREATE TABLE review_reports (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    review_id UUID NOT NULL REFERENCES reviews(id) ON DELETE CASCADE,
    reporter_user_id UUID NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    reason VARCHAR(30) NOT NULL,
    explanation TEXT NOT NULL DEFAULT '',
    status VARCHAR(20) NOT NULL DEFAULT 'PENDING',
    reviewed_at TIMESTAMPTZ,
    reviewed_by UUID REFERENCES users(id) ON DELETE SET NULL,
    admin_action VARCHAR(20),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT unique_reporter_review UNIQUE (review_id, reporter_user_id),
    CONSTRAINT check_review_report_reason CHECK (reason IN ('spam', 'offensive', 'fake', 'other')),
    CONSTRAINT check_review_report_status CHECK (status IN ('PENDING', 'DISMISSED', 'ACTIONED')),
    CONSTRAINT check_review_report_admin_action CHECK (admin_action IS NULL OR admin_action IN ('dismissed', 'removed'))
);

CREATE TRIGGER trg_review_reports_updated_at
BEFORE UPDATE ON review_reports
FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE INDEX idx_review_reports_status ON review_reports(status, created_at DESC);
CREATE INDEX idx_review_reports_review ON review_reports(review_id);

-- ----------------------------------------------------------------------------
-- 3.16 BUSINESS REPORTS
-- ----------------------------------------------------------------------------
CREATE TABLE business_reports (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    business_id BIGINT NOT NULL REFERENCES businesses(id) ON DELETE CASCADE,
    reporter_user_id UUID NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    reason VARCHAR(30) NOT NULL,
    details TEXT,
    status VARCHAR(20) NOT NULL DEFAULT 'PENDING',
    reviewed_at TIMESTAMPTZ,
    reviewed_by UUID REFERENCES users(id) ON DELETE SET NULL,
    admin_notes TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT check_business_report_reason CHECK (reason IN ('WRONG_HOURS', 'WRONG_PHONE', 'WRONG_LOCATION', 'WRONG_INFORMATION', 'CLOSED', 'DUPLICATE', 'OTHER')),
    CONSTRAINT check_business_report_status CHECK (status IN ('PENDING', 'DISMISSED', 'ACTIONED'))
);

CREATE TRIGGER trg_business_reports_updated_at
BEFORE UPDATE ON business_reports
FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE INDEX idx_business_reports_status ON business_reports(status, created_at DESC);
CREATE INDEX idx_business_reports_business ON business_reports(business_id);

-- ----------------------------------------------------------------------------
-- 3.17 SAVED BUSINESSES
-- ----------------------------------------------------------------------------
CREATE TABLE saved_businesses (
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    business_id BIGINT NOT NULL REFERENCES businesses(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    PRIMARY KEY (user_id, business_id)
);

CREATE INDEX idx_saved_businesses_business ON saved_businesses(business_id);

-- ----------------------------------------------------------------------------
-- 3.18 FOLLOWED BUSINESSES
-- ----------------------------------------------------------------------------
CREATE TABLE followed_businesses (
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    business_id BIGINT NOT NULL REFERENCES businesses(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    PRIMARY KEY (user_id, business_id)
);

CREATE INDEX idx_followed_businesses_business ON followed_businesses(business_id);

-- ----------------------------------------------------------------------------
-- 3.19 ADMIN ACTIONS (Audit Log)
-- ----------------------------------------------------------------------------
CREATE TABLE admin_actions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    admin_user_id UUID NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    action_type VARCHAR(50) NOT NULL,
    target_entity_type VARCHAR(30) NOT NULL,
    target_entity_id VARCHAR(64) NOT NULL,
    metadata JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_admin_actions_target ON admin_actions(target_entity_type, target_entity_id);
CREATE INDEX idx_admin_actions_admin_time ON admin_actions(admin_user_id, created_at DESC);
