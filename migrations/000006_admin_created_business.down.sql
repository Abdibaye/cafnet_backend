-- 000006_admin_created_business.down.sql
DROP INDEX IF EXISTS idx_businesses_creation_source;
ALTER TABLE businesses DROP CONSTRAINT IF EXISTS check_business_creation_source;
ALTER TABLE businesses DROP COLUMN IF EXISTS creation_source;
