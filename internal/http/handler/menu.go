package handler

import (
	"net/http"

	"github.com/Abdibaye/cafnet_backend/internal/domain"
	"github.com/Abdibaye/cafnet_backend/internal/http/middleware"
	"github.com/Abdibaye/cafnet_backend/internal/http/request"
	"github.com/Abdibaye/cafnet_backend/internal/http/response"
	"github.com/Abdibaye/cafnet_backend/internal/service"
)

// MenuHandler adapts public menu viewing and authorized manager menu editing.
type MenuHandler struct {
	menuService service.MenuService
}

// NewMenuHandler constructs a MenuHandler.
func NewMenuHandler(menuService service.MenuService) *MenuHandler {
	return &MenuHandler{menuService: menuService}
}

// GetFullMenu returns all categories and items for public venue display.
func (h *MenuHandler) GetFullMenu(w http.ResponseWriter, r *http.Request) {
	id, err := request.PathInt64(r, "id")
	if err != nil {
		response.Error(w, r, err)
		return
	}

	menu, err := h.menuService.GetFullMenu(r.Context(), domain.BusinessID(id))
	if err != nil {
		response.Error(w, r, err)
		return
	}

	response.JSON(w, http.StatusOK, menu)
}

// CreateCategory adds a category header to the menu.
func (h *MenuHandler) CreateCategory(w http.ResponseWriter, r *http.Request) {
	actor, err := middleware.MustCurrentUser(r)
	if err != nil {
		response.Error(w, r, err)
		return
	}

	id, err := request.PathInt64(r, "id")
	if err != nil {
		response.Error(w, r, err)
		return
	}

	var input service.CreateMenuCategoryInput
	if err := request.DecodeJSON(r, &input, 4096); err != nil {
		response.Error(w, r, err)
		return
	}

	cat, err := h.menuService.CreateCategory(r.Context(), actor, domain.BusinessID(id), input)
	if err != nil {
		response.Error(w, r, err)
		return
	}

	response.JSON(w, http.StatusCreated, cat)
}

// UpdateCategory updates category name or sort order.
func (h *MenuHandler) UpdateCategory(w http.ResponseWriter, r *http.Request) {
	actor, err := middleware.MustCurrentUser(r)
	if err != nil {
		response.Error(w, r, err)
		return
	}

	id, err := request.PathInt64(r, "id")
	if err != nil {
		response.Error(w, r, err)
		return
	}

	catID, err := request.PathUUID(r, "categoryID")
	if err != nil {
		response.Error(w, r, err)
		return
	}

	var input service.UpdateMenuCategoryInput
	if err := request.DecodeJSON(r, &input, 4096); err != nil {
		response.Error(w, r, err)
		return
	}

	cat, err := h.menuService.UpdateCategory(r.Context(), actor, domain.BusinessID(id), domain.CategoryID(catID), input)
	if err != nil {
		response.Error(w, r, err)
		return
	}

	response.JSON(w, http.StatusOK, cat)
}

// DeleteCategory removes a menu category.
func (h *MenuHandler) DeleteCategory(w http.ResponseWriter, r *http.Request) {
	actor, err := middleware.MustCurrentUser(r)
	if err != nil {
		response.Error(w, r, err)
		return
	}

	id, err := request.PathInt64(r, "id")
	if err != nil {
		response.Error(w, r, err)
		return
	}

	catID, err := request.PathUUID(r, "categoryID")
	if err != nil {
		response.Error(w, r, err)
		return
	}

	if err := h.menuService.DeleteCategory(r.Context(), actor, domain.BusinessID(id), domain.CategoryID(catID)); err != nil {
		response.Error(w, r, err)
		return
	}

	response.JSON(w, http.StatusOK, map[string]bool{"deleted": true})
}

type reorderCategoriesPayload struct {
	CategoryIDs []string `json:"category_ids"`
}

