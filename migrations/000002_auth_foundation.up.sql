-- ============================================================================
-- CAFNET POSTGRESQL MIGRATION 000002: AUTHENTICATION & SESSION FOUNDATION
-- ============================================================================

-- 1. Extend sessions table with audit & revocation tracking
ALTER TABLE sessions ADD COLUMN IF NOT EXISTS last_used_at TIMESTAMPTZ;
ALTER TABLE sessions ADD COLUMN IF NOT EXISTS revoked_at TIMESTAMPTZ;

CREATE INDEX IF NOT EXISTS idx_sessions_token_hash ON sessions(token_hash);
CREATE INDEX IF NOT EXISTS idx_sessions_active ON sessions(user_id) WHERE revoked_at IS NULL;

-- 2. OTP Challenges Table
CREATE TABLE IF NOT EXISTS otp_challenges (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    phone VARCHAR(20) NOT NULL,
    otp_hash VARCHAR(64) NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    attempts INT NOT NULL DEFAULT 0,
    max_attempts INT NOT NULL DEFAULT 3,
    is_consumed BOOLEAN NOT NULL DEFAULT FALSE,
    consumed_at TIMESTAMPTZ,
    ip_address VARCHAR(45),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TRIGGER trg_otp_challenges_updated_at
BEFORE UPDATE ON otp_challenges
FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE INDEX IF NOT EXISTS idx_otp_challenges_phone_created ON otp_challenges(phone, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_otp_challenges_active ON otp_challenges(id, is_consumed, expires_at);
