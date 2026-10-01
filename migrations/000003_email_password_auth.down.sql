-- ============================================================================
-- CAFNET POSTGRESQL MIGRATION 000003 ROLLBACK
-- ============================================================================

DROP INDEX IF EXISTS idx_users_email_unique;
ALTER TABLE users DROP COLUMN IF EXISTS password_hash;
ALTER TABLE users DROP COLUMN IF EXISTS email;
ALTER TABLE users ALTER COLUMN phone SET NOT NULL;
