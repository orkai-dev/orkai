package orchestrator

import (
	"io"
	"time"

	"github.com/orkai-dev/orkai/apps/api/internal/model"
)

// ErrCapabilityUnsupported is returned when a deploy target does not implement
// a requested optional capability interface.
type ErrCapabilityUnsupported struct {
	Capability Capability
}

func (e ErrCapabilityUnsupported) Error() string {
	return "deploy target does not support capability: " + string(e.Capability)
}

// AsCapability type-asserts target to the requested capability interface.
func AsCapability[T any](target any, cap Capability) (T, error) {
	v, ok := target.(T)
	if !ok {
		var zero T
		return zero, ErrCapabilityUnsupported{Capability: cap}
	}
	return v, nil
}

// RequireKubernetesInspector returns the KubernetesInspector from a deploy target.
func RequireKubernetesInspector(target DeployTarget) (KubernetesInspector, error) {
	return AsCapability[KubernetesInspector](target, CapKubernetes)
}

type TraefikStatus struct {
	Ready    bool   `json:"ready"`
	PodName  string `json:"pod_name"`
	Restarts int32  `json:"restarts"`
	Age      string `json:"age"`
}

// ClusterTopology represents the full resource graph of the cluster.
type ClusterTopology struct {
	Nodes       []TopologyNode       `json:"nodes"`
	Deployments []TopologyDeployment `json:"deployments"`
	Pods        []TopologyPod        `json:"pods"`
	Services    []TopologyService    `json:"services"`
	Ingresses   []TopologyIngress    `json:"ingresses"`
}

type TopologyNode struct {
	Name   string `json:"name"`
	Status string `json:"status"`
	IP     string `json:"ip"`
	Roles  string `json:"roles"`
}

type TopologyDeployment struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
	Ready     int32  `json:"ready"`
	Desired   int32  `json:"desired"`
	AppID     string `json:"app_id,omitempty"`
}

type TopologyPod struct {
	Name       string `json:"name"`
	Namespace  string `json:"namespace"`
	Phase      string `json:"phase"`
	Node       string `json:"node"`
	IP         string `json:"ip"`
	AppID      string `json:"app_id,omitempty"`
	Deployment string `json:"deployment,omitempty"`
}

type TopologyService struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
	Type      string `json:"type"`
	ClusterIP string `json:"cluster_ip"`
	Ports     string `json:"ports"`
	AppID     string `json:"app_id,omitempty"`
}

type TopologyIngress struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
	Host      string `json:"host"`
	Service   string `json:"service"`
	AppID     string `json:"app_id,omitempty"`
}

type DeployOpts struct {
	Image       string
	Replicas    int32
	EnvVars     map[string]string
	Ports       []model.PortMapping
	CPULimit    string
	MemLimit    string
	Annotations map[string]string

	// ImagePullSecret, if non-empty, is a dockerconfigjson document used to
	// authenticate the image pull (e.g. for private/ECR registries).
	ImagePullSecret []byte

	CPURequest             string
	MemRequest             string
	HealthCheck            *model.HealthCheck
	Volumes                []model.VolumeMount
	DeployStrategy         string
	DeployStrategyConfig   *model.DeployStrategyConfig
	TerminationGracePeriod int
	NodePool               string
}

type AppStatus struct {
	Phase            string    `json:"phase"` // running | pending | failed | unknown
	ReadyReplicas    int32     `json:"ready_replicas"`
	DesiredReplicas  int32     `json:"desired_replicas"`
	LastTransitionAt time.Time `json:"last_transition_at"`
	Message          string    `json:"message,omitempty"`
}

type PodInfo struct {
	Name         string            `json:"name"`
	Namespace    string            `json:"namespace"`
	Phase        string            `json:"phase"`
	Node         string            `json:"node"`
	IP           string            `json:"ip"`
	StartedAt    time.Time         `json:"started_at"`
	Resources    ResourceMetrics   `json:"resources"`
	RestartCount int32             `json:"restart_count"`
	Ready        bool              `json:"ready"`
	Containers   []ContainerStatus `json:"containers"`
	AppID        string            `json:"app_id,omitempty"`
}

