package service

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/google/uuid"

	"github.com/Abdibaye/cafnet_backend/internal/config"
	"github.com/Abdibaye/cafnet_backend/internal/domain"
	"github.com/Abdibaye/cafnet_backend/internal/repository"
	"github.com/Abdibaye/cafnet_backend/internal/storage"
)

var (
	ErrGalleryLimitExceeded = errors.New("maximum 20 gallery images allowed per business")
	ErrInvalidTarget        = errors.New("invalid media target entity")
	ErrMediaNotReady        = errors.New("media upload is not finalized")
)

// MediaService provides authoritative media management, presigning, and lifecycle operations.
type MediaService interface {
	CreateUpload(ctx context.Context, actor *domain.User, input domain.CreateUploadInput) (*domain.PresignedUploadResponse, error)
	CompleteUpload(ctx context.Context, actor *domain.User, mediaID domain.MediaID) (*domain.Media, error)
	DeleteMedia(ctx context.Context, actor *domain.User, mediaID domain.MediaID) error
	SetBusinessCover(ctx context.Context, actor *domain.User, businessID domain.BusinessID, mediaID domain.MediaID) error
	ReorderBusinessGallery(ctx context.Context, actor *domain.User, businessID domain.BusinessID, input domain.ReorderMediaInput) error
	GetBusinessMedia(ctx context.Context, businessID domain.BusinessID) ([]domain.BusinessMedia, error)
	GetBusinessCover(ctx context.Context, businessID domain.BusinessID) (*domain.BusinessMedia, error)
	GetSubmissionCover(ctx context.Context, actor *domain.User, submissionID domain.SubmissionID) (*domain.BusinessMedia, error)
	GetMenuItemMedia(ctx context.Context, itemID domain.ItemID) (*domain.BusinessMedia, error)
	DeleteMenuItemMedia(ctx context.Context, actor *domain.User, itemID domain.ItemID) error
}

type mediaService struct {
	cfg          config.StorageConfig
	storage      storage.ObjectStorage
	mediaRepo    repository.MediaRepository
	businessRepo repository.BusinessRepository
	menuRepo     repository.MenuRepository
	subRepo      repository.SubmissionRepository
	authz        AuthorizationService
	txMgr        repository.TxManager
}

// NewMediaService constructs a resource-aware MediaService.
func NewMediaService(
	cfg config.StorageConfig,
	objStorage storage.ObjectStorage,
	mediaRepo repository.MediaRepository,
	businessRepo repository.BusinessRepository,
	menuRepo repository.MenuRepository,
	subRepo repository.SubmissionRepository,
	authz AuthorizationService,
	txMgr repository.TxManager,
) MediaService {
	return &mediaService{
		cfg:          cfg,
		storage:      objStorage,
		mediaRepo:    mediaRepo,
		businessRepo: businessRepo,
		menuRepo:     menuRepo,
		subRepo:      subRepo,
		authz:        authz,
		txMgr:        txMgr,
	}
}

