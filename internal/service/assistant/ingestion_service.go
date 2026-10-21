package assistant

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Abdibaye/cafnet_backend/internal/domain"
	"github.com/Abdibaye/cafnet_backend/internal/repository"
)

// IngestionService synchronizes restaurant entities into vectorized documents.
type IngestionService interface {
	SyncBusiness(ctx context.Context, businessID int64) (int, error)
	SyncAllBusinesses(ctx context.Context) (total int, updated int, err error)
}

type ingestionService struct {
	pool       *pgxpool.Pool
	docRepo    repository.RestaurantDocumentRepository
	geminiRAG  GeminiRAGClient
}

// NewIngestionService constructs an IngestionService.
func NewIngestionService(
	pool *pgxpool.Pool,
	docRepo repository.RestaurantDocumentRepository,
	geminiRAG GeminiRAGClient,
) IngestionService {
	return &ingestionService{
		pool:      pool,
		docRepo:   docRepo,
		geminiRAG: geminiRAG,
	}
}

func (s *ingestionService) SyncBusiness(ctx context.Context, businessID int64) (int, error) {
	bundle, err := s.fetchBusinessBundle(ctx, businessID)
	if err != nil {
		return 0, err
	}
	if bundle == nil {
		return 0, nil
	}

	docs := CompileRestaurantDocuments(*bundle)
	count := 0

	for i := range docs {
		doc := docs[i]
		existing, err := s.docRepo.GetByBusinessAndType(ctx, doc.BusinessID, doc.DocType)
		if err == nil && existing != nil && existing.ContentHash == doc.ContentHash && len(existing.Embedding) > 0 {
			// Content has not changed and embedding already exists
			continue
		}

		// Compute vector embedding using Gemini
		emb, err := s.geminiRAG.Embed(ctx, doc.Content)
		if err != nil {
			slog.Warn("failed to embed document, using zero/deterministic", "business_id", doc.BusinessID, "doc_type", doc.DocType, "error", err)
		}
		doc.Embedding = emb

		if err := s.docRepo.Upsert(ctx, &doc); err != nil {
			return count, fmt.Errorf("failed to upsert doc %s: %w", doc.DocType, err)
		}
		count++
	}

	return count, nil
}

func (s *ingestionService) SyncAllBusinesses(ctx context.Context) (int, int, error) {
	rows, err := s.pool.Query(ctx, "SELECT id FROM businesses WHERE publication_status = 'PUBLISHED' ORDER BY id ASC")
	if err != nil {
		return 0, 0, fmt.Errorf("failed to list businesses for ingestion: %w", err)
	}
	defer rows.Close()

	var businessIDs []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err == nil {
			businessIDs = append(businessIDs, id)
		}
	}
	rows.Close()

	totalDocs := 0
	updatedDocs := 0

	for idx, bID := range businessIDs {
		n, err := s.SyncBusiness(ctx, bID)
		if err != nil {
			slog.Error("failed to sync business into RAG documents", "business_id", bID, "error", err)
			continue
		}
		totalDocs += n
		updatedDocs += n
		if (idx+1)%50 == 0 || idx+1 == len(businessIDs) {
			slog.Info("ingestion progress", "completed", idx+1, "total", len(businessIDs))
		}
	}

	return totalDocs, updatedDocs, nil
}

func (s *ingestionService) fetchBusinessBundle(ctx context.Context, businessID int64) (*RestaurantProfileBundle, error) {
	// 1. Fetch Business
	bQuery := `
		SELECT 
			id, name, slug, category, cuisine, description, short_description,
			location_name, neighborhood, COALESCE(address_line, ''), phone,
			price_range, tags, payment_methods, reservation_available,
			publication_status, verification_status, claim_status, is_featured,
			created_at, updated_at
		FROM businesses
		WHERE id = $1
	`
	var b domain.Business
	err := s.pool.QueryRow(ctx, bQuery, businessID).Scan(
		&b.ID, &b.Name, &b.Slug, &b.Category, &b.Cuisine, &b.Description, &b.ShortDescription,
		&b.LocationName, &b.Neighborhood, &b.AddressLine, &b.Phone,
		&b.PriceRange, &b.Tags, &b.PaymentMethods, &b.ReservationAvailable,
		&b.PublicationStatus, &b.VerificationStatus, &b.ClaimStatus, &b.IsFeatured,
		&b.CreatedAt, &b.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch business: %w", err)
	}

	// 2. Fetch Hours
	hQuery := `
		SELECT id, business_id, day_of_week, opens_at, closes_at, is_closed
		FROM business_hours
		WHERE business_id = $1
		ORDER BY day_of_week ASC
	`
	hRows, err := s.pool.Query(ctx, hQuery, businessID)
	var hours []domain.BusinessHours
	if err == nil {
		defer hRows.Close()
		for hRows.Next() {
			var h domain.BusinessHours
			if err := hRows.Scan(&h.ID, &h.BusinessID, &h.DayOfWeek, &h.OpensAt, &h.ClosesAt, &h.IsClosed); err == nil {
				hours = append(hours, h)
			}
		}
	}

	// 3. Fetch Menu Items
	mQuery := `
		SELECT id, category_id, business_id, name, description, price_etb_minor, sort_order, is_available, created_at, updated_at
		FROM menu_items
		WHERE business_id = $1 AND deleted_at IS NULL
		ORDER BY sort_order ASC
	`
	mRows, err := s.pool.Query(ctx, mQuery, businessID)
	var items []domain.MenuItem
	if err == nil {
		defer mRows.Close()
		for mRows.Next() {
			var m domain.MenuItem
			if err := mRows.Scan(
				&m.ID, &m.CategoryID, &m.BusinessID, &m.Name, &m.Description,
				&m.PriceETBMinor, &m.SortOrder, &m.IsAvailable, &m.CreatedAt, &m.UpdatedAt,
			); err == nil {
				items = append(items, m)
			}
		}
	}

	// 4. Fetch Rating & Reviews summary
	rQuery := `
		SELECT COALESCE(AVG(rating), 0), COUNT(*)
		FROM reviews
		WHERE business_id = $1 AND status = 'VISIBLE' AND deleted_at IS NULL
	`
	var rating float64
	var reviewCount int
	_ = s.pool.QueryRow(ctx, rQuery, businessID).Scan(&rating, &reviewCount)

	var reviews []domain.Review
	if reviewCount > 0 {
		revRows, err := s.pool.Query(ctx, `
			SELECT rating, review_text, is_verified_visit, created_at
			FROM reviews
			WHERE business_id = $1 AND status = 'VISIBLE' AND deleted_at IS NULL
			ORDER BY created_at DESC
			LIMIT 10
		`, businessID)
		if err == nil {
			defer revRows.Close()
			for revRows.Next() {
				var r domain.Review
				if err := revRows.Scan(&r.Rating, &r.ReviewText, &r.IsVerifiedVisit, &r.CreatedAt); err == nil {
					reviews = append(reviews, r)
				}
			}
		}
	}

	return &RestaurantProfileBundle{
		Business:    b,
		Hours:       hours,
		MenuItems:   items,
		Reviews:     reviews,
		Rating:      rating,
		ReviewCount: reviewCount,
	}, nil
}
