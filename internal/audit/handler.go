package audit

import (
	"net/http"
	"strconv"

	"github.com/neurealm/multi-tenant-system/internal/domain"
	"github.com/neurealm/multi-tenant-system/internal/response"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) ListAuditLogs(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := domain.GetTenantID(r.Context())
	if !ok {
		response.FromError(w, domain.ErrUnauthorized)
		return
	}

	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	search := r.URL.Query().Get("search")

	params := domain.PaginationParams{
		Page:   page,
		Limit:  limit,
		Search: search,
	}

	logs, total, err := h.service.ListByTenant(r.Context(), tenantID, params)
	if err != nil {
		response.FromError(w, err)
		return
	}

	response.Paginated(w, params.Page, params.Limit, total, logs)
}