// ReorderCategories updates category ordering.
func (h *MenuHandler) ReorderCategories(w http.ResponseWriter, r *http.Request) {
	actor, err := middleware.MustCurrentUser(r)
	if err != nil {
		response.Error(w, r, err)
		return
	}

	id, err := request.PathInt64(r, "id")
	if err != nil {
		response.Error(w, r, err)
		return
	}

	var payload reorderCategoriesPayload
	if err := request.DecodeJSON(r, &payload, 8192); err != nil {
		response.Error(w, r, err)
		return
	}

	var catIDs []domain.CategoryID
	for _, cid := range payload.CategoryIDs {
		catIDs = append(catIDs, domain.CategoryID(cid))
	}

	if err := h.menuService.ReorderCategories(r.Context(), actor, domain.BusinessID(id), catIDs); err != nil {
		response.Error(w, r, err)
		return
	}

	response.JSON(w, http.StatusOK, map[string]bool{"reordered": true})
}

// CreateItem adds a menu item under a category.
func (h *MenuHandler) CreateItem(w http.ResponseWriter, r *http.Request) {
	actor, err := middleware.MustCurrentUser(r)
	if err != nil {
		response.Error(w, r, err)
		return
	}

	id, err := request.PathInt64(r, "id")
	if err != nil {
		response.Error(w, r, err)
		return
	}

	var input service.CreateMenuItemInput
	if err := request.DecodeJSON(r, &input, 8192); err != nil {
		response.Error(w, r, err)
		return
	}

	item, err := h.menuService.CreateItem(r.Context(), actor, domain.BusinessID(id), input)
	if err != nil {
		response.Error(w, r, err)
		return
	}

	response.JSON(w, http.StatusCreated, item)
}

// UpdateItem updates menu item properties.
func (h *MenuHandler) UpdateItem(w http.ResponseWriter, r *http.Request) {
	actor, err := middleware.MustCurrentUser(r)
	if err != nil {
		response.Error(w, r, err)
		return
	}

	id, err := request.PathInt64(r, "id")
	if err != nil {
		response.Error(w, r, err)
		return
	}

	itemID, err := request.PathUUID(r, "itemID")
	if err != nil {
		response.Error(w, r, err)
		return
	}

	var input service.UpdateMenuItemInput
	if err := request.DecodeJSON(r, &input, 8192); err != nil {
		response.Error(w, r, err)
		return
	}

	item, err := h.menuService.UpdateItem(r.Context(), actor, domain.BusinessID(id), domain.ItemID(itemID), input)
	if err != nil {
		response.Error(w, r, err)
		return
	}

	response.JSON(w, http.StatusOK, item)
}

// DeleteItem removes a menu item.
func (h *MenuHandler) DeleteItem(w http.ResponseWriter, r *http.Request) {
	actor, err := middleware.MustCurrentUser(r)
	if err != nil {
		response.Error(w, r, err)
		return
	}

	id, err := request.PathInt64(r, "id")
	if err != nil {
		response.Error(w, r, err)
		return
	}

	itemID, err := request.PathUUID(r, "itemID")
	if err != nil {
		response.Error(w, r, err)
		return
	}

	if err := h.menuService.DeleteItem(r.Context(), actor, domain.BusinessID(id), domain.ItemID(itemID)); err != nil {
		response.Error(w, r, err)
		return
	}

	response.JSON(w, http.StatusOK, map[string]bool{"deleted": true})
}

type reorderItemsPayload struct {
	ItemIDs []string `json:"item_ids"`
}

// ReorderItems updates sort orders of items within a category.
func (h *MenuHandler) ReorderItems(w http.ResponseWriter, r *http.Request) {
	actor, err := middleware.MustCurrentUser(r)
	if err != nil {
		response.Error(w, r, err)
		return
	}

	id, err := request.PathInt64(r, "id")
	if err != nil {
		response.Error(w, r, err)
		return
	}

	categoryID, err := request.PathUUID(r, "categoryID")
	if err != nil {
		response.Error(w, r, err)
		return
	}

	var payload reorderItemsPayload
	if err := request.DecodeJSON(r, &payload, 8192); err != nil {
		response.Error(w, r, err)
		return
	}

	var itemIDs []domain.ItemID
	for _, iid := range payload.ItemIDs {
		itemIDs = append(itemIDs, domain.ItemID(iid))
	}

	if err := h.menuService.ReorderItems(r.Context(), actor, domain.BusinessID(id), domain.CategoryID(categoryID), itemIDs); err != nil {
		response.Error(w, r, err)
		return
	}

	response.JSON(w, http.StatusOK, map[string]bool{"reordered": true})
}
