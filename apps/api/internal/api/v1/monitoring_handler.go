package v1

import (
	"github.com/orkai-dev/orkai/apps/api/internal/apidocs"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/orkai-dev/orkai/apps/api/internal/apierr"
	"github.com/orkai-dev/orkai/apps/api/internal/httputil"
	"github.com/orkai-dev/orkai/apps/api/internal/service"
	"github.com/orkai-dev/orkai/apps/api/internal/store"
)

// Swag type resolution for apidocs.* annotation references.
var (
	_ apidocs.MessageResponse
	_ apidocs.ProblemDetail
)

type MonitoringHandler struct {
	metrics *service.MetricsCollector
}

func NewMonitoringHandler(metrics *service.MetricsCollector) *MonitoringHandler {
	return &MonitoringHandler{metrics: metrics}
}

// GetSnapshots godoc
// @Summary      Get metric snapshots
// @Tags         monitoring
// @Produce      json
// @Param        source_type query string false "Source type"
// @Param        source_name query string false "Source name"
// @Param        from query string false "Start time (RFC3339)"
// @Param        to query string false "End time (RFC3339)"
// @Success      200 {object} map[string]interface{}
// @Failure      401 {object} apidocs.ProblemDetail
// @Security     BearerAuth
// @Router       /monitoring/snapshots [get]
func (h *MonitoringHandler) GetSnapshots(c *gin.Context) {
	q := store.SnapshotQuery{
		SourceType: c.Query("source_type"),
		SourceName: c.Query("source_name"),
		Limit:      500,
	}
	if from := c.Query("from"); from != "" {
		if t, err := time.Parse(time.RFC3339, from); err == nil {
			q.From = t
		}
	} else {
		q.From = time.Now().Add(-1 * time.Hour)
	}
	if to := c.Query("to"); to != "" {
		if t, err := time.Parse(time.RFC3339, to); err == nil {
			q.To = t
		}
	}

	data, err := h.metrics.GetSnapshots(c.Request.Context(), q)
	if err != nil {
		httputil.RespondError(c, err)
		return
	}
	httputil.RespondOK(c, data)
}

// GetEvents godoc
// @Summary      List monitoring events
// @Tags         monitoring
// @Produce      json
// @Param        event_type query string false "Event type"
// @Param        namespace query string false "Namespace"
// @Param        from query string false "Start time (RFC3339)"
// @Param        page     query int false "Page number"
// @Param        per_page query int false "Items per page (max 100)"
// @Success      200 {object} object
// @Failure      401 {object} apidocs.ProblemDetail
// @Security     BearerAuth
// @Router       /monitoring/events [get]
func (h *MonitoringHandler) GetEvents(c *gin.Context) {
	q := store.EventQuery{
		EventType:  c.Query("event_type"),
		Namespace:  c.Query("namespace"),
		ListParams: bindListParams(c),
	}
	if from := c.Query("from"); from != "" {
		if t, err := time.Parse(time.RFC3339, from); err == nil {
			q.From = t
		}
	}

	events, total, err := h.metrics.GetEvents(c.Request.Context(), q)
	if err != nil {
		httputil.RespondError(c, err)
		return
	}
	httputil.RespondOK(c, httputil.NewListResponse(events, q.Page, q.PerPage, total))
}

// GetAlerts godoc
// @Summary      List alerts
// @Tags         monitoring
// @Produce      json
// @Param        active query bool false "Active only"
// @Param        severity query string false "Severity filter"
// @Param        page     query int false "Page number"
// @Param        per_page query int false "Items per page (max 100)"
// @Success      200 {object} object
// @Failure      401 {object} apidocs.ProblemDetail
// @Security     BearerAuth
// @Router       /monitoring/alerts [get]
func (h *MonitoringHandler) GetAlerts(c *gin.Context) {
	q := store.AlertQuery{
		ActiveOnly: c.Query("active") == "true",
		Severity:   c.Query("severity"),
		ListParams: bindListParams(c),
	}

	alerts, total, err := h.metrics.GetAlerts(c.Request.Context(), q)
	if err != nil {
		httputil.RespondError(c, err)
		return
	}
	httputil.RespondOK(c, httputil.NewListResponse(alerts, q.Page, q.PerPage, total))
}

// GetActiveAlerts godoc
// @Summary      Get active alerts
// @Tags         monitoring
// @Produce      json
// @Success      200 {object} map[string]interface{}
// @Failure      401 {object} apidocs.ProblemDetail
// @Security     BearerAuth
// @Router       /monitoring/alerts/active [get]
func (h *MonitoringHandler) GetActiveAlerts(c *gin.Context) {
	alerts, err := h.metrics.GetActiveAlerts(c.Request.Context())
	if err != nil {
		httputil.RespondError(c, err)
		return
	}
	httputil.RespondOK(c, gin.H{
		"count":  len(alerts),
		"alerts": alerts,
	})
}

// ResolveAlert godoc
// @Summary      Resolve alert
// @Tags         monitoring
// @Produce      json
// @Param        id path string true "Alert ID"
// @Success      200 {object} apidocs.MessageResponse
// @Failure      401 {object} apidocs.ProblemDetail
// @Security     BearerAuth
// @Router       /monitoring/alerts/{id}/resolve [post]
func (h *MonitoringHandler) ResolveAlert(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httputil.RespondError(c, apierr.ErrBadRequest.WithDetail("invalid alert ID"))
		return
	}
	if err := h.metrics.ResolveAlert(c.Request.Context(), id); err != nil {
		httputil.RespondError(c, err)
		return
	}
	httputil.RespondOK(c, gin.H{"message": "alert resolved"})
}
