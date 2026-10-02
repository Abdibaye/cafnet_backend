package service

import (
	"context"

	"github.com/Abdibaye/cafnet_backend/internal/domain"
	"github.com/Abdibaye/cafnet_backend/internal/repository"
	"github.com/Abdibaye/cafnet_backend/internal/storage"
)

// PublicBusinessProfile composes complete public-facing profile information for frontend consumption.
type PublicBusinessProfile struct {
	Business      *domain.Business        `json:"business"`
	Hours         []domain.BusinessHours  `json:"hours"`
	Images        []domain.BusinessImage  `json:"images"`
	Menu          *domain.BusinessMenu    `json:"menu"`
	ActiveUpdates []domain.BusinessUpdate `json:"active_updates"`
	ReviewSummary *domain.ReviewSummary   `json:"review_summary"`
	RecentReviews []domain.Review         `json:"recent_reviews"`
}

// DiscoveryService coordinates public discovery, search, and profile aggregation.
type DiscoveryService interface {
	ListBusinesses(ctx context.Context, filter domain.BusinessFilter) ([]domain.Business, error)
	GetBusiness(ctx context.Context, id domain.BusinessID) (*domain.Business, error)
	GetBusinessBySlug(ctx context.Context, slug string) (*domain.Business, error)
	SearchBusinesses(ctx context.Context, query string, limit, offset int) ([]domain.SearchResult, error)
	GetPublicBusinessProfile(ctx context.Context, id domain.BusinessID) (*PublicBusinessProfile, error)
}

type discoveryService struct {
	businessRepo repository.BusinessRepository
	menuRepo     repository.MenuRepository
	updateRepo   repository.UpdateRepository
	reviewRepo   repository.ReviewRepository
	mediaRepo    repository.MediaRepository
	storage      storage.ObjectStorage
}

// NewDiscoveryService constructs a DiscoveryService.
func NewDiscoveryService(
	businessRepo repository.BusinessRepository,
	menuRepo repository.MenuRepository,
	updateRepo repository.UpdateRepository,
	reviewRepo repository.ReviewRepository,
	mediaRepo repository.MediaRepository,
	objStorage storage.ObjectStorage,
) DiscoveryService {
	return &discoveryService{
		businessRepo: businessRepo,
		menuRepo:     menuRepo,
		updateRepo:   updateRepo,
		reviewRepo:   reviewRepo,
		mediaRepo:    mediaRepo,
		storage:      objStorage,
	}
}

func (s *discoveryService) enrichBusiness(ctx context.Context, b *domain.Business) {
	if b == nil {
		return
	}
	if s.mediaRepo != nil && s.storage != nil {
		if cover, err := s.mediaRepo.GetBusinessCover(ctx, b.ID); err == nil && cover != nil {
			u := s.storage.PublicURL(cover.ObjectKey)
			b.CoverImage = &u
		}
		if count, err := s.mediaRepo.CountBusinessGallery(ctx, b.ID); err == nil {
			b.GalleryCount = count
		}
	}
}

func (s *discoveryService) ListBusinesses(ctx context.Context, filter domain.BusinessFilter) ([]domain.Business, error) {
	// Public discovery strictly enforces PUBLISHED publication status
	pub := domain.PublicationStatusPublished
	filter.PublicationStatus = &pub

	if filter.Limit <= 0 {
		filter.Limit = 20
	}
	if filter.Limit > 100 {
		filter.Limit = 100
	}

	businesses, err := s.businessRepo.List(ctx, filter)
	if err != nil {
		return nil, err
	}

	for i := range businesses {
		s.enrichBusiness(ctx, &businesses[i])
	}

	return businesses, nil
}

func (s *discoveryService) GetBusiness(ctx context.Context, id domain.BusinessID) (*domain.Business, error) {
	b, err := s.businessRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	// Public discovery hides unpublished venues
	if b.PublicationStatus != domain.PublicationStatusPublished {
		return nil, domain.ErrNotFound
	}

	s.enrichBusiness(ctx, b)
	return b, nil
}

func (s *discoveryService) GetBusinessBySlug(ctx context.Context, slug string) (*domain.Business, error) {
	b, err := s.businessRepo.GetBySlug(ctx, slug)
	if err != nil {
		return nil, err
	}

	if b.PublicationStatus != domain.PublicationStatusPublished {
		return nil, domain.ErrNotFound
	}

	s.enrichBusiness(ctx, b)
	return b, nil
}

func (s *discoveryService) SearchBusinesses(ctx context.Context, query string, limit, offset int) ([]domain.SearchResult, error) {
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	if offset < 0 {
		offset = 0
	}

	results, err := s.businessRepo.Search(ctx, query, limit, offset)
	if err != nil {
		return nil, err
	}

	for i := range results {
		s.enrichBusiness(ctx, &results[i].Business)
	}

	return results, nil
}

func (s *discoveryService) GetPublicBusinessProfile(ctx context.Context, id domain.BusinessID) (*PublicBusinessProfile, error) {
	b, err := s.GetBusiness(ctx, id)
	if err != nil {
		return nil, err
	}

	hours, err := s.businessRepo.GetHours(ctx, id)
	if err != nil {
		hours = []domain.BusinessHours{}
	}

	// Fetch real media from media table first
	var images []domain.BusinessImage
	if s.mediaRepo != nil && s.storage != nil {
		mediaList, err := s.mediaRepo.ListByBusiness(ctx, id)
		if err == nil && len(mediaList) > 0 {
			for _, m := range mediaList {
				images = append(images, domain.BusinessImage{
					ID:         string(m.ID),
					BusinessID: id,
					ImageURL:   s.storage.PublicURL(m.ObjectKey),
					SortOrder:  m.SortOrder,
					CreatedAt:  m.CreatedAt,
				})
			}
		}
	}

	// If no media in media table, fall back to businessRepo.GetImages
	if len(images) == 0 {
		images, err = s.businessRepo.GetImages(ctx, id)
		if err != nil {
			images = []domain.BusinessImage{}
		}
	}

	menu, err := s.menuRepo.GetFullMenu(ctx, id)
	if err != nil {
		menu = &domain.BusinessMenu{BusinessID: id, Categories: []domain.CategoryWithItems{}}
	} else if s.mediaRepo != nil && s.storage != nil {
		// Populate real menu item images
		for cIdx := range menu.Categories {
			for iIdx := range menu.Categories[cIdx].Items {
				item := &menu.Categories[cIdx].Items[iIdx]
				if item.Image == nil {
					if itemMedia, err := s.mediaRepo.GetByMenuItemID(ctx, item.ID); err == nil && itemMedia != nil {
						u := s.storage.PublicURL(itemMedia.ObjectKey)
						item.Image = &u
					}
				}
			}
		}
	}

	updates, err := s.updateRepo.ListActive(ctx, &id, 10, 0)
	if err != nil {
		updates = []domain.BusinessUpdate{}
	}

	summary, err := s.reviewRepo.GetSummary(ctx, id)
	if err != nil {
		summary = &domain.ReviewSummary{BusinessID: id, Distribution: [5]int{}}
	}

	recentReviews, err := s.reviewRepo.ListForBusiness(ctx, id, 5, 0)
	if err != nil {
		recentReviews = []domain.Review{}
	}

	return &PublicBusinessProfile{
		Business:      b,
		Hours:         hours,
		Images:        images,
		Menu:          menu,
		ActiveUpdates: updates,
		ReviewSummary: summary,
		RecentReviews: recentReviews,
	}, nil
}
