package v1

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/orkai-dev/orkai/apps/api/internal/api/middleware"
	"github.com/orkai-dev/orkai/apps/api/internal/apidocs"
	"github.com/orkai-dev/orkai/apps/api/internal/apierr"
	"github.com/orkai-dev/orkai/apps/api/internal/httputil"
	"github.com/orkai-dev/orkai/apps/api/internal/service"
	"github.com/orkai-dev/orkai/apps/api/internal/store"
)

// Swag type resolution for apidocs.* annotation references.
var (
	_ apidocs.ApplicationListResponse
	_ apidocs.MessageResponse
	_ apidocs.ProblemDetail
)

func parseMillicores(raw string) float64 {
	if raw == "" || raw == "0" {
		return 0
	}
	if strings.HasSuffix(raw, "n") {
		v, _ := strconv.ParseFloat(strings.TrimSuffix(raw, "n"), 64)
		return v / 1_000_000
	}
	if strings.HasSuffix(raw, "m") {
		v, _ := strconv.ParseFloat(strings.TrimSuffix(raw, "m"), 64)
		return v
	}
	v, _ := strconv.ParseFloat(raw, 64)
	return v * 1000
}

func parseMiB(raw string) float64 {
	if raw == "" || raw == "0" {
		return 0
	}
	if strings.HasSuffix(raw, "Ki") {
		v, _ := strconv.ParseFloat(strings.TrimSuffix(raw, "Ki"), 64)
		return v / 1024
	}
	if strings.HasSuffix(raw, "Mi") {
		v, _ := strconv.ParseFloat(strings.TrimSuffix(raw, "Mi"), 64)
		return v
	}
	if strings.HasSuffix(raw, "Gi") {
		v, _ := strconv.ParseFloat(strings.TrimSuffix(raw, "Gi"), 64)
		return v * 1024
	}
	v, _ := strconv.ParseFloat(raw, 64)
	return v / (1024 * 1024)
}

type AppHandler struct {
	svc     *service.AppService
	metrics *service.MetricsCollector
	store   store.Store
	authz   *service.Authz
}

func NewAppHandler(svc *service.AppService, metrics *service.MetricsCollector, s store.Store, authz *service.Authz) *AppHandler {
	return &AppHandler{svc: svc, metrics: metrics, store: s, authz: authz}
}

// ListAll godoc
// @Summary      List all applications
// @Tags         apps
// @Produce      json
// @Param        search query string false "Search term"
// @Param        status query string false "Status filter"
// @Param        page     query int false "Page number"
// @Param        per_page query int false "Items per page (max 100)"
// @Success      200 {object} apidocs.ApplicationListResponse
// @Failure      401 {object} apidocs.ProblemDetail
// @Security     BearerAuth
// @Router       /apps [get]
func (h *AppHandler) ListAll(c *gin.Context) {
	params := bindListParams(c)
	filter := store.AppListFilter{
		Search: c.Query("search"),
		Status: c.Query("status"),
	}

	// Members only see apps within their teams' projects.
	ids, isAll, err := h.authz.AccessibleProjectIDs(
		c.Request.Context(),
		middleware.GetUserID(c),
		middleware.GetUserRole(c),
		middleware.GetOrgID(c),
	)
	if err != nil {
		httputil.RespondError(c, err)
		return
	}
	if !isAll {
		filter.ProjectIDs = ids
	}

	apps, total, err := h.svc.ListAll(c.Request.Context(), params, filter)
	if err != nil {
		httputil.RespondError(c, err)
		return
	}
	httputil.RespondOK(c, httputil.NewListResponse(apps, params.Page, params.PerPage, total))
}

// ListByProject godoc
// @Summary      List applications in project
// @Tags         apps
// @Produce      json
// @Param        id path string true "Project ID"
// @Param        page     query int false "Page number"
// @Param        per_page query int false "Items per page (max 100)"
// @Success      200 {object} apidocs.ApplicationListResponse
// @Failure      401 {object} apidocs.ProblemDetail
// @Security     BearerAuth
// @Router       /projects/{id}/apps [get]
func (h *AppHandler) ListByProject(c *gin.Context) {
	projectID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httputil.RespondError(c, apierr.ErrBadRequest.WithDetail("invalid project ID"))
		return
	}

	params := bindListParams(c)
	apps, total, err := h.svc.List(c.Request.Context(), projectID, params)
	if err != nil {
		httputil.RespondError(c, err)
		return
	}

	httputil.RespondOK(c, httputil.NewListResponse(apps, params.Page, params.PerPage, total))
}

