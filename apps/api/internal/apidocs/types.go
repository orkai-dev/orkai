package apidocs

import (
	"github.com/orkai-dev/orkai/apps/api/internal/apierr"
	"github.com/orkai-dev/orkai/apps/api/internal/httputil"
	"github.com/orkai-dev/orkai/apps/api/internal/model"
	"github.com/orkai-dev/orkai/apps/api/internal/service"
)

// ProblemDetail is an RFC 9457 error response.
type ProblemDetail = apierr.ProblemDetail

// Auth and user types (service layer).
type (
	RegisterInput        = service.RegisterInput
	LoginInput           = service.LoginInput
	RefreshInput         = service.RefreshInput
	AuthResult           = service.AuthResult
	SetupStatus          = service.SetupStatus
	UpdateProfileInput   = service.UpdateProfileInput
	ChangePasswordInput  = service.ChangePasswordInput
	Verify2FAInput       = service.Verify2FAInput
	CreateProjectInput   = service.CreateProjectInput
	UpdateProjectInput   = service.UpdateProjectInput
	CreateAppInput       = service.CreateAppInput
	UpdateAppInput       = service.UpdateAppInput
	CreateDatabaseInput  = service.CreateDatabaseInput
	UpdateBackupInput    = service.UpdateBackupInput
	CreatePageInput      = service.CreatePageInput
	UpdatePageInput      = service.UpdatePageInput
	CreateCronJobInput   = service.CreateCronJobInput
	UpdateCronJobInput   = service.UpdateCronJobInput
	CreateDomainInput    = service.CreateDomainInput
	CreateNodeInput      = service.CreateNodeInput
	CreateResourceInput  = service.CreateResourceInput
	UpdateResourceInput  = service.UpdateResourceInput
	DNSUpsertRecordInput = service.DNSUpsertRecordInput
	DNSDeleteRecordInput = service.DNSDeleteRecordInput
	SystemBackupConfig   = service.SystemBackupConfig
	SMTPConfig           = service.SMTPConfig
)

// Entity types (model layer).
type (
	User            = model.User
	Project         = model.Project
	Application     = model.Application
	Deployment      = model.Deployment
	ManagedDatabase = model.ManagedDatabase
	Page            = model.Page
	CronJob         = model.CronJob
	Domain          = model.Domain
	Template        = model.Template
	APIKey          = model.APIKey
	SharedResource  = model.SharedResource
	ServerNode      = model.ServerNode
)

// MessageResponse is a simple JSON message payload.
type MessageResponse struct {
	Message string `json:"message"`
}

// ProjectListResponse is a paginated project list.
type ProjectListResponse struct {
	Items      []model.Project     `json:"items"`
	Pagination httputil.Pagination `json:"pagination"`
}

// ApplicationListResponse is a paginated application list.
type ApplicationListResponse struct {
	Items      []model.Application `json:"items"`
	Pagination httputil.Pagination `json:"pagination"`
}

// DeploymentListResponse is a paginated deployment list.
type DeploymentListResponse struct {
	Items      []model.Deployment  `json:"items"`
	Pagination httputil.Pagination `json:"pagination"`
}

// DatabaseListResponse is a paginated database list.
type DatabaseListResponse struct {
	Items      []model.ManagedDatabase `json:"items"`
	Pagination httputil.Pagination     `json:"pagination"`
}

// PageListResponse is a paginated static page list.
type PageListResponse struct {
	Items      []model.Page        `json:"items"`
	Pagination httputil.Pagination `json:"pagination"`
}

// CronJobListResponse is a paginated cron job list.
type CronJobListResponse struct {
	Items      []model.CronJob     `json:"items"`
	Pagination httputil.Pagination `json:"pagination"`
}

// DomainListResponse is a paginated domain list.
type DomainListResponse struct {
	Items      []model.Domain      `json:"items"`
	Pagination httputil.Pagination `json:"pagination"`
}

// TemplateListResponse is a paginated template list.
type TemplateListResponse struct {
	Items      []model.Template    `json:"items"`
	Pagination httputil.Pagination `json:"pagination"`
}

// APIKeyListResponse is a paginated API key list.
type APIKeyListResponse struct {
	Items      []model.APIKey      `json:"items"`
	Pagination httputil.Pagination `json:"pagination"`
}

// ResourceListResponse is a paginated shared resource list.
type ResourceListResponse struct {
	Items      []model.SharedResource `json:"items"`
	Pagination httputil.Pagination    `json:"pagination"`
}

// NodeListResponse is a paginated server node list.
type NodeListResponse struct {
	Items      []model.ServerNode  `json:"items"`
	Pagination httputil.Pagination `json:"pagination"`
}
