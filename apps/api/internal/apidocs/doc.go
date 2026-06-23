// Package apidocs holds OpenAPI metadata and shared swagger types.
// Spec is generated at build time via `make swagger`; not imported by the server.
//
// @title           Orkai API
// @version         1.0
// @description     REST control plane for orka'i self-hosted PaaS
// @BasePath        /api/v1
//
// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
// @description     JWT access token or API key (ork_…)
package apidocs