// Create godoc
// @Summary      Create application
// @Tags         apps
// @Accept       json
// @Produce      json
// @Param        body body apidocs.CreateAppInput true "Request body"
// @Success      201 {object} apidocs.Application
// @Failure      400 {object} apidocs.ProblemDetail
// @Failure      422 {object} apidocs.ProblemDetail
// @Security     BearerAuth
// @Router       /apps [post]
func (h *AppHandler) Create(c *gin.Context) {
	var input service.CreateAppInput
	if err := c.ShouldBindJSON(&input); err != nil {
		httputil.RespondError(c, apierr.ErrValidation.WithDetail(err.Error()))
		return
	}

	// Verify the caller may create resources in the target project
	if !ensureProjectAccess(c, h.authz, input.ProjectID) {
		return
	}

	app, err := h.svc.Create(c.Request.Context(), input)
	if err != nil {
		httputil.RespondError(c, apierr.ErrBadRequest.WithDetail(err.Error()))
		return
	}

	httputil.RespondCreated(c, app, fmt.Sprintf("/api/v1/apps/%s", app.ID))
}

// Get godoc
// @Summary      Get application
// @Tags         apps
// @Produce      json
// @Param        id path string true "Application ID"
// @Success      200 {object} apidocs.Application
// @Failure      404 {object} apidocs.ProblemDetail
// @Security     BearerAuth
// @Router       /apps/{id} [get]
func (h *AppHandler) Get(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httputil.RespondError(c, apierr.ErrBadRequest.WithDetail("invalid app ID"))
		return
	}
	app, err := h.svc.GetByID(c.Request.Context(), id)
	if err != nil {
		httputil.RespondError(c, apierr.ErrNotFound.WithDetail("application not found"))
		return
	}
	httputil.RespondOK(c, app)
}

// Delete godoc
// @Summary      Delete application
// @Tags         apps
// @Produce      json
// @Param        id path string true "Application ID"
// @Success      204
// @Failure      401 {object} apidocs.ProblemDetail
// @Security     BearerAuth
// @Router       /apps/{id} [delete]
func (h *AppHandler) Delete(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httputil.RespondError(c, apierr.ErrBadRequest.WithDetail("invalid app ID"))
		return
	}

	if err := h.svc.Delete(c.Request.Context(), id); err != nil {
		httputil.RespondError(c, err)
		return
	}

	httputil.RespondNoContent(c)
}

// Scale godoc
// @Summary      Scale application
// @Tags         apps
// @Accept       json
// @Produce      json
// @Param        id path string true "Application ID"
// @Param        body body object true "Request body"
// @Success      200 {object} apidocs.Application
// @Failure      401 {object} apidocs.ProblemDetail
// @Security     BearerAuth
// @Router       /apps/{id}/scale [post]
func (h *AppHandler) Scale(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httputil.RespondError(c, apierr.ErrBadRequest.WithDetail("invalid app ID"))
		return
	}

	var input struct {
		Replicas int32 `json:"replicas" binding:"required,min=0,max=100"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		httputil.RespondError(c, apierr.ErrValidation.WithDetail(err.Error()))
		return
	}

	app, err := h.svc.Scale(c.Request.Context(), id, input.Replicas)
	if err != nil {
		httputil.RespondError(c, apierr.ErrBadRequest.WithDetail(err.Error()))
		return
	}

	httputil.RespondOK(c, app)
}

// UpdateEnv godoc
// @Summary      Update application environment variables
// @Tags         apps
// @Accept       json
// @Produce      json
// @Param        id path string true "Application ID"
// @Param        body body object true "Request body"
// @Success      200 {object} apidocs.Application
// @Failure      401 {object} apidocs.ProblemDetail
// @Security     BearerAuth
// @Router       /apps/{id}/env [put]
func (h *AppHandler) UpdateEnv(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httputil.RespondError(c, apierr.ErrBadRequest.WithDetail("invalid app ID"))
		return
	}
	var input struct {
		EnvVars map[string]string `json:"env_vars" binding:"required"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		httputil.RespondError(c, apierr.ErrValidation.WithDetail(err.Error()))
		return
	}
	app, err := h.svc.UpdateEnvVars(c.Request.Context(), id, input.EnvVars)
	if err != nil {
		httputil.RespondError(c, apierr.ErrBadRequest.WithDetail(err.Error()))
		return
	}
	httputil.RespondOK(c, app)
}

