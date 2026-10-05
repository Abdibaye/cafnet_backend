package service

import (
	"context"
	"strings"

	"github.com/Abdibaye/cafnet_backend/internal/domain"
	"github.com/Abdibaye/cafnet_backend/internal/repository"
	"github.com/Abdibaye/cafnet_backend/internal/storage"
)

// CreateMenuCategoryInput defines the payload for creating a category within a venue menu.
type CreateMenuCategoryInput struct {
	Name      string `json:"name"`
	SortOrder int    `json:"sort_order"`
}

// UpdateMenuCategoryInput defines mutable category fields.
type UpdateMenuCategoryInput struct {
	Name      *string `json:"name"`
	SortOrder *int    `json:"sort_order"`
}

// CreateMenuItemInput defines the payload for creating an individual menu offering.
type CreateMenuItemInput struct {
	CategoryID      domain.CategoryID `json:"category_id"`
	Name            string            `json:"name"`
	Description     string            `json:"description"`
	PriceETBMinor   int64             `json:"price_etb_minor"` // Santims (1 ETB = 100 Santims)
	IsAvailable     bool              `json:"is_available"`
	SortOrder       int               `json:"sort_order"`
	CanonicalDishID *domain.DishID    `json:"canonical_dish_id,omitempty"`
}

// UpdateMenuItemInput defines mutable item fields.
type UpdateMenuItemInput struct {
	CategoryID      *domain.CategoryID `json:"category_id"`
	Name            *string            `json:"name"`
	Description     *string            `json:"description"`
	PriceETBMinor   *int64             `json:"price_etb_minor"`
	IsAvailable     *bool              `json:"is_available"`
	SortOrder       *int               `json:"sort_order"`
	CanonicalDishID *domain.DishID     `json:"canonical_dish_id,omitempty"`
}

// MenuService orchestrates menu category and item lifecycles with ReBAC verification.
type MenuService interface {
	GetFullMenu(ctx context.Context, businessID domain.BusinessID) (*domain.BusinessMenu, error)

	CreateCategory(ctx context.Context, actor *domain.User, businessID domain.BusinessID, input CreateMenuCategoryInput) (*domain.MenuCategory, error)
	UpdateCategory(ctx context.Context, actor *domain.User, businessID domain.BusinessID, categoryID domain.CategoryID, input UpdateMenuCategoryInput) (*domain.MenuCategory, error)
	DeleteCategory(ctx context.Context, actor *domain.User, businessID domain.BusinessID, categoryID domain.CategoryID) error
	ReorderCategories(ctx context.Context, actor *domain.User, businessID domain.BusinessID, categoryIDs []domain.CategoryID) error

	CreateItem(ctx context.Context, actor *domain.User, businessID domain.BusinessID, input CreateMenuItemInput) (*domain.MenuItem, error)
	UpdateItem(ctx context.Context, actor *domain.User, businessID domain.BusinessID, itemID domain.ItemID, input UpdateMenuItemInput) (*domain.MenuItem, error)
	DeleteItem(ctx context.Context, actor *domain.User, businessID domain.BusinessID, itemID domain.ItemID) error
	ReorderItems(ctx context.Context, actor *domain.User, businessID domain.BusinessID, categoryID domain.CategoryID, itemIDs []domain.ItemID) error
}

type menuService struct {
	menuRepo     repository.MenuRepository
	authzService AuthorizationService
	mediaRepo    repository.MediaRepository
	storage      storage.ObjectStorage
	txMgr        repository.TxManager
}

// NewMenuService constructs a MenuService.
func NewMenuService(
	menuRepo repository.MenuRepository,
	authzService AuthorizationService,
	mediaRepo repository.MediaRepository,
	objStorage storage.ObjectStorage,
	txMgr repository.TxManager,
) MenuService {
	return &menuService{
		menuRepo:     menuRepo,
		authzService: authzService,
		mediaRepo:    mediaRepo,
		storage:      objStorage,
		txMgr:        txMgr,
	}
}

