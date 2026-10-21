package handler

import (
	"net/http"
	"strings"

	"github.com/Abdibaye/cafnet_backend/internal/auth"
	"github.com/Abdibaye/cafnet_backend/internal/domain"
	"github.com/Abdibaye/cafnet_backend/internal/http/request"
	"github.com/Abdibaye/cafnet_backend/internal/http/response"
	"github.com/Abdibaye/cafnet_backend/internal/service"
)

// AssistantHandler adapts conversational AI food discovery HTTP requests.
type AssistantHandler struct {
	assistantService service.AssistantService
}

// NewAssistantHandler constructs an AssistantHandler.
func NewAssistantHandler(assistantService service.AssistantService) *AssistantHandler {
	return &AssistantHandler{assistantService: assistantService}
}

// Chat handles interactive message exchanges with the AI assistant.
func (h *AssistantHandler) Chat(w http.ResponseWriter, r *http.Request) {
	var req domain.AssistantChatRequest
	if err := request.DecodeJSON(r, &req, 16384); err != nil {
		response.Error(w, r, err)
		return
	}

	var userID *domain.UserID
	if user, ok := auth.UserFromContext(r.Context()); ok && user != nil {
		userID = &user.ID
	}

	resp, err := h.assistantService.Chat(r.Context(), userID, req)
	if err != nil {
		response.Error(w, r, err)
		return
	}

	response.JSON(w, http.StatusOK, resp)
}

// GetSession returns conversation history for an assistant session.
func (h *AssistantHandler) GetSession(w http.ResponseWriter, r *http.Request) {
	sessionID := strings.TrimSpace(r.PathValue("id"))
	if sessionID == "" {
		response.Error(w, r, domain.NewDomainError(domain.ErrInvalidInput, "session identifier is required", nil))
		return
	}

	session, err := h.assistantService.GetSession(r.Context(), domain.AssistantSessionID(sessionID))
	if err != nil {
		response.Error(w, r, err)
		return
	}

	response.JSON(w, http.StatusOK, session)
}

// CreateSession initializes a new assistant chat session.
func (h *AssistantHandler) CreateSession(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		Title string `json:"title"`
	}
	_ = request.DecodeJSON(r, &payload, 4096)

	var userID *domain.UserID
	if user, ok := auth.UserFromContext(r.Context()); ok && user != nil {
		userID = &user.ID
	}

	session, err := h.assistantService.CreateSession(r.Context(), userID, payload.Title)
	if err != nil {
		response.Error(w, r, err)
		return
	}

	response.JSON(w, http.StatusCreated, session)
}
