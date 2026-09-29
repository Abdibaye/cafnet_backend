package request

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/google/uuid"

	"github.com/Abdibaye/cafnet_backend/internal/domain"
)

// DecodeJSON decodes the JSON request body into the destination struct with body size limits.
func DecodeJSON(r *http.Request, dest any, maxBytes int64) error {
	if r.Body == nil {
		return domain.NewDomainError(domain.ErrInvalidInput, "request body cannot be empty", nil)
	}

	if maxBytes <= 0 {
		maxBytes = 1024 * 1024 // 1 MB default
	}

	r.Body = http.MaxBytesReader(nil, r.Body, maxBytes)

	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()

	if err := dec.Decode(dest); err != nil {
		var maxBytesErr *http.MaxBytesError
		if errors.As(err, &maxBytesErr) {
			return domain.NewDomainError(domain.ErrInvalidInput, "request body exceeds size limit", err)
		}
		if errors.Is(err, io.EOF) {
			return domain.NewDomainError(domain.ErrInvalidInput, "request body cannot be empty", err)
		}
		return domain.NewDomainError(domain.ErrInvalidInput, "malformed JSON in request body", err)
	}

	return nil
}

// QueryInt extracts an integer query parameter or returns defaultVal if missing/invalid.
func QueryInt(r *http.Request, key string, defaultVal int) int {
	valStr := r.URL.Query().Get(key)
	if valStr == "" {
		return defaultVal
	}
	v, err := strconv.Atoi(valStr)
	if err != nil {
		return defaultVal
	}
	return v
}

// QueryString extracts a string query parameter or returns defaultVal if empty.
func QueryString(r *http.Request, key string, defaultVal string) string {
	val := strings.TrimSpace(r.URL.Query().Get(key))
	if val == "" {
		return defaultVal
	}
	return val
}

// PathInt64 extracts an int64 path parameter (such as numeric business ID).
func PathInt64(r *http.Request, key string) (int64, error) {
	val := r.PathValue(key)
	if val == "" {
		return 0, domain.NewDomainError(domain.ErrInvalidInput, "missing required path parameter: "+key, nil)
	}
	id, err := strconv.ParseInt(val, 10, 64)
	if err != nil {
		return 0, domain.NewDomainError(domain.ErrInvalidInput, "invalid numeric path parameter: "+key, err)
	}
	return id, nil
}

// PathUUID extracts and validates a UUID path parameter.
func PathUUID(r *http.Request, key string) (string, error) {
	val := r.PathValue(key)
	if val == "" {
		return "", domain.NewDomainError(domain.ErrInvalidInput, "missing required path parameter: "+key, nil)
	}
	if _, err := uuid.Parse(val); err != nil {
		return "", domain.NewDomainError(domain.ErrInvalidInput, "invalid UUID path parameter: "+key, err)
	}
	return val, nil
}