func (s *menuService) GetFullMenu(ctx context.Context, businessID domain.BusinessID) (*domain.BusinessMenu, error) {
	menu, err := s.menuRepo.GetFullMenu(ctx, businessID)
	if err != nil {
		return nil, err
	}

	if s.mediaRepo != nil && s.storage != nil && menu != nil {
		for cIdx := range menu.Categories {
			for iIdx := range menu.Categories[cIdx].Items {
				item := &menu.Categories[cIdx].Items[iIdx]
				if item.Image == nil {
					if med, err := s.mediaRepo.GetByMenuItemID(ctx, item.ID); err == nil && med != nil {
						u := s.storage.PublicURL(med.ObjectKey)
						item.Image = &u
					}
				}
			}
		}
	}

	return menu, nil
}

func (s *menuService) CreateCategory(ctx context.Context, actor *domain.User, businessID domain.BusinessID, input CreateMenuCategoryInput) (*domain.MenuCategory, error) {
	if err := s.authzService.CanManageMenu(ctx, actor, businessID); err != nil {
		return nil, err
	}

	trimmedName := strings.TrimSpace(input.Name)
	if trimmedName == "" || len(trimmedName) > 80 {
		return nil, domain.ErrInvalidInput
	}

	cat := &domain.MenuCategory{
		BusinessID: businessID,
		Name:       trimmedName,
		SortOrder:  input.SortOrder,
	}

	if err := s.menuRepo.CreateCategory(ctx, cat); err != nil {
		return nil, err
	}

	return cat, nil
}

func (s *menuService) UpdateCategory(ctx context.Context, actor *domain.User, businessID domain.BusinessID, categoryID domain.CategoryID, input UpdateMenuCategoryInput) (*domain.MenuCategory, error) {
	if err := s.authzService.CanManageMenu(ctx, actor, businessID); err != nil {
		return nil, err
	}

	cat, err := s.menuRepo.GetCategoryByID(ctx, categoryID)
	if err != nil {
		return nil, err
	}

	// IDOR check: ensure category belongs to the target business
	if cat.BusinessID != businessID {
		return nil, domain.ErrResourceNotManaged
	}

	if input.Name != nil {
		trimmed := strings.TrimSpace(*input.Name)
		if trimmed == "" || len(trimmed) > 80 {
			return nil, domain.ErrInvalidInput
		}
		cat.Name = trimmed
	}
	if input.SortOrder != nil {
		cat.SortOrder = *input.SortOrder
	}

	if err := s.menuRepo.UpdateCategory(ctx, cat); err != nil {
		return nil, err
	}

	return cat, nil
}

func (s *menuService) DeleteCategory(ctx context.Context, actor *domain.User, businessID domain.BusinessID, categoryID domain.CategoryID) error {
	if err := s.authzService.CanManageMenu(ctx, actor, businessID); err != nil {
		return err
	}

	cat, err := s.menuRepo.GetCategoryByID(ctx, categoryID)
	if err != nil {
		return err
	}

	if cat.BusinessID != businessID {
		return domain.ErrResourceNotManaged
	}

	return s.txMgr.WithinTransaction(ctx, func(txCtx context.Context) error {
		return s.menuRepo.DeleteCategory(txCtx, categoryID)
	})
}

func (s *menuService) ReorderCategories(ctx context.Context, actor *domain.User, businessID domain.BusinessID, categoryIDs []domain.CategoryID) error {
	if err := s.authzService.CanManageMenu(ctx, actor, businessID); err != nil {
		return err
	}

	return s.txMgr.WithinTransaction(ctx, func(txCtx context.Context) error {
		return s.menuRepo.ReorderCategories(txCtx, businessID, categoryIDs)
	})
}

