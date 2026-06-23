package orchestrator

import (
	"github.com/google/uuid"
)

// Capability identifies an optional deploy-target feature.
type Capability string

const (
	CapDeploy      Capability = "deploy"
	CapExec        Capability = "exec"
	CapVolumes     Capability = "volumes"
	CapManagedDB   Capability = "managed_db"
	CapBuild       Capability = "build"
	CapIngress     Capability = "ingress"
	CapSecrets     Capability = "secrets"
	CapCron        Capability = "cron"
	CapLogs        Capability = "logs"
	CapKubernetes  Capability = "kubernetes"
	CapPageBuild   Capability = "page_build"
	CapWorkerBuild Capability = "worker_build"
)

// AllCapabilities is the full capability set for K3s/noop targets.
func AllCapabilities() CapSet {
	return NewCapSet(
		CapDeploy,
		CapExec,
		CapVolumes,
		CapManagedDB,
		CapBuild,
		CapIngress,
		CapSecrets,
		CapCron,
		CapLogs,
		CapKubernetes,
		CapPageBuild,
		CapWorkerBuild,
	)
}

// CapSet is a set of supported capabilities.
type CapSet map[Capability]struct{}

// NewCapSet builds a CapSet from the given capabilities.
func NewCapSet(caps ...Capability) CapSet {
	s := make(CapSet, len(caps))
	for _, c := range caps {
		s.Add(c)
	}
	return s
}

// Has reports whether the set contains cap.
func (s CapSet) Has(cap Capability) bool {
	if s == nil {
		return false
	}
	_, ok := s[cap]
	return ok
}

// Add inserts cap into the set.
func (s CapSet) Add(cap Capability) {
	if s == nil {
		return
	}
	s[cap] = struct{}{}
}

// List returns all capabilities in the set (order not guaranteed).
func (s CapSet) List() []Capability {
	if len(s) == 0 {
		return nil
	}
	out := make([]Capability, 0, len(s))
	for c := range s {
		out = append(out, c)
	}
	return out
}

// DeployTarget is a place workloads can be deployed to.
// Every target must implement Deployer; optional capabilities are
// accessed via type assertion (Execer, Builder, etc.).
type DeployTarget interface {
	ID() uuid.UUID
	Kind() string
	Capabilities() CapSet
	Deployer
}
