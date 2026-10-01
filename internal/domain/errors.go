package domain

import (
	"errors"
	"fmt"
)

var (
	// ErrNotFound is returned when an entity does not exist.
	ErrNotFound = errors.New("resource not found")

	// ErrConflict is returned when an operation violates uniqueness invariants.
	ErrConflict = errors.New("resource conflict")

	// ErrInvalidInput is returned when input fields violate domain validation checks.
	ErrInvalidInput = errors.New("invalid input data")

	// ErrConstraintViolation is returned when a foreign key or integrity rule is broken.
	ErrConstraintViolation = errors.New("data constraint violation")

	// ErrIllegalStateTransition is returned when attempting an invalid lifecycle progression.
	ErrIllegalStateTransition = errors.New("illegal state machine transition")

	// ErrUnauthorized is returned when caller lacks required domain permissions.
	ErrUnauthorized = errors.New("unauthorized domain operation")

	// ErrDatabaseUnavailable is returned when the database connection fails or times out.
	ErrDatabaseUnavailable = errors.New("database service unavailable")

	// ErrTransactionFailure is returned when a database transaction fails to commit or aborts.
	ErrTransactionFailure = errors.New("transaction execution failed")

	// Authentication error sentinels
	ErrInvalidOTP         = errors.New("invalid verification code")
	ErrOTPExpired         = errors.New("verification code has expired")
	ErrOTPTooManyAttempts = errors.New("maximum verification attempts exceeded")
	ErrOTPAlreadyUsed     = errors.New("verification code has already been used")
	ErrOTPRequestTooSoon  = errors.New("please wait before requesting another verification code")
	ErrInvalidPhone       = errors.New("invalid phone number format")
	ErrInvalidEmail       = errors.New("invalid email address")
	ErrEmailAlreadyExists = errors.New("an account with this email already exists")
	ErrInvalidCredentials = errors.New("invalid email or password")
	ErrPasswordTooShort   = errors.New("password must be at least 8 characters")
	ErrPasswordTooLong    = errors.New("password must not exceed 128 characters")
	ErrSessionNotFound    = errors.New("session not found")
	ErrSessionExpired     = errors.New("session has expired")
	ErrSessionRevoked     = errors.New("session has been revoked")
	ErrUnauthenticated    = errors.New("unauthenticated access")

	// Authorization error sentinels
	ErrForbidden              = errors.New("access forbidden")
	ErrNotBusinessManager     = errors.New("user is not an authorized manager of this business")
	ErrInsufficientPermission = errors.New("insufficient permission to perform action")
	ErrResourceNotManaged     = errors.New("target resource is not managed by user")
	ErrSelfApprovalForbidden  = errors.New("self-approval of claims or submissions is strictly forbidden")
)

// DomainError provides structured domain error context while supporting errors.Is and errors.As.
type DomainError struct {
	Base    error
	Message string
	Err     error
}

func (e *DomainError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %s (%v)", e.Base.Error(), e.Message, e.Err)
	}
	return fmt.Sprintf("%s: %s", e.Base.Error(), e.Message)
}

func (e *DomainError) Unwrap() error {
	return e.Base
}

// NewDomainError wraps a base sentinel error with a contextual message and underlying error.
func NewDomainError(base error, message string, underlying error) error {
	return &DomainError{
		Base:    base,
		Message: message,
		Err:     underlying,
	}
}
