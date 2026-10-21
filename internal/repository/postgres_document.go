package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Abdibaye/cafnet_backend/internal/domain"
)

type postgresDocumentRepo struct {
	pool *pgxpool.Pool
}

// NewPostgresDocumentRepository constructs a RestaurantDocumentRepository with pgvector backing.
func NewPostgresDocumentRepository(pool *pgxpool.Pool) RestaurantDocumentRepository {
	return &postgresDocumentRepo{pool: pool}
}

func (r *postgresDocumentRepo) Upsert(ctx context.Context, doc *domain.RestaurantDocument) error {
	metaJSON, err := json.Marshal(doc.Metadata)
	if err != nil {
		metaJSON = []byte("{}")
	}

	var vecStr *string
	if len(doc.Embedding) > 0 {
		s := vectorToString(doc.Embedding)
		vecStr = &s
	}

	query := `
		INSERT INTO restaurant_documents (
			id, business_id, doc_type, title, content, metadata, embedding, content_hash, updated_at
		) VALUES (
			COALESCE(NULLIF($1, '')::uuid, gen_random_uuid()),
			$2, $3, $4, $5, $6, $7::vector, $8, NOW()
		)
		ON CONFLICT (business_id, doc_type) DO UPDATE SET
			title = EXCLUDED.title,
			content = EXCLUDED.content,
			metadata = EXCLUDED.metadata,
			embedding = EXCLUDED.embedding,
			content_hash = EXCLUDED.content_hash,
			updated_at = NOW()
		RETURNING id, created_at, updated_at
	`

	var idStr string
	err = r.pool.QueryRow(
		ctx,
		query,
		doc.ID,
		doc.BusinessID,
		doc.DocType,
		doc.Title,
		doc.Content,
		metaJSON,
		vecStr,
		doc.ContentHash,
	).Scan(&idStr, &doc.CreatedAt, &doc.UpdatedAt)

	if err != nil {
		return fmt.Errorf("failed to upsert restaurant document: %w", err)
	}
	doc.ID = idStr
	return nil
}

func (r *postgresDocumentRepo) GetByBusinessAndType(ctx context.Context, businessID int64, docType string) (*domain.RestaurantDocument, error) {
	query := `
		SELECT id, business_id, doc_type, title, content, metadata, content_hash, (embedding IS NOT NULL) AS has_embedding, created_at, updated_at
		FROM restaurant_documents
		WHERE business_id = $1 AND doc_type = $2
	`

	var doc domain.RestaurantDocument
	var metaBytes []byte
	var hasEmbedding bool
	err := r.pool.QueryRow(ctx, query, businessID, docType).Scan(
		&doc.ID,
		&doc.BusinessID,
		&doc.DocType,
		&doc.Title,
		&doc.Content,
		&metaBytes,
		&doc.ContentHash,
		&hasEmbedding,
		&doc.CreatedAt,
		&doc.UpdatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get restaurant document: %w", err)
	}

	if hasEmbedding {
		doc.Embedding = make([]float32, 768)
	}

	if len(metaBytes) > 0 {
		_ = json.Unmarshal(metaBytes, &doc.Metadata)
	}
	return &doc, nil
}

func (r *postgresDocumentRepo) ListByBusiness(ctx context.Context, businessID int64) ([]domain.RestaurantDocument, error) {
	query := `
		SELECT id, business_id, doc_type, title, content, metadata, content_hash, created_at, updated_at
		FROM restaurant_documents
		WHERE business_id = $1
		ORDER BY doc_type ASC
	`

	rows, err := r.pool.Query(ctx, query, businessID)
	if err != nil {
		return nil, fmt.Errorf("failed to list restaurant documents: %w", err)
	}
	defer rows.Close()

	var docs []domain.RestaurantDocument
	for rows.Next() {
		var doc domain.RestaurantDocument
		var metaBytes []byte
		if err := rows.Scan(
			&doc.ID,
			&doc.BusinessID,
			&doc.DocType,
			&doc.Title,
			&doc.Content,
			&metaBytes,
			&doc.ContentHash,
			&doc.CreatedAt,
			&doc.UpdatedAt,
		); err != nil {
			return nil, err
		}
		if len(metaBytes) > 0 {
			_ = json.Unmarshal(metaBytes, &doc.Metadata)
		}
		docs = append(docs, doc)
	}
	return docs, nil
}

