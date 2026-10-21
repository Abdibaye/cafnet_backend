package handler

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/Abdibaye/cafnet_backend/internal/domain"
	"github.com/Abdibaye/cafnet_backend/internal/http/request"
	"github.com/Abdibaye/cafnet_backend/internal/http/response"
	"github.com/Abdibaye/cafnet_backend/internal/service"
)

// DiscoveryHandler adapts public browsing, search, and profile queries.
type DiscoveryHandler struct {
	discoveryService service.DiscoveryService
}

// NewDiscoveryHandler constructs a DiscoveryHandler.
func NewDiscoveryHandler(discoveryService service.DiscoveryService) *DiscoveryHandler {
	return &DiscoveryHandler{discoveryService: discoveryService}
}

// ListBusinesses handles public directory queries with optional category and area filters.
func (h *DiscoveryHandler) ListBusinesses(w http.ResponseWriter, r *http.Request) {
	limit := request.QueryInt(r, "limit", 20)
	offset := request.QueryInt(r, "offset", 0)

	filter := domain.BusinessFilter{
		Limit:  limit,
		Offset: offset,
	}

	if cat := request.QueryString(r, "category", ""); cat != "" {
		c := domain.BusinessCategory(cat)
		filter.Category = &c
	}
	if hood := request.QueryString(r, "neighborhood", ""); hood != "" {
		filter.Neighborhood = &hood
	}
	if src := request.QueryString(r, "creation_source", ""); src != "" {
		s := domain.CreationSource(src)
		filter.CreationSource = &s
	}
	if cs := request.QueryString(r, "claim_status", ""); cs != "" {
		s := domain.ClaimStatus(cs)
		filter.ClaimStatus = &s
	}

	businesses, err := h.discoveryService.ListBusinesses(r.Context(), filter)
	if err != nil {
		response.Error(w, r, err)
		return
	}

	hasMore := len(businesses) == limit
	response.JSONWithPagination(w, http.StatusOK, businesses, &response.Pagination{
		Limit:   limit,
		Offset:  offset,
		HasMore: hasMore,
	})
}

// GetBusinessProfile returns the aggregated public profile for a single venue.
// Supports both integer ID (e.g. /api/v1/businesses/1) and slug (e.g. /api/v1/businesses/my-cafe).
func (h *DiscoveryHandler) GetBusinessProfile(w http.ResponseWriter, r *http.Request) {
	rawID := strings.TrimSpace(r.PathValue("id"))
	if rawID == "" {
		response.Error(w, r, domain.NewDomainError(domain.ErrInvalidInput, "business identifier is required", nil))
		return
	}

	id, err := strconv.ParseInt(rawID, 10, 64)
	if err == nil {
		profile, err := h.discoveryService.GetPublicBusinessProfile(r.Context(), domain.BusinessID(id))
		if err != nil {
			response.Error(w, r, err)
			return
		}
		response.JSON(w, http.StatusOK, profile)
		return
	}

	b, err := h.discoveryService.GetBusinessBySlug(r.Context(), rawID)
	if err != nil {
		response.Error(w, r, err)
		return
	}

	profile, err := h.discoveryService.GetPublicBusinessProfile(r.Context(), b.ID)
	if err != nil {
		response.Error(w, r, err)
		return
	}

	response.JSON(w, http.StatusOK, profile)
}

// GetBusinessBySlug returns a business matching the URL slug.
func (h *DiscoveryHandler) GetBusinessBySlug(w http.ResponseWriter, r *http.Request) {
	slug := strings.TrimSpace(r.PathValue("slug"))
	if slug == "" {
		response.Error(w, r, domain.NewDomainError(domain.ErrInvalidInput, "slug parameter is required", nil))
		return
	}

	b, err := h.discoveryService.GetBusinessBySlug(r.Context(), slug)
	if err != nil {
		response.Error(w, r, err)
		return
	}

	response.JSON(w, http.StatusOK, b)
}

