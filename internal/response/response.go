package response

import (
	"errors"
	"math"
	"net/http"

	"github.com/gin-gonic/gin"
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

func JSON(c *gin.Context, status int, payload interface{}) {
	c.JSON(status, payload)
}

func Success(c *gin.Context, status int, message string, data interface{}) {
	c.JSON(status, APIResponse{
		Success: true,
		Message: message,
		Data:    data,
	})
}

func Paginated(c *gin.Context, page, limit int, total int64, data interface{}) {
	totalPages := 0
	if limit > 0 {
		totalPages = int(math.Ceil(float64(total) / float64(limit)))
	}
	c.JSON(http.StatusOK, PaginatedResponse{
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

func Error(c *gin.Context, status int, code, message string, details map[string]interface{}) {
	c.JSON(status, ErrorResponse{
		Success: false,
		Error: ErrorDetail{
			Code:    code,
			Message: message,
			Details: details,
		},
	})
}

// FromError converts a Go error or domain.AppError into a standardized Gin HTTP response
func FromError(c *gin.Context, err error) {
	if err == nil {
		return
	}

	var appErr *domain.AppError
	if errors.As(err, &appErr) {
		Error(c, appErr.Status, appErr.Code, appErr.Message, appErr.Details)
		return
	}

	switch {
	case errors.Is(err, domain.ErrNotFound), errors.Is(err, domain.ErrCrossTenantAccess):
		Error(c, http.StatusNotFound, "NOT_FOUND", "resource not found", nil)
	case errors.Is(err, domain.ErrUnauthorized), errors.Is(err, domain.ErrInvalidToken):
		Error(c, http.StatusUnauthorized, "UNAUTHORIZED", err.Error(), nil)
	case errors.Is(err, domain.ErrForbidden), errors.Is(err, domain.ErrTenantSuspended), errors.Is(err, domain.ErrUserSuspended):
		Error(c, http.StatusForbidden, "FORBIDDEN", err.Error(), nil)
	case errors.Is(err, domain.ErrConflict):
		Error(c, http.StatusConflict, "CONFLICT", err.Error(), nil)
	case errors.Is(err, domain.ErrBadRequest), errors.Is(err, domain.ErrCannotDeleteAdmin):
		Error(c, http.StatusBadRequest, "BAD_REQUEST", err.Error(), nil)
	case errors.Is(err, domain.ErrValidation):
		Error(c, http.StatusUnprocessableEntity, "VALIDATION_ERROR", err.Error(), nil)
	default:
		Error(c, http.StatusInternalServerError, "INTERNAL_SERVER_ERROR", "an unexpected error occurred", nil)
	}
}