func (r *postgresDocumentRepo) SearchSimilar(
	ctx context.Context,
	queryEmbedding []float32,
	filter domain.DocumentSearchFilter,
) ([]domain.RetrievedDocument, error) {
	limit := filter.Limit
	if limit <= 0 {
		limit = 5
	}
	if limit > 20 {
		limit = 20
	}

	var vecStr string
	if len(queryEmbedding) > 0 {
		vecStr = vectorToString(queryEmbedding)
	}

	// Dynamic hybrid query builder
	var conditions []string
	var args []any
	argIdx := 1

	if vecStr != "" {
		conditions = append(conditions, "embedding IS NOT NULL")
	}

	if filter.Neighborhood != nil && strings.TrimSpace(*filter.Neighborhood) != "" {
		conditions = append(conditions, fmt.Sprintf("(metadata->>'neighborhood' ILIKE '%%' || $%d || '%%' OR metadata->>'location_name' ILIKE '%%' || $%d || '%%')", argIdx, argIdx))
		args = append(args, strings.TrimSpace(*filter.Neighborhood))
		argIdx++
	}

	if filter.MaxPriceETB != nil && *filter.MaxPriceETB > 0 {
		conditions = append(conditions, fmt.Sprintf("((metadata->>'min_price_etb')::numeric IS NULL OR (metadata->>'min_price_etb')::numeric <= $%d)", argIdx))
		args = append(args, *filter.MaxPriceETB)
		argIdx++
	}

	if filter.MinRating != nil && *filter.MinRating > 0 {
		conditions = append(conditions, fmt.Sprintf("((metadata->>'rating')::numeric IS NOT NULL AND (metadata->>'rating')::numeric >= $%d)", argIdx))
		args = append(args, *filter.MinRating)
		argIdx++
	}

	if filter.DocType != nil && strings.TrimSpace(*filter.DocType) != "" {
		conditions = append(conditions, fmt.Sprintf("doc_type = $%d", argIdx))
		args = append(args, strings.TrimSpace(*filter.DocType))
		argIdx++
	}

	whereClause := ""
	if len(conditions) > 0 {
		whereClause = "WHERE " + strings.Join(conditions, " AND ")
	}

	var query string
	if vecStr != "" {
		// Vector similarity search with cosine distance
		args = append([]any{vecStr}, args...)
		// Adjust placeholder numbers since vecStr is placed at $1
		query = fmt.Sprintf(`
			SELECT 
				id, business_id, doc_type, title, content, metadata, content_hash, created_at, updated_at,
				1 - (embedding <=> $1::vector) AS similarity
			FROM restaurant_documents
			%s
			ORDER BY embedding <=> $1::vector ASC
			LIMIT %d
		`, buildDynamicWhere(conditions), limit)
	} else {
		// Keyword or fallback listing
		query = fmt.Sprintf(`
			SELECT 
				id, business_id, doc_type, title, content, metadata, content_hash, created_at, updated_at,
				1.0 AS similarity
			FROM restaurant_documents
			%s
			ORDER BY updated_at DESC
			LIMIT %d
		`, whereClause, limit)
	}

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to search similar restaurant documents: %w", err)
	}
	defer rows.Close()

	var results []domain.RetrievedDocument
	for rows.Next() {
		var doc domain.RestaurantDocument
		var metaBytes []byte
		var similarity float64

		if err := rows.Scan(
			&doc.ID,
			&doc.BusinessID,
			&doc.DocType,
			&doc.Title,
			&doc.Content,
			&metaBytes,
			&doc.ContentHash,
			&doc.CreatedAt,
			&doc.UpdatedAt,
			&similarity,
		); err != nil {
			return nil, err
		}

		if len(metaBytes) > 0 {
			_ = json.Unmarshal(metaBytes, &doc.Metadata)
		}

		results = append(results, domain.RetrievedDocument{
			Document:   doc,
			Similarity: similarity,
		})
	}

	return results, nil
}

func buildDynamicWhere(conditions []string) string {
	if len(conditions) == 0 {
		return ""
	}
	// Re-index placeholders starting from $2 since $1 is query vector
	var reindexed []string
	argCounter := 2
	for _, c := range conditions {
		if strings.Contains(c, "$") {
			// replace any $X with the current argCounter
			parts := strings.Split(c, "$")
			var sb strings.Builder
			sb.WriteString(parts[0])
			for i := 1; i < len(parts); i++ {
				// skip old digits
				j := 0
				for j < len(parts[i]) && parts[i][j] >= '0' && parts[i][j] <= '9' {
					j++
				}
				sb.WriteString(strconv.Itoa(argCounter))
				sb.WriteString(parts[i][j:])
			}
			argCounter++
			reindexed = append(reindexed, sb.String())
		} else {
			reindexed = append(reindexed, c)
		}
	}
	return "WHERE " + strings.Join(reindexed, " AND ")
}

func (r *postgresDocumentRepo) Count(ctx context.Context) (int, error) {
	var count int
	err := r.pool.QueryRow(ctx, "SELECT COUNT(*) FROM restaurant_documents").Scan(&count)
	return count, err
}

func (r *postgresDocumentRepo) DeleteByBusiness(ctx context.Context, businessID int64) error {
	_, err := r.pool.Exec(ctx, "DELETE FROM restaurant_documents WHERE business_id = $1", businessID)
	return err
}

func vectorToString(v []float32) string {
	var sb strings.Builder
	sb.WriteByte('[')
	for i, val := range v {
		if i > 0 {
			sb.WriteByte(',')
		}
		sb.WriteString(strconv.FormatFloat(float64(val), 'f', 6, 32))
	}
	sb.WriteByte(']')
	return sb.String()
}
