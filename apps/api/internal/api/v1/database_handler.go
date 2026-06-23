package v1

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/orkai-dev/orkai/apps/api/internal/api/middleware"
	"github.com/orkai-dev/orkai/apps/api/internal/apidocs"
	"github.com/orkai-dev/orkai/apps/api/internal/apierr"
	"github.com/orkai-dev/orkai/apps/api/internal/httputil"
	"github.com/orkai-dev/orkai/apps/api/internal/model"
	"github.com/orkai-dev/orkai/apps/api/internal/service"
	"github.com/orkai-dev/orkai/apps/api/internal/store"
)

// Swag type resolution for apidocs.* annotation references.
var (
	_ apidocs.DatabaseListResponse
	_ apidocs.MessageResponse
	_ apidocs.ProblemDetail
)

type DatabaseHandler struct {
	svc   *service.DatabaseService
	store store.Store
	authz *service.Authz
}

func NewDatabaseHandler(svc *service.DatabaseService, s store.Store, authz *service.Authz) *DatabaseHandler {
	return &DatabaseHandler{svc: svc, store: s, authz: authz}
}

// dbErr wraps a service error as a ProblemDetail if it isn't one already.
func dbErr(err error) error {
	if _, ok := err.(*apierr.ProblemDetail); ok {
		return err
	}
	return apierr.ErrBadRequest.WithDetail(err.Error())
}

// ListByProject godoc
// @Summary      List databases in project
// @Tags         databases
// @Produce      json
// @Param        id path string true "Project ID"
// @Param        page     query int false "Page number"
// @Param        per_page query int false "Items per page (max 100)"
// @Success      200 {object} apidocs.DatabaseListResponse
// @Failure      401 {object} apidocs.ProblemDetail
// @Security     BearerAuth
// @Router       /projects/{id}/databases [get]
func (h *DatabaseHandler) ListByProject(c *gin.Context) {
	projectID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httputil.RespondError(c, apierr.ErrBadRequest.WithDetail("invalid project ID"))
		return
	}

	params := bindListParams(c)
	dbs, total, err := h.svc.List(c.Request.Context(), projectID, params)
	if err != nil {
		httputil.RespondError(c, dbErr(err))
		return
	}

	httputil.RespondOK(c, httputil.NewListResponse(dbs, params.Page, params.PerPage, total))
}

// ListAll godoc
// @Summary      List all databases
// @Tags         databases
// @Produce      json
// @Param        search query string false "Search term"
// @Param        status query string false "Status filter"
// @Param        page     query int false "Page number"
// @Param        per_page query int false "Items per page (max 100)"
// @Success      200 {object} apidocs.DatabaseListResponse
// @Failure      401 {object} apidocs.ProblemDetail
// @Security     BearerAuth
// @Router       /databases [get]
func (h *DatabaseHandler) ListAll(c *gin.Context) {
	params := bindListParams(c)
	filter := store.DatabaseListFilter{
		Search: c.Query("search"),
		Status: c.Query("status"),
	}

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

	dbs, total, err := h.svc.ListAll(c.Request.Context(), params, filter)
	if err != nil {
		httputil.RespondError(c, dbErr(err))
		return
	}

	httputil.RespondOK(c, httputil.NewListResponse(dbs, params.Page, params.PerPage, total))
}

// Create godoc
// @Summary      Create database
// @Tags         databases
// @Accept       json
// @Produce      json
// @Param        body body apidocs.CreateDatabaseInput true "Request body"
// @Success      201 {object} apidocs.ManagedDatabase
// @Failure      422 {object} apidocs.ProblemDetail
// @Security     BearerAuth
// @Router       /databases [post]
func (h *DatabaseHandler) Create(c *gin.Context) {
	var input service.CreateDatabaseInput
	if err := c.ShouldBindJSON(&input); err != nil {
		httputil.RespondError(c, apierr.ErrValidation.WithDetail(err.Error()))
		return
	}

	// Verify the caller may create resources in the target project
	if !ensureProjectAccess(c, h.authz, input.ProjectID) {
		return
	}

	db, err := h.svc.Create(c.Request.Context(), input)
	if err != nil {
		httputil.RespondError(c, dbErr(err))
		return
	}

	httputil.RespondCreated(c, db, fmt.Sprintf("/api/v1/databases/%s", db.ID))
}

