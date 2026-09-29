-- ============================================================================
-- CAFNET POSTGRESQL MIGRATION 000002 ROLLBACK: AUTHENTICATION & SESSION FOUNDATION
-- ============================================================================

DROP TRIGGER IF EXISTS trg_otp_challenges_updated_at ON otp_challenges;
DROP TABLE IF EXISTS otp_challenges;
DROP INDEX IF EXISTS idx_sessions_active;
DROP INDEX IF EXISTS idx_sessions_token_hash;
ALTER TABLE sessions DROP COLUMN IF EXISTS revoked_at;
ALTER TABLE sessions DROP COLUMN IF EXISTS last_used_at;