func (s *menuService) CreateItem(ctx context.Context, actor *domain.User, businessID domain.BusinessID, input CreateMenuItemInput) (*domain.MenuItem, error) {
	if err := s.authzService.CanManageMenu(ctx, actor, businessID); err != nil {
		return nil, err
	}

	trimmedName := strings.TrimSpace(input.Name)
	if trimmedName == "" || len(trimmedName) > 100 || input.PriceETBMinor < 0 || input.PriceETBMinor > 100000000 {
		return nil, domain.ErrInvalidInput
	}

	// Verify target category exists and belongs to this business
	cat, err := s.menuRepo.GetCategoryByID(ctx, input.CategoryID)
	if err != nil {
		return nil, err
	}
	if cat.BusinessID != businessID {
		return nil, domain.ErrResourceNotManaged
	}

	item := &domain.MenuItem{
		CategoryID:      input.CategoryID,
		BusinessID:      businessID,
		Name:            trimmedName,
		Description:     strings.TrimSpace(input.Description),
		PriceETBMinor:   input.PriceETBMinor,
		IsAvailable:     input.IsAvailable,
		SortOrder:       input.SortOrder,
		CanonicalDishID: input.CanonicalDishID,
	}

	if err := s.menuRepo.CreateItem(ctx, item); err != nil {
		return nil, err
	}

	return item, nil
}

func (s *menuService) UpdateItem(ctx context.Context, actor *domain.User, businessID domain.BusinessID, itemID domain.ItemID, input UpdateMenuItemInput) (*domain.MenuItem, error) {
	if err := s.authzService.CanManageMenu(ctx, actor, businessID); err != nil {
		return nil, err
	}

	item, err := s.menuRepo.GetItemByID(ctx, itemID)
	if err != nil {
		return nil, err
	}

	if item.BusinessID != businessID {
		return nil, domain.ErrResourceNotManaged
	}

	if input.CategoryID != nil {
		targetCat, err := s.menuRepo.GetCategoryByID(ctx, *input.CategoryID)
		if err != nil {
			return nil, err
		}
		if targetCat.BusinessID != businessID {
			return nil, domain.ErrResourceNotManaged
		}
		item.CategoryID = *input.CategoryID
	}

	if input.Name != nil {
		trimmed := strings.TrimSpace(*input.Name)
		if trimmed == "" || len(trimmed) > 100 {
			return nil, domain.ErrInvalidInput
		}
		item.Name = trimmed
	}
	if input.Description != nil {
		item.Description = strings.TrimSpace(*input.Description)
	}
	if input.PriceETBMinor != nil {
		if *input.PriceETBMinor < 0 || *input.PriceETBMinor > 100000000 {
			return nil, domain.ErrInvalidInput
		}
		item.PriceETBMinor = *input.PriceETBMinor
	}
	if input.IsAvailable != nil {
		item.IsAvailable = *input.IsAvailable
	}
	if input.SortOrder != nil {
		item.SortOrder = *input.SortOrder
	}
	if input.CanonicalDishID != nil {
		item.CanonicalDishID = input.CanonicalDishID
	}

	if err := s.menuRepo.UpdateItem(ctx, item); err != nil {
		return nil, err
	}

	if s.mediaRepo != nil && s.storage != nil {
		if med, err := s.mediaRepo.GetByMenuItemID(ctx, item.ID); err == nil && med != nil {
			u := s.storage.PublicURL(med.ObjectKey)
			item.Image = &u
		}
	}

	return item, nil
}

func (s *menuService) DeleteItem(ctx context.Context, actor *domain.User, businessID domain.BusinessID, itemID domain.ItemID) error {
	if err := s.authzService.CanManageMenu(ctx, actor, businessID); err != nil {
		return err
	}

	item, err := s.menuRepo.GetItemByID(ctx, itemID)
	if err != nil {
		return err
	}

	if item.BusinessID != businessID {
		return domain.ErrResourceNotManaged
	}

	return s.menuRepo.DeleteItem(ctx, itemID)
}

func (s *menuService) ReorderItems(ctx context.Context, actor *domain.User, businessID domain.BusinessID, categoryID domain.CategoryID, itemIDs []domain.ItemID) error {
	if err := s.authzService.CanManageMenu(ctx, actor, businessID); err != nil {
		return err
	}

	cat, err := s.menuRepo.GetCategoryByID(ctx, categoryID)
	if err != nil {
		return err
	}
	if cat.BusinessID != businessID {
		return domain.ErrResourceNotManaged
	}

	return s.txMgr.WithinTransaction(ctx, func(txCtx context.Context) error {
		return s.menuRepo.ReorderItems(txCtx, categoryID, itemIDs)
	})
}
