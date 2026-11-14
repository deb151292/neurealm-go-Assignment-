package response

import (
	"encoding/json"
	"errors"
	"math"
	"net/http"

	"github.com/deb151292/neurealm-go-Assignment-/internal/domain"
)

type APIResponse struct {
	Success bool        `json:"success"`
	Message string      `json:"message,omitempty"`
	Data    interface{} `json:"data,omitempty"`
}

type PaginationMeta struct {
	Page       int   `json:"page"`
	Limit      int   `json:"limit"`
	Total      int64 `json:"total"`
	TotalPages int   `json:"total_pages"`
}

type PaginatedResponse struct {
	Success    bool           `json:"success"`
	Data       interface{}    `json:"data"`
	Pagination PaginationMeta `json:"pagination"`
}

type ErrorDetail struct {
	Code    string                 `json:"code"`
	Message string                 `json:"message"`
	Details map[string]interface{} `json:"details,omitempty"`
}

type ErrorResponse struct {
	Success bool        `json:"success"`
	Error   ErrorDetail `json:"error"`
}

func JSON(w http.ResponseWriter, status int, payload interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func Success(w http.ResponseWriter, status int, message string, data interface{}) {
	JSON(w, status, APIResponse{
		Success: true,
		Message: message,
		Data:    data,
	})
}

func Paginated(w http.ResponseWriter, page, limit int, total int64, data interface{}) {
	totalPages := 0
	if limit > 0 {
		totalPages = int(math.Ceil(float64(total) / float64(limit)))
	}
	JSON(w, http.StatusOK, PaginatedResponse{
		Success: true,
		Data:    data,
		Pagination: PaginationMeta{
			Page:       page,
			Limit:      limit,
			Total:      total,
			TotalPages: totalPages,
		},
	})
}

func Error(w http.ResponseWriter, status int, code, message string, details map[string]interface{}) {
	JSON(w, status, ErrorResponse{
		Success: false,
		Error: ErrorDetail{
			Code:    code,
			Message: message,
			Details: details,
		},
	})
}

// FromError converts a Go error or domain.AppError into a standardized HTTP response
func FromError(w http.ResponseWriter, err error) {
	if err == nil {
		return
	}

	var appErr *domain.AppError
	if errors.As(err, &appErr) {
		Error(w, appErr.Status, appErr.Code, appErr.Message, appErr.Details)
		return
	}

	switch {
	case errors.Is(err, domain.ErrNotFound), errors.Is(err, domain.ErrCrossTenantAccess):
		Error(w, http.StatusNotFound, "NOT_FOUND", "resource not found", nil)
	case errors.Is(err, domain.ErrUnauthorized), errors.Is(err, domain.ErrInvalidToken):
		Error(w, http.StatusUnauthorized, "UNAUTHORIZED", err.Error(), nil)
	case errors.Is(err, domain.ErrForbidden), errors.Is(err, domain.ErrTenantSuspended), errors.Is(err, domain.ErrUserSuspended):
		Error(w, http.StatusForbidden, "FORBIDDEN", err.Error(), nil)
	case errors.Is(err, domain.ErrConflict):
		Error(w, http.StatusConflict, "CONFLICT", err.Error(), nil)
	case errors.Is(err, domain.ErrBadRequest), errors.Is(err, domain.ErrCannotDeleteAdmin):
		Error(w, http.StatusBadRequest, "BAD_REQUEST", err.Error(), nil)
	case errors.Is(err, domain.ErrValidation):
		Error(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", err.Error(), nil)
	default:
		Error(w, http.StatusInternalServerError, "INTERNAL_SERVER_ERROR", "an unexpected error occurred", nil)
	}
}
