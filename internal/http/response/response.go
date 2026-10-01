package response

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/Abdibaye/cafnet_backend/internal/domain"
)

// Pagination encapsulates standard list pagination metadata.
type Pagination struct {
	Limit   int  `json:"limit"`
	Offset  int  `json:"offset"`
	HasMore bool `json:"has_more"`
}

type dataResponse struct {
	Data       any         `json:"data"`
	Pagination *Pagination `json:"pagination,omitempty"`
}

type errorDetail struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	RequestID string `json:"request_id,omitempty"`
}

type errorEnvelope struct {
	Error errorDetail `json:"error"`
}

// JSON writes a structured JSON success response.
func JSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if data != nil {
		_ = json.NewEncoder(w).Encode(dataResponse{Data: data})
	}
}

// JSONWithPagination writes a structured JSON list response with pagination metadata.
func JSONWithPagination(w http.ResponseWriter, status int, data any, pagination *Pagination) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(dataResponse{
		Data:       data,
		Pagination: pagination,
	})
}

// Error maps domain and application errors into standardized HTTP status codes and machine-readable JSON envelopes.
func Error(w http.ResponseWriter, r *http.Request, err error) {
	status, code, message := MapDomainError(err)

	reqID := ""
	if r != nil {
		reqID = r.Header.Get("X-Request-ID")
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(errorEnvelope{
		Error: errorDetail{
			Code:      code,
			Message:   message,
			RequestID: reqID,
		},
	})
}

// MapDomainError converts domain errors to HTTP status code, machine-readable code, and safe message.
func MapDomainError(err error) (int, string, string) {
	if err == nil {
		return http.StatusOK, "ok", ""
	}

	// 1. Not Found
	if errors.Is(err, domain.ErrNotFound) {
		return http.StatusNotFound, "resource_not_found", "The requested resource was not found"
	}

	// 2. Authentication & Sessions
	if errors.Is(err, domain.ErrInvalidCredentials) {
		return http.StatusUnauthorized, "invalid_credentials", "Invalid email or password"
	}
	if errors.Is(err, domain.ErrEmailAlreadyExists) {
		return http.StatusConflict, "email_already_exists", "An account with this email already exists"
	}
	if errors.Is(err, domain.ErrInvalidEmail) {
		return http.StatusBadRequest, "invalid_input", "Please enter a valid email address"
	}
	if errors.Is(err, domain.ErrPasswordTooShort) {
		return http.StatusBadRequest, "invalid_input", "Password must be at least 8 characters"
	}
	if errors.Is(err, domain.ErrPasswordTooLong) {
		return http.StatusBadRequest, "invalid_input", "Password must not exceed 128 characters"
	}
	if errors.Is(err, domain.ErrUnauthenticated) {
		return http.StatusUnauthorized, "unauthenticated", "Authentication credentials are required"
	}
	if errors.Is(err, domain.ErrSessionNotFound) || errors.Is(err, domain.ErrSessionExpired) {
		return http.StatusUnauthorized, "session_expired", "Your session has expired. Please sign in again"
	}
	if errors.Is(err, domain.ErrSessionRevoked) {
		return http.StatusUnauthorized, "session_revoked", "Session has been revoked"
	}
	if errors.Is(err, domain.ErrInvalidOTP) {
		return http.StatusBadRequest, "invalid_otp", "The verification code entered is incorrect"
	}
	if errors.Is(err, domain.ErrOTPExpired) {
		return http.StatusBadRequest, "otp_expired", "The verification code has expired"
	}
	if errors.Is(err, domain.ErrOTPAlreadyUsed) {
		return http.StatusConflict, "otp_already_used", "This verification code has already been consumed"
	}
	if errors.Is(err, domain.ErrOTPTooManyAttempts) {
		return http.StatusTooManyRequests, "otp_too_many_attempts", "Maximum verification attempts exceeded. Please request a new code"
	}
	if errors.Is(err, domain.ErrOTPRequestTooSoon) {
		return http.StatusTooManyRequests, "rate_limited", "Please wait before requesting another verification code"
	}

	// 3. Authorization & Permissions
	if errors.Is(err, domain.ErrNotBusinessManager) {
		return http.StatusForbidden, "not_business_manager", "You are not an authorized manager of this business"
	}
	if errors.Is(err, domain.ErrResourceNotManaged) {
		return http.StatusForbidden, "resource_not_managed", "You do not have management authority over the specified resource"
	}
	if errors.Is(err, domain.ErrSelfApprovalForbidden) {
		return http.StatusForbidden, "self_approval_forbidden", "Administrative self-approval of claims or submissions is prohibited"
	}
	if errors.Is(err, domain.ErrForbidden) || errors.Is(err, domain.ErrInsufficientPermission) {
		return http.StatusForbidden, "forbidden", "You do not have permission to perform this action"
	}

	// 4. Invalidation & Domain State
	if errors.Is(err, domain.ErrIllegalStateTransition) {
		var domainErr *domain.DomainError
		msg := "Illegal state machine transition"
		if errors.As(err, &domainErr) && domainErr.Message != "" {
			msg = domainErr.Message
		}
		return http.StatusConflict, "illegal_state_transition", msg
	}
	if errors.Is(err, domain.ErrConflict) {
		var domainErr *domain.DomainError
		msg := "Resource conflict"
		if errors.As(err, &domainErr) && domainErr.Message != "" {
			msg = domainErr.Message
		}
		return http.StatusConflict, "conflict", msg
	}
	if errors.Is(err, domain.ErrInvalidInput) || errors.Is(err, domain.ErrInvalidPhone) {
		var domainErr *domain.DomainError
		msg := "Invalid input data"
		if errors.As(err, &domainErr) && domainErr.Message != "" {
			msg = domainErr.Message
		}
		return http.StatusBadRequest, "invalid_input", msg
	}
	if errors.Is(err, domain.ErrConstraintViolation) {
		return http.StatusBadRequest, "constraint_violation", "Data integrity constraint violated"
	}

	// 5. Database Availability
	if errors.Is(err, domain.ErrDatabaseUnavailable) {
		return http.StatusServiceUnavailable, "service_unavailable", "Database service temporarily unavailable"
	}

	// Fallback to internal error without leaking SQL/pgx details
	return http.StatusInternalServerError, "internal_server_error", "An unexpected server error occurred"
}
