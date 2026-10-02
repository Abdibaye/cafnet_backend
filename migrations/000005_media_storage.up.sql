-- Migration 000005: Media Storage & Normalized Media Model for Tigris

CREATE TABLE IF NOT EXISTS media (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    created_by_user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    business_id BIGINT REFERENCES businesses(id) ON DELETE CASCADE,
    menu_item_id UUID REFERENCES menu_items(id) ON DELETE CASCADE,
    submission_id UUID REFERENCES business_submissions(id) ON DELETE CASCADE,
    media_type VARCHAR(30) NOT NULL,
    object_key VARCHAR(500) NOT NULL UNIQUE,
    content_type VARCHAR(100) NOT NULL,
    file_size BIGINT NOT NULL DEFAULT 0,
    status VARCHAR(20) NOT NULL DEFAULT 'PENDING',
    sort_order INT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT check_media_type CHECK (media_type IN ('BUSINESS_COVER', 'BUSINESS_GALLERY', 'MENU_ITEM')),
    CONSTRAINT check_media_status CHECK (status IN ('PENDING', 'READY', 'FAILED', 'DELETED'))
);

-- Indices for rapid querying by owner entity
CREATE INDEX IF NOT EXISTS idx_media_business_type_sort ON media(business_id, media_type, sort_order) WHERE status = 'READY';
CREATE INDEX IF NOT EXISTS idx_media_menu_item ON media(menu_item_id) WHERE status = 'READY';
CREATE INDEX IF NOT EXISTS idx_media_submission ON media(submission_id) WHERE status = 'READY';
CREATE INDEX IF NOT EXISTS idx_media_creator ON media(created_by_user_id);
CREATE INDEX IF NOT EXISTS idx_media_status ON media(status);

-- Add optional direct foreign key references for fast 1:1 join lookups
ALTER TABLE business_submissions ADD COLUMN IF NOT EXISTS cover_media_id UUID REFERENCES media(id) ON DELETE SET NULL;
ALTER TABLE menu_items ADD COLUMN IF NOT EXISTS image_media_id UUID REFERENCES media(id) ON DELETE SET NULL;

CREATE TRIGGER trg_media_updated_at
BEFORE UPDATE ON media
FOR EACH ROW EXECUTE FUNCTION set_updated_at();
