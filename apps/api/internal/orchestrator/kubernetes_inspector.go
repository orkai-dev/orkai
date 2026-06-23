package orchestrator

import (
	"context"

	"github.com/orkai-dev/orkai/apps/api/internal/model"
)

// NamespaceManager handles namespace lifecycle (K8s-only).
type NamespaceManager interface {
	CreateNamespace(ctx context.Context, name string) error
	DeleteNamespace(ctx context.Context, name string) error
}

// ConfigMapManager handles K8s ConfigMap lifecycle.
type ConfigMapManager interface {
	EnsureConfigMap(ctx context.Context, namespace, name string, data map[string]string) error
	DeleteConfigMap(ctx context.Context, namespace, name string) error
}

// ResourceQuotaManager handles K8s ResourceQuota lifecycle.
type ResourceQuotaManager interface {
	EnsureResourceQuota(ctx context.Context, namespace string, quota model.ResourceQuotaConfig) error
	DeleteResourceQuota(ctx context.Context, namespace string) error
}

// NetworkPolicyManager handles K8s NetworkPolicy lifecycle.
type NetworkPolicyManager interface {
	EnsureNetworkPolicy(ctx context.Context, namespace string, enabled bool) error
}

// ServiceAccountManager handles K8s ServiceAccount lifecycle.
type ServiceAccountManager interface {
	EnsureServiceAccount(ctx context.Context, namespace, name string) error
	DeleteServiceAccount(ctx context.Context, namespace, name string) error
}

// ClusterInspector provides cluster-wide information (K8s-only).
type ClusterInspector interface {
	GetNodes(ctx context.Context) ([]NodeInfo, error)
	GetClusterMetrics(ctx context.Context) (*ClusterMetrics, error)
	GetNamespaceMetrics(ctx context.Context, namespace string) (*ResourceMetrics, error)
	GetAllPods(ctx context.Context) ([]PodInfo, error)
	GetClusterEvents(ctx context.Context, limit int) ([]ClusterEvent, error)
	GetPVCs(ctx context.Context) ([]PVCInfo, error)
	GetStorageClasses(ctx context.Context) ([]StorageClassInfo, error)
	GetNamespaces(ctx context.Context) ([]NamespaceInfo, error)
	GetNodeMetrics(ctx context.Context) ([]NodeMetrics, error)
	GetClusterTopology(ctx context.Context) (*ClusterTopology, error)
	SetNodeLabel(ctx context.Context, nodeName, key, value string) error
	RemoveNodeLabel(ctx context.Context, nodeName, key string) error
	GetNodePools(ctx context.Context) ([]string, error)
}

// TraefikManager handles Traefik configuration (K8s-only).
type TraefikManager interface {
	GetTraefikConfig(ctx context.Context) (string, error)
	UpdateTraefikConfig(ctx context.Context, yaml string) error
	RestartTraefik(ctx context.Context) error
	GetTraefikStatus(ctx context.Context) (*TraefikStatus, error)
}

// HelmInspector provides information about Helm releases (K8s-only).
type HelmInspector interface {
	GetHelmReleases(ctx context.Context) ([]HelmRelease, error)
}

// DaemonSetInspector provides information about DaemonSets (K8s-only).
type DaemonSetInspector interface {
	GetDaemonSets(ctx context.Context) ([]DaemonSetInfo, error)
}

// CleanupInspector provides cluster cleanup operations (K8s-only).
type CleanupInspector interface {
	GetCleanupStats(ctx context.Context) (*CleanupStats, error)
	CleanupEvictedPods(ctx context.Context) (*CleanupResult, error)
	CleanupFailedPods(ctx context.Context) (*CleanupResult, error)
	CleanupCompletedPods(ctx context.Context) (*CleanupResult, error)
	CleanupStaleReplicaSets(ctx context.Context) (*CleanupResult, error)
	CleanupCompletedJobs(ctx context.Context) (*CleanupResult, error)
	GetOrphanIngresses(ctx context.Context, validHosts map[string]bool, systemIngresses map[string]string) ([]string, error)
	CleanupOrphanIngresses(ctx context.Context, validHosts map[string]bool, systemIngresses map[string]string) (*CleanupResult, error)
}

// KubernetesInspector groups all K8s-specific platform operations.
// Only K3s/K8s deploy targets implement this interface.
type KubernetesInspector interface {
	NamespaceManager
	ConfigMapManager
	ResourceQuotaManager
	NetworkPolicyManager
	ServiceAccountManager
	ClusterInspector
	TraefikManager
	HelmInspector
	DaemonSetInspector
	CleanupInspector
}
