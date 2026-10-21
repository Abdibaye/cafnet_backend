-- ============================================================================
-- CAFNET POSTGRESQL MIGRATION 000008: ASSISTANT CONVERSATIONS
-- ============================================================================

CREATE TABLE IF NOT EXISTS assistant_sessions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID REFERENCES users(id) ON DELETE SET NULL,
    title VARCHAR(255) NOT NULL DEFAULT 'Food Discovery',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TRIGGER trg_assistant_sessions_updated_at
BEFORE UPDATE ON assistant_sessions
FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE INDEX IF NOT EXISTS idx_assistant_sessions_user_id ON assistant_sessions(user_id) WHERE user_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_assistant_sessions_updated_at ON assistant_sessions(updated_at DESC);

CREATE TABLE IF NOT EXISTS assistant_messages (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    session_id UUID NOT NULL REFERENCES assistant_sessions(id) ON DELETE CASCADE,
    role VARCHAR(20) NOT NULL,
    content TEXT NOT NULL,
    intent VARCHAR(50) NOT NULL DEFAULT 'food_search',
    search_filter JSONB,
    matched_dish_id UUID REFERENCES canonical_dishes(id) ON DELETE SET NULL,
    results_count INT NOT NULL DEFAULT 0,
    follow_up_suggestions JSONB DEFAULT '[]'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_assistant_messages_session_id ON assistant_messages(session_id, created_at ASC);
