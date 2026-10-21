package repository

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/Abdibaye/cafnet_backend/internal/domain"
	"github.com/Abdibaye/cafnet_backend/pkg/db"
)

type postgresAssistantRepository struct {
	pool *db.Pool
}

// NewAssistantRepository constructs a PostgreSQL backed AssistantRepository.
func NewAssistantRepository(pool *db.Pool) AssistantRepository {
	return &postgresAssistantRepository{pool: pool}
}

func (r *postgresAssistantRepository) CreateSession(ctx context.Context, session *domain.AssistantSession) error {
	conn := GetDB(ctx, r.pool.Pool)

	if strings.TrimSpace(string(session.ID)) == "" {
		session.ID = domain.AssistantSessionID(uuid.NewString())
	}
	if session.Title == "" {
		session.Title = "Food Discovery"
	}
	now := time.Now()
	session.CreatedAt = now
	session.UpdatedAt = now

	var uID *string
	if session.UserID != nil {
		s := string(*session.UserID)
		uID = &s
	}

	sql := `
		INSERT INTO assistant_sessions (id, user_id, title, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, created_at, updated_at;
	`

	err := conn.QueryRow(ctx, sql, session.ID, uID, session.Title, session.CreatedAt, session.UpdatedAt).
		Scan(&session.ID, &session.CreatedAt, &session.UpdatedAt)
	if err != nil {
		return MapPostgresError(err, "AssistantSession")
	}

	return nil
}

func (r *postgresAssistantRepository) GetSessionByID(ctx context.Context, id domain.AssistantSessionID) (*domain.AssistantSession, error) {
	conn := GetDB(ctx, r.pool.Pool)

	sql := `
		SELECT id, user_id, title, created_at, updated_at
		FROM assistant_sessions
		WHERE id = $1;
	`

	var session domain.AssistantSession
	var uID *string

	err := conn.QueryRow(ctx, sql, id).Scan(
		&session.ID,
		&uID,
		&session.Title,
		&session.CreatedAt,
		&session.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, MapPostgresError(err, "AssistantSession")
	}

	if uID != nil {
		userID := domain.UserID(*uID)
		session.UserID = &userID
	}

	// Fetch recent messages
	msgs, err := r.ListMessages(ctx, session.ID, 50)
	if err == nil {
		session.Messages = msgs
	}

	return &session, nil
}

func (r *postgresAssistantRepository) ListSessionsByUser(ctx context.Context, userID domain.UserID, limit, offset int) ([]domain.AssistantSession, error) {
	conn := GetDB(ctx, r.pool.Pool)

	if limit <= 0 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}

	sql := `
		SELECT id, user_id, title, created_at, updated_at
		FROM assistant_sessions
		WHERE user_id = $1
		ORDER BY updated_at DESC
		LIMIT $2 OFFSET $3;
	`

	rows, err := conn.Query(ctx, sql, userID, limit, offset)
	if err != nil {
		return nil, MapPostgresError(err, "AssistantSession")
	}
	defer rows.Close()

	var sessions []domain.AssistantSession
	for rows.Next() {
		var s domain.AssistantSession
		var uID *string
		if err := rows.Scan(&s.ID, &uID, &s.Title, &s.CreatedAt, &s.UpdatedAt); err != nil {
			return nil, err
		}
		if uID != nil {
			uid := domain.UserID(*uID)
			s.UserID = &uid
		}
		sessions = append(sessions, s)
	}

	return sessions, nil
}

func (r *postgresAssistantRepository) DeleteSession(ctx context.Context, id domain.AssistantSessionID) error {
	conn := GetDB(ctx, r.pool.Pool)

	sql := `DELETE FROM assistant_sessions WHERE id = $1;`
	tag, err := conn.Exec(ctx, sql, id)
	if err != nil {
		return MapPostgresError(err, "AssistantSession")
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (r *postgresAssistantRepository) CreateMessage(ctx context.Context, msg *domain.AssistantMessage) error {
	conn := GetDB(ctx, r.pool.Pool)

	if strings.TrimSpace(string(msg.ID)) == "" {
		msg.ID = domain.AssistantMessageID(uuid.NewString())
	}
	if msg.CreatedAt.IsZero() {
		msg.CreatedAt = time.Now()
	}

	var filterJSON []byte
	if msg.SearchFilter != nil {
		filterJSON, _ = json.Marshal(msg.SearchFilter)
	}

	var suggestionsJSON []byte
	if len(msg.FollowUpSuggestions) > 0 {
		suggestionsJSON, _ = json.Marshal(msg.FollowUpSuggestions)
	} else {
		suggestionsJSON = []byte("[]")
	}

	var dID *string
	if msg.MatchedDishID != nil {
		s := string(*msg.MatchedDishID)
		dID = &s
	}

	sql := `
		INSERT INTO assistant_messages (
			id, session_id, role, content, intent, search_filter, matched_dish_id, results_count, follow_up_suggestions, created_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		RETURNING id, created_at;
	`

	err := conn.QueryRow(
		ctx,
		sql,
		msg.ID,
		msg.SessionID,
		msg.Role,
		msg.Content,
		msg.Intent,
		filterJSON,
		dID,
		msg.ResultsCount,
		suggestionsJSON,
		msg.CreatedAt,
	).Scan(&msg.ID, &msg.CreatedAt)
	if err != nil {
		return MapPostgresError(err, "AssistantMessage")
	}

	// Touch session updated_at
	_, _ = conn.Exec(ctx, `UPDATE assistant_sessions SET updated_at = NOW() WHERE id = $1;`, msg.SessionID)

	return nil
}

func (r *postgresAssistantRepository) ListMessages(ctx context.Context, sessionID domain.AssistantSessionID, limit int) ([]domain.AssistantMessage, error) {
	conn := GetDB(ctx, r.pool.Pool)

	if limit <= 0 {
		limit = 50
	}

	sql := `
		SELECT
			id, session_id, role, content, intent, search_filter, matched_dish_id, results_count, follow_up_suggestions, created_at
		FROM assistant_messages
		WHERE session_id = $1
		ORDER BY created_at ASC
		LIMIT $2;
	`

	rows, err := conn.Query(ctx, sql, sessionID, limit)
	if err != nil {
		return nil, MapPostgresError(err, "AssistantMessage")
	}
	defer rows.Close()

	var messages []domain.AssistantMessage
	for rows.Next() {
		var m domain.AssistantMessage
		var filterRaw []byte
		var suggestionsRaw []byte
		var dID *string

		if err := rows.Scan(
			&m.ID,
			&m.SessionID,
			&m.Role,
			&m.Content,
			&m.Intent,
			&filterRaw,
			&dID,
			&m.ResultsCount,
			&suggestionsRaw,
			&m.CreatedAt,
		); err != nil {
			return nil, err
		}

		if len(filterRaw) > 0 {
			var filter domain.FoodSearchFilter
			if err := json.Unmarshal(filterRaw, &filter); err == nil {
				m.SearchFilter = &filter
			}
		}

		if len(suggestionsRaw) > 0 {
			var suggestions []string
			if err := json.Unmarshal(suggestionsRaw, &suggestions); err == nil {
				m.FollowUpSuggestions = suggestions
			}
		}

		if dID != nil {
			dishID := domain.DishID(*dID)
			m.MatchedDishID = &dishID
		}

		messages = append(messages, m)
	}

	return messages, nil
}
