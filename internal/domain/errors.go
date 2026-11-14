package domain

import (
	"errors"
	"fmt"
)

var (
	ErrNotFound          = errors.New("resource not found")
	ErrUnauthorized      = errors.New("unauthorized")
	ErrForbidden         = errors.New("forbidden: insufficient permissions")
	ErrConflict          = errors.New("resource already exists")
	ErrBadRequest        = errors.New("bad request")
	ErrValidation        = errors.New("validation failed")
	ErrTenantSuspended   = errors.New("tenant is suspended")
	ErrUserSuspended     = errors.New("user is suspended in this tenant")
	ErrInvalidToken      = errors.New("invalid or expired token")
	ErrCannotDeleteAdmin = errors.New("cannot delete system admin role")
	ErrCrossTenantAccess = errors.New("cross-tenant access denied: resource not found")
)

type AppError struct {
	Code    string                 `json:"code"`
	Message string                 `json:"message"`
	Status  int                    `json:"status"`
	Details map[string]interface{} `json:"details,omitempty"`
	Err     error                  `json:"-"`
}

func (e *AppError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %v", e.Message, e.Err)
	}
	return e.Message
}

func (e *AppError) Unwrap() error {
	return e.Err
}

func NewAppError(status int, code, message string, err error) *AppError {
	return &AppError{
		Status:  status,
		Code:    code,
		Message: message,
		Err:     err,
	}
}