type PodEvent struct {
	Type      string    `json:"type"` // Normal | Warning
	Reason    string    `json:"reason"`
	Message   string    `json:"message"`
	Count     int32     `json:"count"`
	FirstSeen time.Time `json:"first_seen"`
	LastSeen  time.Time `json:"last_seen"`
}

type ContainerStatus struct {
	Name         string `json:"name"`
	Ready        bool   `json:"ready"`
	RestartCount int32  `json:"restart_count"`
	State        string `json:"state"`  // running | waiting | terminated
	Reason       string `json:"reason"` // CrashLoopBackOff, etc.
}

type IngressStatus struct {
	Ready      bool   `json:"ready"`
	Message    string `json:"message,omitempty"`
	CertSecret string `json:"cert_secret,omitempty"`
}

type NodeInfo struct {
	Name      string          `json:"name"`
	IP        string          `json:"ip"`
	Status    string          `json:"status"`
	Roles     []string        `json:"roles"`
	Pool      string          `json:"pool,omitempty"`
	Version   string          `json:"version"`
	OS        string          `json:"os"`
	Arch      string          `json:"arch"`
	Resources ResourceMetrics `json:"resources"`
}

type ClusterMetrics struct {
	Nodes       int             `json:"nodes"`
	TotalPods   int             `json:"total_pods"`
	RunningPods int             `json:"running_pods"`
	Resources   ResourceMetrics `json:"resources"`
}

type ResourceMetrics struct {
	CPUUsed      string `json:"cpu_used"`
	CPUTotal     string `json:"cpu_total"`
	MemUsed      string `json:"mem_used"`
	MemTotal     string `json:"mem_total"`
	StorageUsed  string `json:"storage_used,omitempty"`
	StorageTotal string `json:"storage_total,omitempty"`
}

type ClusterEvent struct {
	Type           string    `json:"type"`
	Reason         string    `json:"reason"`
	Message        string    `json:"message"`
	Namespace      string    `json:"namespace"`
	InvolvedObject string    `json:"involved_object"`
	Count          int32     `json:"count"`
	FirstSeen      time.Time `json:"first_seen"`
	LastSeen       time.Time `json:"last_seen"`
}

type PVCInfo struct {
	Name         string   `json:"name"`
	Namespace    string   `json:"namespace"`
	Status       string   `json:"status"`
	Capacity     string   `json:"capacity"`
	StorageClass string   `json:"storage_class"`
	VolumeName   string   `json:"volume_name"`
	UsedBy       []string `json:"used_by"`
}

type StorageClassInfo struct {
	Name           string `json:"name"`
	Provisioner    string `json:"provisioner"`
	IsDefault      bool   `json:"is_default"`
	AllowExpansion bool   `json:"allow_expansion"`
}

type NamespaceInfo struct {
	Name     string `json:"name"`
	Status   string `json:"status"`
	PodCount int    `json:"pod_count"`
	SvcCount int    `json:"svc_count"`
}

type NodeMetrics struct {
	Name     string `json:"name"`
	CPUUsed  string `json:"cpu_used"`
	CPUTotal string `json:"cpu_total"`
	MemUsed  string `json:"mem_used"`
	MemTotal string `json:"mem_total"`
	PodCount int    `json:"pod_count"`
}

type VolumeOpts struct {
	Name         string
	Namespace    string
	Size         string
	StorageClass string
}

type LogOpts struct {
	Container  string
	Follow     bool
	TailLines  int64
	Since      time.Time
	Timestamps bool
}

type ExecOpts struct {
	Container string
	Command   []string
	TTY       bool
}

// TerminalSession represents a bidirectional terminal connection.
type TerminalSession interface {
	io.Reader
	io.Writer
	Resize(width, height uint16) error
	Close() error
}

// LogCallback is called periodically with incremental build logs during a build.
type LogCallback func(logs string)

