package v1

import (
	"github.com/gin-gonic/gin"
	"github.com/orkai-dev/orkai/apps/api/internal/apidocs"
	"github.com/orkai-dev/orkai/apps/api/internal/apierr"
	"github.com/orkai-dev/orkai/apps/api/internal/httputil"
	"github.com/orkai-dev/orkai/apps/api/internal/orchestrator"
	"github.com/orkai-dev/orkai/apps/api/internal/service"
)

// Swag type resolution for apidocs.* annotation references.
var (
	_ apidocs.ProblemDetail
)

// SystemBackupHandler handles system backup API endpoints.
type SystemBackupHandler struct {
	svc *service.SystemBackupService
}

// NewSystemBackupHandler creates a new SystemBackupHandler.
func NewSystemBackupHandler(svc *service.SystemBackupService) *SystemBackupHandler {
	return &SystemBackupHandler{svc: svc}
}

// GetConfig returns the current system backup configuration.
// GetConfig godoc
// @Summary      Get system backup configuration
// @Tags         system
// @Description  Requires admin role.
// @Produce      json
// @Success      200 {object} apidocs.SystemBackupConfig
// @Failure      403 {object} apidocs.ProblemDetail
// @Security     BearerAuth
// @Router       /system/backup/config [get]
func (h *SystemBackupHandler) GetConfig(c *gin.Context) {
	cfg, err := h.svc.GetConfig(c.Request.Context())
	if err != nil {
		httputil.RespondError(c, err)
		return
	}
	httputil.RespondOK(c, cfg)
}

// SaveConfig updates the system backup configuration.
// SaveConfig godoc
// @Summary      Save system backup configuration
// @Tags         system
// @Description  Requires admin role.
// @Accept       json
// @Produce      json
// @Param        body body apidocs.SystemBackupConfig true "Request body"
// @Success      200 {object} map[string]interface{}
// @Failure      400 {object} apidocs.ProblemDetail
// @Failure      403 {object} apidocs.ProblemDetail
// @Security     BearerAuth
// @Router       /system/backup/config [put]
func (h *SystemBackupHandler) SaveConfig(c *gin.Context) {
	var input service.SystemBackupConfig
	if err := c.ShouldBindJSON(&input); err != nil {
		httputil.RespondError(c, apierr.ErrValidation.WithDetail(err.Error()))
		return
	}
	if err := h.svc.SaveConfig(c.Request.Context(), &input); err != nil {
		httputil.RespondError(c, apierr.ErrBadRequest.WithDetail(err.Error()))
		return
	}
	httputil.RespondOK(c, gin.H{"status": "saved"})
}

// TriggerBackup starts an immediate system backup.
// TriggerBackup godoc
// @Summary      Trigger system backup
// @Tags         system
// @Description  Requires admin role.
// @Produce      json
// @Success      200 {object} object
// @Failure      403 {object} apidocs.ProblemDetail
// @Security     BearerAuth
// @Router       /system/backup/trigger [post]
func (h *SystemBackupHandler) TriggerBackup(c *gin.Context) {
	backup, err := h.svc.TriggerBackup(c.Request.Context())
	if err != nil {
		httputil.RespondError(c, err)
		return
	}
	httputil.RespondOK(c, backup)
}

// ListBackups returns a paginated list of system backups.
// ListBackups godoc
// @Summary      List system backups
// @Tags         system
// @Description  Requires admin role.
// @Produce      json
// @Param        page     query int false "Page number"
// @Param        per_page query int false "Items per page (max 100)"
// @Success      200 {object} object
// @Failure      403 {object} apidocs.ProblemDetail
// @Security     BearerAuth
// @Router       /system/backup/list [get]
func (h *SystemBackupHandler) ListBackups(c *gin.Context) {
	params := bindListParams(c)
	backups, total, err := h.svc.ListBackups(c.Request.Context(), params)
	if err != nil {
		httputil.RespondError(c, err)
		return
	}
	httputil.RespondOK(c, httputil.NewListResponse(backups, params.Page, params.PerPage, total))
}

// ScanS3Backups lists available backup files in an S3 bucket.
// ScanS3Backups godoc
// @Summary      Scan S3 backups for restore
// @Tags         system
// @Accept       json
// @Produce      json
// @Param        body body object true "Request body"
// @Success      200 {array} object
// @Failure      400 {object} apidocs.ProblemDetail
// @Router       /system/restore/scan [post]
func (h *SystemBackupHandler) ScanS3Backups(c *gin.Context) {
	var input struct {
		Endpoint  string `json:"endpoint" binding:"required"`
		Bucket    string `json:"bucket" binding:"required"`
		AccessKey string `json:"access_key" binding:"required"`
		SecretKey string `json:"secret_key" binding:"required"`
		Path      string `json:"path"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		httputil.RespondError(c, apierr.ErrValidation.WithDetail(err.Error()))
		return
	}

	s3Config := orchestrator.S3Config{
		Endpoint:  input.Endpoint,
		Bucket:    input.Bucket,
		AccessKey: input.AccessKey,
		SecretKey: input.SecretKey,
	}

	files, err := h.svc.ScanS3Backups(c.Request.Context(), s3Config, input.Path)
	if err != nil {
		httputil.RespondError(c, apierr.ErrBadRequest.WithDetail(err.Error()))
		return
	}
	httputil.RespondOK(c, files)
}

// RestoreFromS3 downloads a backup from S3 and restores it.
// RestoreFromS3 godoc
// @Summary      Restore system from S3 backup
// @Tags         system
// @Accept       json
// @Produce      json
// @Param        body body object true "Request body"
// @Success      200 {object} map[string]interface{}
// @Router       /system/restore/execute [post]
func (h *SystemBackupHandler) RestoreFromS3(c *gin.Context) {
	var input struct {
		Endpoint  string `json:"endpoint" binding:"required"`
		Bucket    string `json:"bucket" binding:"required"`
		AccessKey string `json:"access_key" binding:"required"`
		SecretKey string `json:"secret_key" binding:"required"`
		S3Key     string `json:"s3_key" binding:"required"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		httputil.RespondError(c, apierr.ErrValidation.WithDetail(err.Error()))
		return
	}

	s3Config := orchestrator.S3Config{
		Endpoint:  input.Endpoint,
		Bucket:    input.Bucket,
		AccessKey: input.AccessKey,
		SecretKey: input.SecretKey,
	}

	if err := h.svc.RestoreFromS3(c.Request.Context(), s3Config, input.S3Key); err != nil {
		httputil.RespondError(c, err)
		return
	}
	httputil.RespondOK(c, gin.H{"status": "restored"})
}
