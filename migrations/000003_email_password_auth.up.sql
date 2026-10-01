-- ============================================================================
-- CAFNET POSTGRESQL MIGRATION 000003: EMAIL & PASSWORD AUTHENTICATION
-- ============================================================================

-- 1. Make phone nullable for email-only users
ALTER TABLE users ALTER COLUMN phone DROP NOT NULL;

-- 2. Add email and password_hash columns
ALTER TABLE users ADD COLUMN IF NOT EXISTS email VARCHAR(255);
ALTER TABLE users ADD COLUMN IF NOT EXISTS password_hash VARCHAR(255);

-- 3. Enforce email uniqueness across normalized (lowercase) addresses
CREATE UNIQUE INDEX IF NOT EXISTS idx_users_email_unique ON users(LOWER(email)) WHERE email IS NOT NULL AND deleted_at IS NULL;
