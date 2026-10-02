-- Migration 000005 Down: Remove Media Storage Table

DROP TRIGGER IF EXISTS trg_media_updated_at ON media;
ALTER TABLE business_submissions DROP COLUMN IF EXISTS cover_media_id;
ALTER TABLE menu_items DROP COLUMN IF EXISTS image_media_id;
DROP TABLE IF EXISTS media CASCADE;
