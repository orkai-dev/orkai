package v1

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/orkai-dev/orkai/apps/api/internal/api/middleware"
	"github.com/orkai-dev/orkai/apps/api/internal/apidocs"
	"github.com/orkai-dev/orkai/apps/api/internal/apierr"
	"github.com/orkai-dev/orkai/apps/api/internal/httputil"
	"github.com/orkai-dev/orkai/apps/api/internal/service"
)

// Swag type resolution for apidocs.* annotation references.
var (
	_ apidocs.ProblemDetail
)

type DNSHandler struct {
	svc *service.ResourceService
}

func NewDNSHandler(svc *service.ResourceService) *DNSHandler {
	return &DNSHandler{svc: svc}
}

// ListZones godoc
// @Summary      List DNS zones
// @Tags         dns
// @Description  Requires admin role.
// @Produce      json
// @Param        id path string true "Resource ID"
// @Success      200 {array} object
// @Failure      403 {object} apidocs.ProblemDetail
// @Security     BearerAuth
// @Router       /resources/{id}/dns/zones [get]
func (h *DNSHandler) ListZones(c *gin.Context) {
	orgID := middleware.GetOrgID(c)
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httputil.RespondError(c, apierr.ErrBadRequest.WithDetail("invalid resource ID"))
		return
	}
	zones, err := h.svc.DNSZones(c.Request.Context(), orgID, id)
	if err != nil {
		httputil.RespondError(c, err)
		return
	}
	if zones == nil {
		zones = []service.DNSZone{}
	}
	httputil.RespondOK(c, zones)
}

// ListRecords godoc
// @Summary      List DNS records
// @Tags         dns
// @Description  Requires admin role.
// @Produce      json
// @Param        id path string true "Resource ID"
// @Param        zone_id query string true "Zone ID"
// @Success      200 {array} object
// @Failure      400 {object} apidocs.ProblemDetail
// @Failure      403 {object} apidocs.ProblemDetail
// @Security     BearerAuth
// @Router       /resources/{id}/dns/records [get]
func (h *DNSHandler) ListRecords(c *gin.Context) {
	orgID := middleware.GetOrgID(c)
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httputil.RespondError(c, apierr.ErrBadRequest.WithDetail("invalid resource ID"))
		return
	}
	zoneID := c.Query("zone_id")
	if zoneID == "" {
		httputil.RespondError(c, apierr.ErrBadRequest.WithDetail("zone_id is required"))
		return
	}
	records, err := h.svc.DNSRecords(c.Request.Context(), orgID, id, zoneID)
	if err != nil {
		httputil.RespondError(c, err)
		return
	}
	if records == nil {
		records = []service.DNSRecord{}
	}
	httputil.RespondOK(c, records)
}

// UpsertRecord godoc
// @Summary      Upsert DNS record
// @Tags         dns
// @Description  Requires admin role.
// @Accept       json
// @Produce      json
// @Param        id path string true "Resource ID"
// @Param        body body apidocs.DNSUpsertRecordInput true "Request body"
// @Success      200 {object} map[string]interface{}
// @Failure      403 {object} apidocs.ProblemDetail
// @Security     BearerAuth
// @Router       /resources/{id}/dns/records [post]
func (h *DNSHandler) UpsertRecord(c *gin.Context) {
	orgID := middleware.GetOrgID(c)
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httputil.RespondError(c, apierr.ErrBadRequest.WithDetail("invalid resource ID"))
		return
	}
	var input service.DNSUpsertRecordInput
	if err := c.ShouldBindJSON(&input); err != nil {
		httputil.RespondError(c, apierr.ErrValidation.WithDetail(err.Error()))
		return
	}
	if err := h.svc.DNSUpsertRecord(c.Request.Context(), orgID, id, input); err != nil {
		httputil.RespondError(c, err)
		return
	}
	httputil.RespondOK(c, gin.H{"success": true})
}

// DeleteRecord godoc
// @Summary      Delete DNS record
// @Tags         dns
// @Description  Requires admin role.
// @Accept       json
// @Produce      json
// @Param        id path string true "Resource ID"
// @Param        body body apidocs.DNSDeleteRecordInput true "Request body"
// @Success      200 {object} map[string]interface{}
// @Failure      403 {object} apidocs.ProblemDetail
// @Security     BearerAuth
// @Router       /resources/{id}/dns/records/delete [post]
func (h *DNSHandler) DeleteRecord(c *gin.Context) {
	orgID := middleware.GetOrgID(c)
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httputil.RespondError(c, apierr.ErrBadRequest.WithDetail("invalid resource ID"))
		return
	}
	var input service.DNSDeleteRecordInput
	if err := c.ShouldBindJSON(&input); err != nil {
		httputil.RespondError(c, apierr.ErrValidation.WithDetail(err.Error()))
		return
	}
	if err := h.svc.DNSDeleteRecord(c.Request.Context(), orgID, id, input); err != nil {
		httputil.RespondError(c, err)
		return
	}
	httputil.RespondOK(c, gin.H{"success": true})
}
