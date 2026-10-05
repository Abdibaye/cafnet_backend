-- ============================================================================
-- CAFNET POSTGRESQL MIGRATION 000007: CANONICAL FOOD DISCOVERY TAXONOMY (DOWN)
-- ============================================================================

DROP INDEX IF EXISTS idx_menu_items_canonical_dish;
ALTER TABLE menu_items DROP COLUMN IF EXISTS canonical_dish_id;
DROP TABLE IF EXISTS dish_aliases;
DROP TABLE IF EXISTS canonical_dishes;