// GetStatus godoc
// @Summary      Get application status
// @Tags         apps
// @Produce      json
// @Param        id path string true "Application ID"
// @Success      200 {object} map[string]interface{}
// @Failure      401 {object} apidocs.ProblemDetail
// @Security     BearerAuth
// @Router       /apps/{id}/status [get]
func (h *AppHandler) GetStatus(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httputil.RespondError(c, apierr.ErrBadRequest.WithDetail("invalid app ID"))
		return
	}
	status, err := h.svc.GetStatus(c.Request.Context(), id)
	if err != nil {
		httputil.RespondError(c, err)
		return
	}
	httputil.RespondOK(c, status)
}

// GetCapabilities godoc
// @Summary      Get application target capabilities
// @Tags         apps
// @Produce      json
// @Param        id path string true "Application ID"
// @Success      200 {object} map[string]interface{}
// @Failure      401 {object} apidocs.ProblemDetail
// @Security     BearerAuth
// @Router       /apps/{id}/capabilities [get]
func (h *AppHandler) GetCapabilities(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httputil.RespondError(c, apierr.ErrBadRequest.WithDetail("invalid app ID"))
		return
	}
	caps, err := h.svc.GetTargetCapabilities(c.Request.Context(), id)
	if err != nil {
		httputil.RespondError(c, err)
		return
	}
	httputil.RespondOK(c, caps)
}

// GetPods godoc
// @Summary      List application pods
// @Tags         apps
// @Produce      json
// @Param        id path string true "Application ID"
// @Success      200 {array} object
// @Failure      401 {object} apidocs.ProblemDetail
// @Security     BearerAuth
// @Router       /apps/{id}/pods [get]
func (h *AppHandler) GetPods(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httputil.RespondError(c, apierr.ErrBadRequest.WithDetail("invalid app ID"))
		return
	}
	pods, err := h.svc.GetPods(c.Request.Context(), id)
	if err != nil {
		httputil.RespondError(c, err)
		return
	}

	// Record real-time metric point for in-memory ring buffer
	if h.metrics != nil && len(pods) > 0 {
		var cpuUsed, cpuLimit, memUsed, memLimit float64
		for _, p := range pods {
			cpuUsed += parseMillicores(p.Resources.CPUUsed)
			cpuLimit += parseMillicores(p.Resources.CPUTotal)
			memUsed += parseMiB(p.Resources.MemUsed)
			memLimit += parseMiB(p.Resources.MemTotal)
		}
		h.metrics.RecordAppMetric(id, service.AppMetricPoint{
			Time:     time.Now(),
			CPUUsed:  cpuUsed,
			CPULimit: cpuLimit,
			MemUsed:  memUsed,
			MemLimit: memLimit,
		})
	}

	httputil.RespondList(c, pods)
}

// GetMetrics godoc
// @Summary      Get application metrics
// @Tags         apps
// @Produce      json
// @Param        id path string true "Application ID"
// @Success      200 {array} object
// @Failure      401 {object} apidocs.ProblemDetail
// @Security     BearerAuth
// @Router       /apps/{id}/metrics [get]
func (h *AppHandler) GetMetrics(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httputil.RespondError(c, apierr.ErrBadRequest.WithDetail("invalid app ID"))
		return
	}
	if h.metrics == nil {
		httputil.RespondOK(c, []struct{}{})
		return
	}
	httputil.RespondOK(c, h.metrics.GetAppMetrics(id))
}

// Restart godoc
// @Summary      Restart application
// @Tags         apps
// @Produce      json
// @Param        id path string true "Application ID"
// @Success      200 {object} apidocs.MessageResponse
// @Failure      401 {object} apidocs.ProblemDetail
// @Security     BearerAuth
// @Router       /apps/{id}/restart [post]
func (h *AppHandler) Restart(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httputil.RespondError(c, apierr.ErrBadRequest.WithDetail("invalid app ID"))
		return
	}
	if err := h.svc.Restart(c.Request.Context(), id); err != nil {
		httputil.RespondError(c, apierr.ErrBadRequest.WithDetail(err.Error()))
		return
	}
	httputil.RespondOK(c, gin.H{"message": "restart triggered"})
}

