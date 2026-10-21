-- ============================================================================
-- CAFNET POSTGRESQL MIGRATION 000008: ASSISTANT CONVERSATIONS (DOWN)
-- ============================================================================

DROP TABLE IF EXISTS assistant_messages CASCADE;
DROP TRIGGER IF EXISTS trg_assistant_sessions_updated_at ON assistant_sessions;
DROP TABLE IF EXISTS assistant_sessions CASCADE;