func (s *mediaService) CreateUpload(ctx context.Context, actor *domain.User, input domain.CreateUploadInput) (*domain.PresignedUploadResponse, error) {
	if actor == nil {
		return nil, domain.ErrUnauthenticated
	}

	// 1. File Validation: MIME type
	if err := storage.ValidateMimeType(input.ContentType, s.cfg.AllowedMimeTypes); err != nil {
		return nil, domain.NewDomainError(domain.ErrInvalidInput, err.Error(), err)
	}

	ext, err := storage.MimeToExtension(input.ContentType)
	if err != nil {
		return nil, domain.NewDomainError(domain.ErrInvalidInput, err.Error(), err)
	}

	// 2. File Validation: File Size (default limit: 10MB)
	if err := storage.ValidateFileSize(input.FileSize, s.cfg.MaxUploadSizeBytes); err != nil {
		return nil, domain.NewDomainError(domain.ErrInvalidInput, err.Error(), err)
	}

	mediaID := domain.MediaID(uuid.New().String())

	var objectKey string
	var mediaType domain.MediaType
	var targetBizID *domain.BusinessID
	var targetSubID *domain.SubmissionID
	var targetItemID *domain.ItemID
	var sortOrder int

	switch input.TargetType {
	case domain.TargetTypeSubmission:
		mediaType = domain.MediaTypeBusinessCover
		if input.TargetID != "" {
			subID := domain.SubmissionID(input.TargetID)
			sub, err := s.subRepo.GetByID(ctx, subID)
			if err != nil {
				return nil, err
			}
			if sub.SubmitterUserID != actor.ID && !actor.IsAdmin() {
				return nil, domain.ErrForbidden
			}
			targetSubID = &subID
			objectKey = storage.SubmissionCoverKey(string(subID), string(mediaID), ext)
		} else {
			// Pre-submission upload: deterministic token key
			objectKey = storage.SubmissionCoverKey(string(mediaID), string(mediaID), ext)
		}

	case domain.TargetTypeBusinessCover:
		mediaType = domain.MediaTypeBusinessCover
		if input.TargetID != "" {
			bID, err := strconv.ParseInt(input.TargetID, 10, 64)
			if err != nil {
				return nil, domain.NewDomainError(domain.ErrInvalidInput, "invalid business_id", err)
			}
			bizID := domain.BusinessID(bID)
			if err := s.authz.CanEditBusiness(ctx, actor, bizID); err != nil {
				return nil, err
			}
			targetBizID = &bizID
			objectKey = storage.BusinessCoverKey(bID, string(mediaID), ext)
		} else if actor.IsAdmin() {
			objectKey = fmt.Sprintf("businesses/pending/%s%s", mediaID, ext)
		} else {
			return nil, domain.NewDomainError(domain.ErrInvalidInput, "business_id required", nil)
		}

	case domain.TargetTypeBusinessGallery:
		mediaType = domain.MediaTypeBusinessGallery
		bID, err := strconv.ParseInt(input.TargetID, 10, 64)
		if err != nil {
			return nil, domain.NewDomainError(domain.ErrInvalidInput, "invalid business_id", err)
		}
		bizID := domain.BusinessID(bID)
		if err := s.authz.CanEditBusiness(ctx, actor, bizID); err != nil {
			return nil, err
		}

		// Enforce server-side limit: 20 gallery images
		currentCount, err := s.mediaRepo.CountBusinessGallery(ctx, bizID)
		if err != nil {
			return nil, err
		}
		if currentCount >= 20 {
			return nil, domain.NewDomainError(domain.ErrConflict, ErrGalleryLimitExceeded.Error(), nil)
		}

		targetBizID = &bizID
		sortOrder = currentCount
		objectKey = storage.BusinessGalleryKey(bID, string(mediaID), ext)

	case domain.TargetTypeMenuItem:
		mediaType = domain.MediaTypeMenuItem
		if input.TargetID == "" {
			return nil, domain.NewDomainError(domain.ErrInvalidInput, "menu item_id is required", nil)
		}
		itemID := domain.ItemID(input.TargetID)
		item, err := s.menuRepo.GetItemByID(ctx, itemID)
		if err != nil {
			return nil, err
		}
		if err := s.authz.CanManageMenu(ctx, actor, item.BusinessID); err != nil {
			return nil, err
		}
		targetBizID = &item.BusinessID
		targetItemID = &itemID
		objectKey = storage.MenuItemKey(int64(item.BusinessID), string(itemID), string(mediaID), ext)

	default:
		return nil, domain.NewDomainError(domain.ErrInvalidInput, "unsupported target_type", nil)
	}

	// 3. Generate short-lived presigned upload URL (10 minutes)
	lifetime := s.cfg.PresignedURLExpiration
	if lifetime <= 0 {
		lifetime = 10 * time.Minute
	}

	uploadURL, err := s.storage.CreatePresignedUpload(ctx, objectKey, input.ContentType, input.FileSize, lifetime)
	if err != nil {
		return nil, fmt.Errorf("failed to create presigned upload url: %w", err)
	}

	// 4. Persist media record in PENDING status
	mediaRecord := &domain.Media{
		ID:              mediaID,
		CreatedByUserID: actor.ID,
		BusinessID:      targetBizID,
		MenuItemID:      targetItemID,
		SubmissionID:    targetSubID,
		MediaType:       mediaType,
		ObjectKey:       objectKey,
		ContentType:     input.ContentType,
		FileSize:        input.FileSize,
		Status:          domain.MediaStatusPending,
		SortOrder:       sortOrder,
	}

	if err := s.mediaRepo.Create(ctx, mediaRecord); err != nil {
		return nil, err
	}

	expiresAt := time.Now().Add(lifetime)
	return &domain.PresignedUploadResponse{
		MediaID:   mediaID,
		UploadURL: uploadURL,
		ObjectKey: objectKey,
		ExpiresAt: expiresAt,
	}, nil
}

