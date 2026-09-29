package service

import (
	"context"
	"strings"

	"github.com/Abdibaye/cafnet_backend/internal/domain"
	"github.com/Abdibaye/cafnet_backend/internal/repository"
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
	CategoryID    domain.CategoryID `json:"category_id"`
	Name          string            `json:"name"`
	Description   string            `json:"description"`
	PriceETBMinor int64             `json:"price_etb_minor"` // Santims (1 ETB = 100 Santims)
	IsAvailable   bool              `json:"is_available"`
	SortOrder     int               `json:"sort_order"`
}

// UpdateMenuItemInput defines mutable item fields.
type UpdateMenuItemInput struct {
	CategoryID    *domain.CategoryID `json:"category_id"`
	Name          *string            `json:"name"`
	Description   *string            `json:"description"`
	PriceETBMinor *int64             `json:"price_etb_minor"`
	IsAvailable   *bool              `json:"is_available"`
	SortOrder     *int               `json:"sort_order"`
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
	txMgr        repository.TxManager
}

// NewMenuService constructs a MenuService.
func NewMenuService(
	menuRepo repository.MenuRepository,
	authzService AuthorizationService,
	txMgr repository.TxManager,
) MenuService {
	return &menuService{
		menuRepo:     menuRepo,
		authzService: authzService,
		txMgr:        txMgr,
	}
}

func (s *menuService) GetFullMenu(ctx context.Context, businessID domain.BusinessID) (*domain.BusinessMenu, error) {
	return s.menuRepo.GetFullMenu(ctx, businessID)
}

func (s *menuService) CreateCategory(ctx context.Context, actor *domain.User, businessID domain.BusinessID, input CreateMenuCategoryInput) (*domain.MenuCategory, error) {
	if err := s.authzService.CanManageMenu(ctx, actor, businessID); err != nil {
		return nil, err
	}

	if strings.TrimSpace(input.Name) == "" {
		return nil, domain.ErrInvalidInput
	}

	cat := &domain.MenuCategory{
		BusinessID: businessID,
		Name:       strings.TrimSpace(input.Name),
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

	if input.Name != nil && strings.TrimSpace(*input.Name) != "" {
		cat.Name = strings.TrimSpace(*input.Name)
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

	return s.menuRepo.DeleteCategory(ctx, categoryID)
}

func (s *menuService) ReorderCategories(ctx context.Context, actor *domain.User, businessID domain.BusinessID, categoryIDs []domain.CategoryID) error {
	if err := s.authzService.CanManageMenu(ctx, actor, businessID); err != nil {
		return err
	}

	return s.menuRepo.ReorderCategories(ctx, businessID, categoryIDs)
}

func (s *menuService) CreateItem(ctx context.Context, actor *domain.User, businessID domain.BusinessID, input CreateMenuItemInput) (*domain.MenuItem, error) {
	if err := s.authzService.CanManageMenu(ctx, actor, businessID); err != nil {
		return nil, err
	}

	if strings.TrimSpace(input.Name) == "" || input.PriceETBMinor < 0 {
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
		CategoryID:    input.CategoryID,
		BusinessID:    businessID,
		Name:          strings.TrimSpace(input.Name),
		Description:   input.Description,
		PriceETBMinor: input.PriceETBMinor,
		IsAvailable:   input.IsAvailable,
		SortOrder:     input.SortOrder,
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

	if input.Name != nil && strings.TrimSpace(*input.Name) != "" {
		item.Name = strings.TrimSpace(*input.Name)
	}
	if input.Description != nil {
		item.Description = *input.Description
	}
	if input.PriceETBMinor != nil {
		if *input.PriceETBMinor < 0 {
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

	if err := s.menuRepo.UpdateItem(ctx, item); err != nil {
		return nil, err
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

	return s.menuRepo.ReorderItems(ctx, categoryID, itemIDs)
}
