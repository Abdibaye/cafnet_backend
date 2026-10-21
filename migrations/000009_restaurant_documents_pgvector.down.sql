-- ============================================================================
-- CAFNET POSTGRESQL MIGRATION 000009: RESTAURANT DOCUMENTS & PGVECTOR (DOWN)
-- ============================================================================

DROP TRIGGER IF EXISTS trg_restaurant_documents_updated_at ON restaurant_documents;
DROP TABLE IF EXISTS restaurant_documents;