// Get godoc
// @Summary      Get database
// @Tags         databases
// @Produce      json
// @Param        id path string true "Database ID"
// @Success      200 {object} apidocs.ManagedDatabase
// @Failure      404 {object} apidocs.ProblemDetail
// @Security     BearerAuth
// @Router       /databases/{id} [get]
func (h *DatabaseHandler) Get(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httputil.RespondError(c, apierr.ErrBadRequest.WithDetail("invalid database ID"))
		return
	}

	db, err := h.svc.GetByID(c.Request.Context(), id)
	if err != nil {
		httputil.RespondError(c, apierr.ErrNotFound.WithDetail("database not found"))
		return
	}

	httputil.RespondOK(c, db)
}

// Delete godoc
// @Summary      Delete database
// @Tags         databases
// @Produce      json
// @Param        id path string true "Database ID"
// @Success      204
// @Failure      401 {object} apidocs.ProblemDetail
// @Security     BearerAuth
// @Router       /databases/{id} [delete]
func (h *DatabaseHandler) Delete(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httputil.RespondError(c, apierr.ErrBadRequest.WithDetail("invalid database ID"))
		return
	}

	if err := h.svc.Delete(c.Request.Context(), id); err != nil {
		httputil.RespondError(c, dbErr(err))
		return
	}

	httputil.RespondNoContent(c)
}

// ListVersions godoc
// @Summary      List supported database versions
// @Tags         databases
// @Produce      json
// @Param        engine query string false "Database engine"
// @Success      200 {object} map[string]interface{}
// @Failure      401 {object} apidocs.ProblemDetail
// @Security     BearerAuth
// @Router       /databases/versions [get]
func (h *DatabaseHandler) ListVersions(c *gin.Context) {
	engine := c.Query("engine")
	if engine != "" {
		versions, ok := model.SupportedVersions[model.DBEngine(engine)]
		if !ok {
			httputil.RespondError(c, apierr.ErrBadRequest.WithDetail("unknown engine"))
			return
		}
		c.JSON(http.StatusOK, versions)
		return
	}
	c.JSON(http.StatusOK, model.SupportedVersions)
}

// GetCredentials godoc
// @Summary      Get database credentials
// @Tags         databases
// @Produce      json
// @Param        id path string true "Database ID"
// @Success      200 {object} map[string]interface{}
// @Failure      401 {object} apidocs.ProblemDetail
// @Security     BearerAuth
// @Router       /databases/{id}/credentials [get]
func (h *DatabaseHandler) GetCredentials(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httputil.RespondError(c, apierr.ErrBadRequest.WithDetail("invalid database ID"))
		return
	}

	creds, err := h.svc.GetCredentials(c.Request.Context(), id)
	if err != nil {
		httputil.RespondError(c, dbErr(err))
		return
	}

	httputil.RespondOK(c, creds)
}

// GetStatus godoc
// @Summary      Get database status
// @Tags         databases
// @Produce      json
// @Param        id path string true "Database ID"
// @Success      200 {object} map[string]interface{}
// @Failure      401 {object} apidocs.ProblemDetail
// @Security     BearerAuth
// @Router       /databases/{id}/status [get]
func (h *DatabaseHandler) GetStatus(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httputil.RespondError(c, apierr.ErrBadRequest.WithDetail("invalid database ID"))
		return
	}

	status, err := h.svc.GetStatus(c.Request.Context(), id)
	if err != nil {
		httputil.RespondError(c, dbErr(err))
		return
	}

	httputil.RespondOK(c, status)
}

// GetPods godoc
// @Summary      List database pods
// @Tags         databases
// @Produce      json
// @Param        id path string true "Database ID"
// @Success      200 {array} object
// @Failure      401 {object} apidocs.ProblemDetail
// @Security     BearerAuth
// @Router       /databases/{id}/pods [get]
func (h *DatabaseHandler) GetPods(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httputil.RespondError(c, apierr.ErrBadRequest.WithDetail("invalid database ID"))
		return
	}

	pods, err := h.svc.GetPods(c.Request.Context(), id)
	if err != nil {
		httputil.RespondError(c, dbErr(err))
		return
	}

	httputil.RespondList(c, pods)
}

// TriggerBackup godoc
// @Summary      Trigger database backup
// @Tags         databases
// @Produce      json
// @Param        id path string true "Database ID"
// @Success      201 {object} object
// @Failure      401 {object} apidocs.ProblemDetail
// @Security     BearerAuth
// @Router       /databases/{id}/backups [post]
func (h *DatabaseHandler) TriggerBackup(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httputil.RespondError(c, apierr.ErrBadRequest.WithDetail("invalid database ID"))
		return
	}

	backup, err := h.svc.TriggerBackup(c.Request.Context(), id)
	if err != nil {
		httputil.RespondError(c, dbErr(err))
		return
	}

	httputil.RespondCreated(c, backup, fmt.Sprintf("/api/v1/databases/%s/backups", id))
}

