package handler

import (
	"strconv"

	"github.com/deb151292/neurealm-go-Assignment-/internal/domain"
	"github.com/deb151292/neurealm-go-Assignment-/internal/response"
	"github.com/deb151292/neurealm-go-Assignment-/internal/service"
	"github.com/gin-gonic/gin"
)

type AuditHandler struct {
	service *service.AuditService
}

func NewAuditHandler(service *service.AuditService) *AuditHandler {
	return &AuditHandler{service: service}
}

func (h *AuditHandler) ListAuditLogs(c *gin.Context) {
	tenantID, ok := domain.GetTenantID(c.Request.Context())
	if !ok {
		response.FromError(c, domain.ErrUnauthorized)
		return
	}

	page, _ := strconv.Atoi(c.Query("page"))
	limit, _ := strconv.Atoi(c.Query("limit"))
	search := c.Query("search")

	params := domain.PaginationParams{
		Page:   page,
		Limit:  limit,
		Search: search,
	}

	logs, total, err := h.service.ListByTenant(c.Request.Context(), tenantID, params)
	if err != nil {
		response.FromError(c, err)
		return
	}

	response.Paginated(c, params.Page, params.Limit, total, logs)
}