func (s *mediaService) CompleteUpload(ctx context.Context, actor *domain.User, mediaID domain.MediaID) (*domain.Media, error) {
	if actor == nil {
		return nil, domain.ErrUnauthenticated
	}

	media, err := s.mediaRepo.GetByID(ctx, mediaID)
	if err != nil {
		return nil, err
	}

	// Authorization check: Must be the creator or an authorized manager/admin
	if media.CreatedByUserID != actor.ID && !actor.IsAdmin() {
		return nil, domain.ErrForbidden
	}

	if media.BusinessID != nil {
		if err := s.authz.CanEditBusiness(ctx, actor, *media.BusinessID); err != nil {
			return nil, err
		}
	}

	// Verify object exists in Tigris object storage
	exists, err := s.storage.ObjectExists(ctx, media.ObjectKey)
	if err != nil {
		return nil, fmt.Errorf("storage verification failed: %w", err)
	}
	if !exists {
		return nil, domain.NewDomainError(domain.ErrNotFound, "uploaded object not found in storage; upload may have failed or timed out", nil)
	}

	// Inspect object metadata in storage
	meta, err := s.storage.HeadObject(ctx, media.ObjectKey)
	if err != nil {
		return nil, fmt.Errorf("failed to inspect uploaded object metadata: %w", err)
	}

	actualSize := meta.ContentLength
	if actualSize > s.cfg.MaxUploadSizeBytes {
		// Clean up oversized object
		_ = s.storage.DeleteObject(ctx, media.ObjectKey)
		_ = s.mediaRepo.UpdateStatus(ctx, mediaID, domain.MediaStatusFailed, actualSize)
		return nil, domain.NewDomainError(domain.ErrInvalidInput, "uploaded file exceeds size limit", nil)
	}

	// Atomically mark media READY and update canonical associations
	err = s.txMgr.WithinTransaction(ctx, func(txCtx context.Context) error {
		if err := s.mediaRepo.UpdateStatus(txCtx, mediaID, domain.MediaStatusReady, actualSize); err != nil {
			return err
		}

		// If this is a business cover, set it as the canonical cover
		if media.MediaType == domain.MediaTypeBusinessCover && media.BusinessID != nil {
			if err := s.mediaRepo.SetBusinessCover(txCtx, *media.BusinessID, mediaID); err != nil {
				return err
			}
		}

		// If this is for a menu item, associate it
		if media.MediaType == domain.MediaTypeMenuItem && media.MenuItemID != nil {
			if err := s.mediaRepo.AssociateWithMenuItem(txCtx, mediaID, *media.MenuItemID); err != nil {
				return err
			}
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	media.Status = domain.MediaStatusReady
	media.FileSize = actualSize
	media.URL = s.storage.PublicURL(media.ObjectKey)

	return media, nil
}

func (s *mediaService) DeleteMedia(ctx context.Context, actor *domain.User, mediaID domain.MediaID) error {
	if actor == nil {
		return domain.ErrUnauthenticated
	}

	media, err := s.mediaRepo.GetByID(ctx, mediaID)
	if err != nil {
		return err
	}

	// Authorize derived ownership through ReBAC / creator
	if media.BusinessID != nil {
		if err := s.authz.CanEditBusiness(ctx, actor, *media.BusinessID); err != nil {
			return err
		}
	} else if media.SubmissionID != nil {
		sub, err := s.subRepo.GetByID(ctx, *media.SubmissionID)
		if err != nil {
			return err
		}
		if sub.SubmitterUserID != actor.ID && !actor.IsAdmin() {
			return domain.ErrForbidden
		}
	} else if media.CreatedByUserID != actor.ID && !actor.IsAdmin() {
		return domain.ErrForbidden
	}

	// If this was a business cover, clear cover first
	if media.MediaType == domain.MediaTypeBusinessCover && media.BusinessID != nil {
		_ = s.mediaRepo.ClearBusinessCover(ctx, *media.BusinessID)
	}

	// Delete from storage
	_ = s.storage.DeleteObject(ctx, media.ObjectKey)

	// Delete record from PostgreSQL
	return s.mediaRepo.Delete(ctx, mediaID)
}

func (s *mediaService) SetBusinessCover(ctx context.Context, actor *domain.User, businessID domain.BusinessID, mediaID domain.MediaID) error {
	if actor == nil {
		return domain.ErrUnauthenticated
	}

	if err := s.authz.CanEditBusiness(ctx, actor, businessID); err != nil {
		return err
	}

	return s.mediaRepo.SetBusinessCover(ctx, businessID, mediaID)
}

func (s *mediaService) ReorderBusinessGallery(ctx context.Context, actor *domain.User, businessID domain.BusinessID, input domain.ReorderMediaInput) error {
	if actor == nil {
		return domain.ErrUnauthenticated
	}

	if err := s.authz.CanEditBusiness(ctx, actor, businessID); err != nil {
		return err
	}

	if len(input.MediaIDs) == 0 {
		return nil
	}

	return s.mediaRepo.ReorderBusinessGallery(ctx, businessID, input.MediaIDs)
}

func (s *mediaService) GetBusinessMedia(ctx context.Context, businessID domain.BusinessID) ([]domain.BusinessMedia, error) {
	list, err := s.mediaRepo.ListByBusiness(ctx, businessID)
	if err != nil {
		return nil, err
	}

	var res []domain.BusinessMedia
	for _, m := range list {
		res = append(res, domain.BusinessMedia{
			ID:        m.ID,
			URL:       s.storage.PublicURL(m.ObjectKey),
			Type:      m.MediaType,
			SortOrder: m.SortOrder,
		})
	}
	return res, nil
}

func (s *mediaService) GetBusinessCover(ctx context.Context, businessID domain.BusinessID) (*domain.BusinessMedia, error) {
	cover, err := s.mediaRepo.GetBusinessCover(ctx, businessID)
	if err != nil {
		return nil, err
	}
	if cover == nil {
		return nil, nil
	}

	return &domain.BusinessMedia{
		ID:        cover.ID,
		URL:       s.storage.PublicURL(cover.ObjectKey),
		Type:      cover.MediaType,
		SortOrder: cover.SortOrder,
	}, nil
}

func (s *mediaService) GetSubmissionCover(ctx context.Context, actor *domain.User, submissionID domain.SubmissionID) (*domain.BusinessMedia, error) {
	sub, err := s.subRepo.GetByID(ctx, submissionID)
	if err != nil {
		return nil, err
	}

	if actor == nil || (sub.SubmitterUserID != actor.ID && !actor.IsAdmin()) {
		return nil, domain.ErrForbidden
	}

	cover, err := s.mediaRepo.GetBySubmissionID(ctx, submissionID)
	if err != nil {
		// If not found, return nil without error
		if errors.Is(err, domain.ErrNotFound) {
			return nil, nil
		}
		return nil, err
	}

	return &domain.BusinessMedia{
		ID:        cover.ID,
		URL:       s.storage.PublicURL(cover.ObjectKey),
		Type:      cover.MediaType,
		SortOrder: cover.SortOrder,
	}, nil
}

func (s *mediaService) GetMenuItemMedia(ctx context.Context, itemID domain.ItemID) (*domain.BusinessMedia, error) {
	media, err := s.mediaRepo.GetByMenuItemID(ctx, itemID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return nil, nil
		}
		return nil, err
	}

	return &domain.BusinessMedia{
		ID:        media.ID,
		URL:       s.storage.PublicURL(media.ObjectKey),
		Type:      media.MediaType,
		SortOrder: media.SortOrder,
	}, nil
}

func (s *mediaService) DeleteMenuItemMedia(ctx context.Context, actor *domain.User, itemID domain.ItemID) error {
	if actor == nil {
		return domain.ErrUnauthenticated
	}

	item, err := s.menuRepo.GetItemByID(ctx, itemID)
	if err != nil {
		return err
	}

	if err := s.authz.CanManageMenu(ctx, actor, item.BusinessID); err != nil {
		return err
	}

	media, err := s.mediaRepo.GetByMenuItemID(ctx, itemID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return nil
		}
		return err
	}

	_ = s.storage.DeleteObject(ctx, media.ObjectKey)
	return s.mediaRepo.Delete(ctx, media.ID)
}