// UpdateExternalAccess godoc
// @Summary      Update database external access
// @Tags         databases
// @Accept       json
// @Produce      json
// @Param        id path string true "Database ID"
// @Param        body body object true "Request body"
// @Success      200 {object} apidocs.ManagedDatabase
// @Failure      401 {object} apidocs.ProblemDetail
// @Security     BearerAuth
// @Router       /databases/{id}/external-access [post]
func (h *DatabaseHandler) UpdateExternalAccess(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httputil.RespondError(c, apierr.ErrBadRequest.WithDetail("invalid database ID"))
		return
	}

	var input struct {
		Enabled bool  `json:"enabled"`
		Port    int32 `json:"port"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		httputil.RespondError(c, apierr.ErrValidation.WithDetail(err.Error()))
		return
	}

	db, err := h.svc.UpdateExternalAccess(c.Request.Context(), id, input.Enabled, input.Port)
	if err != nil {
		httputil.RespondError(c, dbErr(err))
		return
	}

	httputil.RespondOK(c, db)
}

// UsedPorts godoc
// @Summary      List used external database ports
// @Tags         databases
// @Produce      json
// @Success      200 {array} object
// @Failure      401 {object} apidocs.ProblemDetail
// @Security     BearerAuth
// @Router       /databases/used-ports [get]
func (h *DatabaseHandler) UsedPorts(c *gin.Context) {
	ports, err := h.svc.UsedExternalPorts(c.Request.Context())
	if err != nil {
		httputil.RespondError(c, dbErr(err))
		return
	}
	if ports == nil {
		ports = []model.ExternalPortInfo{}
	}
	httputil.RespondOK(c, ports)
}

// UpdateBackupConfig godoc
// @Summary      Update database backup configuration
// @Tags         databases
// @Accept       json
// @Produce      json
// @Param        id path string true "Database ID"
// @Param        body body apidocs.UpdateBackupInput true "Request body"
// @Success      200 {object} apidocs.ManagedDatabase
// @Failure      401 {object} apidocs.ProblemDetail
// @Security     BearerAuth
// @Router       /databases/{id}/backup-config [put]
func (h *DatabaseHandler) UpdateBackupConfig(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httputil.RespondError(c, apierr.ErrBadRequest.WithDetail("invalid database ID"))
		return
	}

	var input service.UpdateBackupInput
	if err := c.ShouldBindJSON(&input); err != nil {
		httputil.RespondError(c, apierr.ErrValidation.WithDetail(err.Error()))
		return
	}

	db, err := h.svc.UpdateBackupConfig(c.Request.Context(), id, input)
	if err != nil {
		httputil.RespondError(c, dbErr(err))
		return
	}

	httputil.RespondOK(c, db)
}

// RestoreBackup godoc
// @Summary      Restore database from backup
// @Tags         databases
// @Produce      json
// @Param        id path string true "Database ID"
// @Param        backupId path string true "Backup ID"
// @Success      200 {object} apidocs.MessageResponse
// @Failure      401 {object} apidocs.ProblemDetail
// @Security     BearerAuth
// @Router       /databases/{id}/backups/{backupId}/restore [post]
func (h *DatabaseHandler) RestoreBackup(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httputil.RespondError(c, apierr.ErrBadRequest.WithDetail("invalid database ID"))
		return
	}

	backupID, err := uuid.Parse(c.Param("backupId"))
	if err != nil {
		httputil.RespondError(c, apierr.ErrBadRequest.WithDetail("invalid backup ID"))
		return
	}

	if err := h.svc.RestoreBackup(c.Request.Context(), id, backupID); err != nil {
		httputil.RespondError(c, dbErr(err))
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "restore started"})
}

// ListBackups godoc
// @Summary      List database backups
// @Tags         databases
// @Produce      json
// @Param        id path string true "Database ID"
// @Param        page     query int false "Page number"
// @Param        per_page query int false "Items per page (max 100)"
// @Success      200 {object} object
// @Failure      401 {object} apidocs.ProblemDetail
// @Security     BearerAuth
// @Router       /databases/{id}/backups [get]
func (h *DatabaseHandler) ListBackups(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httputil.RespondError(c, apierr.ErrBadRequest.WithDetail("invalid database ID"))
		return
	}

	params := bindListParams(c)
	backups, total, err := h.svc.ListBackups(c.Request.Context(), id, params)
	if err != nil {
		httputil.RespondError(c, dbErr(err))
		return
	}

	httputil.RespondOK(c, httputil.NewListResponse(backups, params.Page, params.PerPage, total))
}
