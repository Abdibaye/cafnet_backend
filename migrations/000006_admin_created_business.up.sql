-- 000006_admin_created_business.up.sql
-- Adds creation_source to identify whether a business originated from owner submission or admin creation.

ALTER TABLE businesses
ADD COLUMN IF NOT EXISTS creation_source VARCHAR(30) NOT NULL DEFAULT 'OWNER_SUBMISSION';

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'check_business_creation_source'
    ) THEN
        ALTER TABLE businesses
        ADD CONSTRAINT check_business_creation_source
        CHECK (creation_source IN ('OWNER_SUBMISSION', 'ADMIN_CREATED'));
    END IF;
END $$;

CREATE INDEX IF NOT EXISTS idx_businesses_creation_source ON businesses(creation_source);
