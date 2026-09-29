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

	response.JSON(w, http.StatusOK, b)
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