// Search handles full-text search queries across businesses and menus.
func (h *DiscoveryHandler) Search(w http.ResponseWriter, r *http.Request) {
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	limit := request.QueryInt(r, "limit", 20)
	offset := request.QueryInt(r, "offset", 0)

	if q == "" {
		response.JSONWithPagination(w, http.StatusOK, []domain.SearchResult{}, &response.Pagination{
			Limit:   limit,
			Offset:  offset,
			HasMore: false,
		})
		return
	}

	results, err := h.discoveryService.SearchBusinesses(r.Context(), q, limit, offset)
	if err != nil {
		response.Error(w, r, err)
		return
	}

	hasMore := len(results) == limit
	response.JSONWithPagination(w, http.StatusOK, results, &response.Pagination{
		Limit:   limit,
		Offset:  offset,
		HasMore: hasMore,
	})
}

// SearchFood handles food discovery search returning canonical taxonomy and matching menu offerings.
func (h *DiscoveryHandler) SearchFood(w http.ResponseWriter, r *http.Request) {
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	limit := request.QueryInt(r, "limit", 20)
	offset := request.QueryInt(r, "offset", 0)

	filter := domain.FoodSearchFilter{
		Query:  q,
		Limit:  limit,
		Offset: offset,
	}

	if maxPriceStr := r.URL.Query().Get("max_price"); maxPriceStr != "" {
		if val, err := strconv.ParseFloat(maxPriceStr, 64); err == nil && val >= 0 {
			minor := int64(val * 100)
			filter.MaxPriceETBMinor = &minor
		}
	} else if maxMinorStr := r.URL.Query().Get("max_price_minor"); maxMinorStr != "" {
		if val, err := strconv.ParseInt(maxMinorStr, 10, 64); err == nil && val >= 0 {
			filter.MaxPriceETBMinor = &val
		}
	}

	if minPriceStr := r.URL.Query().Get("min_price"); minPriceStr != "" {
		if val, err := strconv.ParseFloat(minPriceStr, 64); err == nil && val >= 0 {
			minor := int64(val * 100)
			filter.MinPriceETBMinor = &minor
		}
	} else if minMinorStr := r.URL.Query().Get("min_price_minor"); minMinorStr != "" {
		if val, err := strconv.ParseInt(minMinorStr, 10, 64); err == nil && val >= 0 {
			filter.MinPriceETBMinor = &val
		}
	}

	ratingStr := r.URL.Query().Get("min_rating")
	if ratingStr == "" {
		ratingStr = r.URL.Query().Get("rating")
	}
	if ratingStr != "" {
		if val, err := strconv.ParseFloat(ratingStr, 64); err == nil && val >= 0 && val <= 5.0 {
			filter.MinRating = &val
		}
	}

	neighborhood := strings.TrimSpace(r.URL.Query().Get("neighborhood"))
	if neighborhood == "" {
		neighborhood = strings.TrimSpace(r.URL.Query().Get("area"))
	}
	if neighborhood != "" {
		filter.Neighborhood = &neighborhood
	}

	resp, err := h.discoveryService.SearchFoodWithFilter(r.Context(), filter)
	if err != nil {
		response.Error(w, r, err)
		return
	}

	response.JSON(w, http.StatusOK, resp)
}

// GetDish returns canonical dish details including aliases.
func (h *DiscoveryHandler) GetDish(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" {
		response.Error(w, r, domain.NewDomainError(domain.ErrInvalidInput, "dish identifier is required", nil))
		return
	}

	dish, err := h.discoveryService.GetCanonicalDish(r.Context(), domain.DishID(id))
	if err != nil {
		if dBySlug, slugErr := h.discoveryService.GetCanonicalDishBySlug(r.Context(), id); slugErr == nil {
			response.JSON(w, http.StatusOK, dBySlug)
			return
		}
		response.Error(w, r, err)
		return
	}

	response.JSON(w, http.StatusOK, dish)
}

// ListDishes returns canonical dishes list with optional search.
func (h *DiscoveryHandler) ListDishes(w http.ResponseWriter, r *http.Request) {
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	limit := request.QueryInt(r, "limit", 50)
	offset := request.QueryInt(r, "offset", 0)

	dishes, err := h.discoveryService.ListCanonicalDishes(r.Context(), q, limit, offset)
	if err != nil {
		response.Error(w, r, err)
		return
	}

	response.JSON(w, http.StatusOK, dishes)
}
