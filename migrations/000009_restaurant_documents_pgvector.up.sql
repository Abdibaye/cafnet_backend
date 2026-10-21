-- ============================================================================
-- CAFNET POSTGRESQL MIGRATION 000009: RESTAURANT DOCUMENTS & PGVECTOR
-- ============================================================================

CREATE EXTENSION IF NOT EXISTS vector;

CREATE TABLE IF NOT EXISTS restaurant_documents (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    business_id BIGINT NOT NULL REFERENCES businesses(id) ON DELETE CASCADE,
    doc_type VARCHAR(50) NOT NULL DEFAULT 'profile',
    title VARCHAR(255) NOT NULL,
    content TEXT NOT NULL,
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    embedding vector(768),
    content_hash VARCHAR(64) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT unique_business_doc_type UNIQUE (business_id, doc_type)
);

CREATE TRIGGER trg_restaurant_documents_updated_at
BEFORE UPDATE ON restaurant_documents
FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE INDEX IF NOT EXISTS idx_restaurant_documents_business_id ON restaurant_documents(business_id);
CREATE INDEX IF NOT EXISTS idx_restaurant_documents_doc_type ON restaurant_documents(doc_type);
CREATE INDEX IF NOT EXISTS idx_restaurant_documents_metadata ON restaurant_documents USING GIN (metadata);
CREATE INDEX IF NOT EXISTS idx_restaurant_documents_embedding ON restaurant_documents USING hnsw (embedding vector_cosine_ops);
