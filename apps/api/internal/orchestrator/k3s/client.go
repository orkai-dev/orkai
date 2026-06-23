package k3s

import (
	"context"
	"fmt"
	"log/slog"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/orkai-dev/orkai/apps/api/internal/config"
	"github.com/orkai-dev/orkai/apps/api/internal/orchestrator"
)

// Orchestrator implements K3s/K8s deploy-target capabilities with real API calls.
type Orchestrator struct {
	client              kubernetes.Interface
	config              *rest.Config
	logger              *slog.Logger
	baseCtx             context.Context
	baseCancel          context.CancelFunc
	defaultStorageClass string
}

// Compile-time checks that Orchestrator implements capability interfaces.
var (
	_ orchestrator.Deployer            = (*Orchestrator)(nil)
	_ orchestrator.KubernetesInspector = (*Orchestrator)(nil)
	_ orchestrator.Builder             = (*Orchestrator)(nil)
	_ orchestrator.DatabaseManager     = (*Orchestrator)(nil)
	_ orchestrator.IngressBinder       = (*Orchestrator)(nil)
	_ orchestrator.LogStreamer         = (*Orchestrator)(nil)
	_ orchestrator.Execer              = (*Orchestrator)(nil)
	_ orchestrator.SecretSink          = (*Orchestrator)(nil)
	_ orchestrator.CronManager         = (*Orchestrator)(nil)
	_ orchestrator.VolumeProvider      = (*Orchestrator)(nil)
)

// New creates a K3s orchestrator connected to a real cluster.
func New(cfg config.K8sConfig, logger *slog.Logger) (*Orchestrator, error) {
	var restCfg *rest.Config
	var err error

	if cfg.InCluster {
		restCfg, err = rest.InClusterConfig()
	} else if cfg.Kubeconfig != "" {
		restCfg, err = clientcmd.BuildConfigFromFlags("", cfg.Kubeconfig)
	} else {
		// Try default kubeconfig locations
		loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
		configOverrides := &clientcmd.ConfigOverrides{}
		restCfg, err = clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, configOverrides).ClientConfig()
	}
	if err != nil {
		return nil, fmt.Errorf("failed to build k8s config: %w", err)
	}

	clientset, err := kubernetes.NewForConfig(restCfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create k8s client: %w", err)
	}

	// Verify connection
	_, err = clientset.Discovery().ServerVersion()
	if err != nil {
		return nil, fmt.Errorf("failed to connect to k8s cluster: %w", err)
	}

	logger.Info("connected to K3s cluster")

	baseCtx, baseCancel := context.WithCancel(context.Background())

	return &Orchestrator{
		client:     clientset,
		config:     restCfg,
		logger:     logger,
		baseCtx:    baseCtx,
		baseCancel: baseCancel,
	}, nil
}

// Shutdown cancels background goroutines owned by the orchestrator.
func (o *Orchestrator) Shutdown() {
	if o.baseCancel != nil {
		o.baseCancel()
	}
}