// ClearBuildCache godoc
// @Summary      Clear application build cache
// @Tags         apps
// @Produce      json
// @Param        id path string true "Application ID"
// @Success      200 {object} apidocs.MessageResponse
// @Failure      401 {object} apidocs.ProblemDetail
// @Security     BearerAuth
// @Router       /apps/{id}/clear-cache [post]
func (h *AppHandler) ClearBuildCache(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httputil.RespondError(c, apierr.ErrBadRequest.WithDetail("invalid app ID"))
		return
	}
	if err := h.svc.ClearBuildCache(c.Request.Context(), id); err != nil {
		httputil.RespondError(c, err)
		return
	}
	httputil.RespondOK(c, gin.H{"message": "build cache cleared"})
}

// Stop godoc
// @Summary      Stop application
// @Tags         apps
// @Produce      json
// @Param        id path string true "Application ID"
// @Success      200 {object} apidocs.MessageResponse
// @Failure      401 {object} apidocs.ProblemDetail
// @Security     BearerAuth
// @Router       /apps/{id}/stop [post]
func (h *AppHandler) Stop(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httputil.RespondError(c, apierr.ErrBadRequest.WithDetail("invalid app ID"))
		return
	}
	if err := h.svc.Stop(c.Request.Context(), id); err != nil {
		httputil.RespondError(c, apierr.ErrBadRequest.WithDetail(err.Error()))
		return
	}
	httputil.RespondOK(c, gin.H{"message": "stopped"})
}

// Update godoc
// @Summary      Update application
// @Tags         apps
// @Accept       json
// @Produce      json
// @Param        id path string true "Application ID"
// @Param        body body apidocs.UpdateAppInput true "Request body"
// @Success      200 {object} apidocs.Application
// @Failure      401 {object} apidocs.ProblemDetail
// @Security     BearerAuth
// @Router       /apps/{id} [patch]
func (h *AppHandler) Update(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httputil.RespondError(c, apierr.ErrBadRequest.WithDetail("invalid app ID"))
		return
	}
	var input service.UpdateAppInput
	if err := c.ShouldBindJSON(&input); err != nil {
		httputil.RespondError(c, apierr.ErrValidation.WithDetail(err.Error()))
		return
	}
	app, err := h.svc.Update(c.Request.Context(), id, input)
	if err != nil {
		httputil.RespondError(c, apierr.ErrBadRequest.WithDetail(err.Error()))
		return
	}
	httputil.RespondOK(c, app)
}

// EnableWebhook godoc
// @Summary      Enable auto-deploy webhook
// @Tags         apps
// @Produce      json
// @Param        id path string true "Application ID"
// @Success      200 {object} map[string]interface{}
// @Failure      401 {object} apidocs.ProblemDetail
// @Security     BearerAuth
// @Router       /apps/{id}/webhook/enable [post]
func (h *AppHandler) EnableWebhook(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httputil.RespondError(c, apierr.ErrBadRequest.WithDetail("invalid app ID"))
		return
	}
	scheme := "http"
	if c.Request.TLS != nil || c.GetHeader("X-Forwarded-Proto") == "https" {
		scheme = "https"
	}
	baseURL := fmt.Sprintf("%s://%s", scheme, c.Request.Host)
	config, err := h.svc.EnableWebhook(c.Request.Context(), id, baseURL)
	if err != nil {
		httputil.RespondError(c, err)
		return
	}
	httputil.RespondOK(c, config)
}

// DisableWebhook godoc
// @Summary      Disable auto-deploy webhook
// @Tags         apps
// @Produce      json
// @Param        id path string true "Application ID"
// @Success      200 {object} map[string]interface{}
// @Failure      401 {object} apidocs.ProblemDetail
// @Security     BearerAuth
// @Router       /apps/{id}/webhook/disable [post]
func (h *AppHandler) DisableWebhook(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httputil.RespondError(c, apierr.ErrBadRequest.WithDetail("invalid app ID"))
		return
	}
	if err := h.svc.DisableWebhook(c.Request.Context(), id); err != nil {
		httputil.RespondError(c, err)
		return
	}
	httputil.RespondOK(c, gin.H{"auto_deploy": false})
}