type BuildOpts struct {
	GitRepo      string
	GitBranch    string
	CommitSHA    string
	GitToken     string
	Dockerfile   string
	BuildContext string
	BuildArgs    map[string]string
	BuildEnvVars map[string]string
	BuildType    string
	NoCache      bool
	OnLog        LogCallback
}

type BuildResult struct {
	Image    string
	Duration time.Duration
	Logs     string
}

type HelmRelease struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
	Chart     string `json:"chart"`
	Revision  string `json:"revision"`
	Status    string `json:"status"`
	Updated   string `json:"updated"`
}

type CleanupStats struct {
	EvictedPods        int      `json:"evicted_pods"`
	EvictedPodNames    []string `json:"evicted_pod_names"`
	FailedPods         int      `json:"failed_pods"`
	FailedPodNames     []string `json:"failed_pod_names"`
	CompletedPods      int      `json:"completed_pods"`
	CompletedPodNames  []string `json:"completed_pod_names"`
	StaleReplicaSets   int      `json:"stale_replicasets"`
	StaleRSNames       []string `json:"stale_rs_names"`
	CompletedJobs      int      `json:"completed_jobs"`
	CompletedJobNames  []string `json:"completed_job_names"`
	UnboundPVCs        int      `json:"unbound_pvcs"`
	UnboundPVCNames    []string `json:"unbound_pvc_names"`
	OrphanIngresses    int      `json:"orphan_ingresses"`
	OrphanIngressNames []string `json:"orphan_ingress_names"`
}

type CleanupResult struct {
	Deleted int    `json:"deleted"`
	Message string `json:"message"`
}

// S3Config holds credentials for S3-compatible object storage used for database backups.
type S3Config struct {
	Endpoint  string `json:"endpoint"`
	Bucket    string `json:"bucket"`
	AccessKey string `json:"access_key"`
	SecretKey string `json:"secret_key"`
	// SessionToken is set for temporary credentials (e.g. resolved from an EC2
	// instance role or an assumed role). Empty for static long-lived keys.
	SessionToken string `json:"session_token,omitempty"`
	Region       string `json:"region"`
}

// ObjectTransfer describes an in-cluster object-storage transfer container used
// by managed-database backup/restore Jobs. The provider resolves the image,
// credentials env, and shell command; the orchestrator mounts them into a Job.
type ObjectTransfer struct {
	Image   string
	Env     map[string]string
	Command []string
}

type DatabaseCredentials struct {
	Host             string `json:"host"`
	Port             int32  `json:"port"`
	Username         string `json:"username"`
	Password         string `json:"password"`
	DatabaseName     string `json:"database_name"`
	ConnectionString string `json:"connection_string"`
	InternalURL      string `json:"internal_url"`
}

type DaemonSetInfo struct {
	Name             string `json:"name"`
	Namespace        string `json:"namespace"`
	DesiredScheduled int32  `json:"desired_scheduled"`
	CurrentScheduled int32  `json:"current_scheduled"`
	Ready            int32  `json:"ready"`
	NodeSelector     string `json:"node_selector"`
	Images           string `json:"images"`
	CreatedAt        string `json:"created_at"`
}

// Compile-time checks: NoopOrchestrator satisfies capability interfaces.
var (
	_ Deployer            = (*NoopOrchestrator)(nil)
	_ DatabaseManager     = (*NoopOrchestrator)(nil)
	_ IngressBinder       = (*NoopOrchestrator)(nil)
	_ VolumeProvider      = (*NoopOrchestrator)(nil)
	_ LogStreamer         = (*NoopOrchestrator)(nil)
	_ Execer              = (*NoopOrchestrator)(nil)
	_ Builder             = (*NoopOrchestrator)(nil)
	_ SecretSink          = (*NoopOrchestrator)(nil)
	_ CronManager         = (*NoopOrchestrator)(nil)
	_ KubernetesInspector = (*NoopOrchestrator)(nil)
)