// RegenerateWebhook godoc
// @Summary      Regenerate webhook secret
// @Tags         apps
// @Produce      json
// @Param        id path string true "Application ID"
// @Success      200 {object} map[string]interface{}
// @Failure      401 {object} apidocs.ProblemDetail
// @Security     BearerAuth
// @Router       /apps/{id}/webhook/regenerate [post]
func (h *AppHandler) RegenerateWebhook(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httputil.RespondError(c, apierr.ErrBadRequest.WithDetail("invalid app ID"))
		return
	}
	scheme := "http"
	if c.Request.TLS != nil || c.GetHeader("X-Forwarded-Proto") == "https" {
		scheme = "https"
	}
	baseURL := fmt.Sprintf("%s://%s", scheme, c.Request.Host)
	config, err := h.svc.RegenerateWebhookSecret(c.Request.Context(), id, baseURL)
	if err != nil {
		httputil.RespondError(c, err)
		return
	}
	httputil.RespondOK(c, config)
}

// GetWebhookConfig godoc
// @Summary      Get webhook configuration
// @Tags         apps
// @Produce      json
// @Param        id path string true "Application ID"
// @Success      200 {object} map[string]interface{}
// @Failure      401 {object} apidocs.ProblemDetail
// @Security     BearerAuth
// @Router       /apps/{id}/webhook [get]
func (h *AppHandler) GetWebhookConfig(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httputil.RespondError(c, apierr.ErrBadRequest.WithDetail("invalid app ID"))
		return
	}
	scheme := "http"
	if c.Request.TLS != nil || c.GetHeader("X-Forwarded-Proto") == "https" {
		scheme = "https"
	}
	baseURL := fmt.Sprintf("%s://%s", scheme, c.Request.Host)
	config, err := h.svc.GetWebhookConfig(c.Request.Context(), id, baseURL)
	if err != nil {
		httputil.RespondError(c, err)
		return
	}
	httputil.RespondOK(c, config)
}

// GetSecrets godoc
// @Summary      List application secret keys
// @Tags         apps
// @Produce      json
// @Param        id path string true "Application ID"
// @Success      200 {object} map[string]interface{}
// @Failure      401 {object} apidocs.ProblemDetail
// @Security     BearerAuth
// @Router       /apps/{id}/secrets [get]
func (h *AppHandler) GetSecrets(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httputil.RespondError(c, apierr.ErrBadRequest.WithDetail("invalid app ID"))
		return
	}
	keys, err := h.svc.GetSecretKeys(c.Request.Context(), id)
	if err != nil {
		httputil.RespondError(c, err)
		return
	}
	httputil.RespondOK(c, keys)
}

// UpdateSecrets godoc
// @Summary      Update application secrets
// @Tags         apps
// @Accept       json
// @Produce      json
// @Param        id path string true "Application ID"
// @Param        body body object true "Request body"
// @Success      200 {object} map[string]interface{}
// @Failure      401 {object} apidocs.ProblemDetail
// @Security     BearerAuth
// @Router       /apps/{id}/secrets [put]
func (h *AppHandler) UpdateSecrets(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httputil.RespondError(c, apierr.ErrBadRequest.WithDetail("invalid app ID"))
		return
	}
	var secrets map[string]string
	if err := c.ShouldBindJSON(&secrets); err != nil {
		httputil.RespondError(c, apierr.ErrValidation.WithDetail(err.Error()))
		return
	}
	keys, err := h.svc.UpdateSecrets(c.Request.Context(), id, secrets)
	if err != nil {
		httputil.RespondError(c, apierr.ErrBadRequest.WithDetail(err.Error()))
		return
	}
	httputil.RespondOK(c, keys)
}

// GetPodEvents godoc
// @Summary      List pod events
// @Tags         apps
// @Produce      json
// @Param        id path string true "Application ID"
// @Param        podName path string true "Pod name"
// @Success      200 {array} object
// @Failure      401 {object} apidocs.ProblemDetail
// @Security     BearerAuth
// @Router       /apps/{id}/pods/{podName}/events [get]
func (h *AppHandler) GetPodEvents(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httputil.RespondError(c, apierr.ErrBadRequest.WithDetail("invalid app ID"))
		return
	}
	podName := c.Param("podName")
	if podName == "" {
		httputil.RespondError(c, apierr.ErrBadRequest.WithDetail("pod name required"))
		return
	}
	events, err := h.svc.GetPodEvents(c.Request.Context(), id, podName)
	if err != nil {
		httputil.RespondError(c, err)
		return
	}
	httputil.RespondList(c, events)
}
