package v1

import (
	"context"
	"fmt"
	"strconv"

	"github.com/orkai-dev/orkai/apps/api/internal/apidocs"

	"github.com/gin-gonic/gin"
	"github.com/orkai-dev/orkai/apps/api/internal/api/middleware"
	"github.com/orkai-dev/orkai/apps/api/internal/apierr"
	"github.com/orkai-dev/orkai/apps/api/internal/httputil"
	"github.com/orkai-dev/orkai/apps/api/internal/model"
	"github.com/orkai-dev/orkai/apps/api/internal/orchestrator"
	"github.com/orkai-dev/orkai/apps/api/internal/service"
	"github.com/orkai-dev/orkai/apps/api/internal/store"
)

// Swag type resolution for apidocs.* annotation references.
var (
	_ apidocs.MessageResponse
	_ apidocs.ProblemDetail
)

type ClusterHandler struct {
	target  orchestrator.DeployTarget
	k8s     orchestrator.KubernetesInspector
	volumes orchestrator.VolumeProvider
	store   store.Store
	notif   *service.NotificationService
}

func NewClusterHandler(targets *orchestrator.TargetRegistry, s store.Store, notif *service.NotificationService) *ClusterHandler {
	target := targets.Default()
	k8s, err := orchestrator.RequireKubernetesInspector(target)
	if err != nil {
		panic("default deploy target must implement KubernetesInspector: " + err.Error())
	}
	volumes, _ := orchestrator.AsCapability[orchestrator.VolumeProvider](target, orchestrator.CapVolumes)
	return &ClusterHandler{target: target, k8s: k8s, volumes: volumes, store: s, notif: notif}
}

// GetCapabilities godoc
// @Summary      Get cluster capabilities
// @Tags         cluster
// @Description  Requires admin role.
// @Produce      json
// @Success      200 {object} map[string]interface{}
// @Failure      403 {object} apidocs.ProblemDetail
// @Security     BearerAuth
// @Router       /cluster/capabilities [get]
func (h *ClusterHandler) GetCapabilities(c *gin.Context) {
	caps := h.target.Capabilities().List()
	httputil.RespondOK(c, gin.H{
		"target_id":    h.target.ID(),
		"kind":         h.target.Kind(),
		"capabilities": caps,
	})
}

// GetNodes godoc
// @Summary      List cluster nodes
// @Tags         cluster
// @Description  Requires admin role.
// @Produce      json
// @Success      200 {array} object
// @Failure      403 {object} apidocs.ProblemDetail
// @Security     BearerAuth
// @Router       /cluster/nodes [get]
func (h *ClusterHandler) GetNodes(c *gin.Context) {
	nodes, err := h.k8s.GetNodes(c.Request.Context())
	if err != nil {
		httputil.RespondError(c, err)
		return
	}
	httputil.RespondList(c, nodes)
}

// GetMetrics godoc
// @Summary      Get cluster metrics
// @Tags         cluster
// @Description  Requires admin role.
// @Produce      json
// @Success      200 {object} map[string]interface{}
// @Failure      403 {object} apidocs.ProblemDetail
// @Security     BearerAuth
// @Router       /cluster/metrics [get]
func (h *ClusterHandler) GetMetrics(c *gin.Context) {
	metrics, err := h.k8s.GetClusterMetrics(c.Request.Context())
	if err != nil {
		httputil.RespondError(c, err)
		return
	}
	httputil.RespondOK(c, metrics)
}

// GetAllPods godoc
// @Summary      List all cluster pods
// @Tags         cluster
// @Description  Requires admin role.
// @Produce      json
// @Success      200 {array} object
// @Failure      403 {object} apidocs.ProblemDetail
// @Security     BearerAuth
// @Router       /cluster/pods [get]
func (h *ClusterHandler) GetAllPods(c *gin.Context) {
	pods, err := h.k8s.GetAllPods(c.Request.Context())
	if err != nil {
		httputil.RespondError(c, err)
		return
	}
	httputil.RespondList(c, pods)
}

// GetEvents godoc
// @Summary      List cluster events
// @Tags         cluster
// @Description  Requires admin role.
// @Produce      json
// @Param        limit query int false "Max events (default 100)"
// @Success      200 {array} object
// @Failure      403 {object} apidocs.ProblemDetail
// @Security     BearerAuth
// @Router       /cluster/events [get]
func (h *ClusterHandler) GetEvents(c *gin.Context) {
	limitStr := c.DefaultQuery("limit", "100")
	limit, _ := strconv.Atoi(limitStr)
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	events, err := h.k8s.GetClusterEvents(c.Request.Context(), limit)
	if err != nil {
		httputil.RespondError(c, err)
		return
	}
	httputil.RespondList(c, events)
}

// GetPVCs godoc
// @Summary      List persistent volume claims
// @Tags         cluster
// @Description  Requires admin role.
// @Produce      json
// @Success      200 {array} object
// @Failure      403 {object} apidocs.ProblemDetail
// @Security     BearerAuth
// @Router       /cluster/pvcs [get]
func (h *ClusterHandler) GetPVCs(c *gin.Context) {
	pvcs, err := h.k8s.GetPVCs(c.Request.Context())
	if err != nil {
		httputil.RespondError(c, err)
		return
	}
	httputil.RespondList(c, pvcs)
}

// GetStorageClasses godoc
// @Summary      List storage classes
// @Tags         cluster
// @Description  Requires admin role.
// @Produce      json
// @Success      200 {array} object
// @Failure      403 {object} apidocs.ProblemDetail
// @Security     BearerAuth
// @Router       /cluster/storage-classes [get]
func (h *ClusterHandler) GetStorageClasses(c *gin.Context) {
	classes, err := h.k8s.GetStorageClasses(c.Request.Context())
	if err != nil {
		httputil.RespondError(c, err)
		return
	}
	httputil.RespondList(c, classes)
}

// GetNamespaces godoc
// @Summary      List namespaces
// @Tags         cluster
// @Description  Requires admin role.
// @Produce      json
// @Success      200 {array} object
// @Failure      403 {object} apidocs.ProblemDetail
// @Security     BearerAuth
// @Router       /cluster/namespaces [get]
func (h *ClusterHandler) GetNamespaces(c *gin.Context) {
	namespaces, err := h.k8s.GetNamespaces(c.Request.Context())
	if err != nil {
		httputil.RespondError(c, err)
		return
	}
	httputil.RespondList(c, namespaces)
}

// GetNodeMetrics godoc
// @Summary      Get node metrics
// @Tags         cluster
// @Description  Requires admin role.
// @Produce      json
// @Success      200 {array} object
// @Failure      403 {object} apidocs.ProblemDetail
// @Security     BearerAuth
// @Router       /cluster/node-metrics [get]
func (h *ClusterHandler) GetNodeMetrics(c *gin.Context) {
	metrics, err := h.k8s.GetNodeMetrics(c.Request.Context())
	if err != nil {
		httputil.RespondError(c, err)
		return
	}
	httputil.RespondList(c, metrics)
}

// GetTopology godoc
// @Summary      Get cluster topology
// @Tags         cluster
// @Description  Requires admin role.
// @Produce      json
// @Success      200 {object} map[string]interface{}
// @Failure      403 {object} apidocs.ProblemDetail
// @Security     BearerAuth
// @Router       /cluster/topology [get]
func (h *ClusterHandler) GetTopology(c *gin.Context) {
	topo, err := h.k8s.GetClusterTopology(c.Request.Context())
	if err != nil {
		httputil.RespondError(c, err)
		return
	}
	httputil.RespondOK(c, topo)
}

// GetNodePools godoc
// @Summary      List node pools
// @Tags         cluster
// @Description  Requires admin role.
// @Produce      json
// @Success      200 {object} map[string]interface{}
// @Failure      403 {object} apidocs.ProblemDetail
// @Security     BearerAuth
// @Router       /cluster/node-pools [get]
func (h *ClusterHandler) GetNodePools(c *gin.Context) {
	pools, err := h.k8s.GetNodePools(c.Request.Context())
	if err != nil {
		httputil.RespondError(c, err)
		return
	}
	httputil.RespondOK(c, pools)
}

// SetNodePool godoc
// @Summary      Set node pool label
// @Tags         cluster
// @Description  Requires admin role.
// @Accept       json
// @Produce      json
// @Param        name path string true "Node name"
// @Param        body body object true "Request body"
// @Success      200 {object} apidocs.MessageResponse
// @Failure      403 {object} apidocs.ProblemDetail
// @Security     BearerAuth
// @Router       /cluster/nodes/{name}/pool [put]
func (h *ClusterHandler) SetNodePool(c *gin.Context) {
	nodeName := c.Param("name")
	var body struct {
		Pool string `json:"pool"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		httputil.RespondError(c, apierr.ErrBadRequest.WithDetail(err.Error()))
		return
	}
	if body.Pool == "" {
		// Remove pool label
		if err := h.k8s.RemoveNodeLabel(c.Request.Context(), nodeName, "orkai/pool"); err != nil {
			httputil.RespondError(c, err)
			return
		}
	} else {
		if err := h.k8s.SetNodeLabel(c.Request.Context(), nodeName, "orkai/pool", body.Pool); err != nil {
			httputil.RespondError(c, err)
			return
		}
	}
	httputil.RespondOK(c, gin.H{"message": "node pool updated"})
}

// GetTraefikConfig godoc
// @Summary      Get Traefik configuration
// @Tags         cluster
// @Description  Requires admin role.
// @Produce      json
// @Success      200 {object} map[string]interface{}
// @Failure      403 {object} apidocs.ProblemDetail
// @Security     BearerAuth
// @Router       /cluster/traefik-config [get]
func (h *ClusterHandler) GetTraefikConfig(c *gin.Context) {
	yaml, err := h.k8s.GetTraefikConfig(c.Request.Context())
	if err != nil {
		httputil.RespondError(c, err)
		return
	}
	httputil.RespondOK(c, gin.H{"yaml": yaml})
}

// UpdateTraefikConfig godoc
// @Summary      Update Traefik configuration
// @Tags         cluster
// @Description  Requires admin role.
// @Accept       json
// @Produce      json
// @Param        body body object true "Request body"
// @Success      200 {object} apidocs.MessageResponse
// @Failure      403 {object} apidocs.ProblemDetail
// @Security     BearerAuth
// @Router       /cluster/traefik-config [put]
func (h *ClusterHandler) UpdateTraefikConfig(c *gin.Context) {
	var body struct {
		YAML string `json:"yaml" binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		httputil.RespondError(c, apierr.ErrBadRequest.WithDetail(err.Error()))
		return
	}
	if err := h.k8s.UpdateTraefikConfig(c.Request.Context(), body.YAML); err != nil {
		httputil.RespondError(c, err)
		return
	}
	httputil.RespondOK(c, gin.H{"message": "traefik config updated"})
}

// GetHelmReleases godoc
// @Summary      List Helm releases
// @Tags         cluster
// @Description  Requires admin role.
// @Produce      json
// @Success      200 {array} object
// @Failure      403 {object} apidocs.ProblemDetail
// @Security     BearerAuth
// @Router       /cluster/helm-releases [get]
func (h *ClusterHandler) GetHelmReleases(c *gin.Context) {
	releases, err := h.k8s.GetHelmReleases(c.Request.Context())
	if err != nil {
		httputil.RespondError(c, err)
		return
	}
	httputil.RespondList(c, releases)
}

// GetDaemonSets godoc
// @Summary      List DaemonSets
// @Tags         cluster
// @Description  Requires admin role.
// @Produce      json
// @Success      200 {array} object
// @Failure      403 {object} apidocs.ProblemDetail
// @Security     BearerAuth
// @Router       /cluster/daemonsets [get]
func (h *ClusterHandler) GetDaemonSets(c *gin.Context) {
	daemonSets, err := h.k8s.GetDaemonSets(c.Request.Context())
	if err != nil {
		httputil.RespondError(c, err)
		return
	}
	httputil.RespondList(c, daemonSets)
}

// DeletePVC godoc
// @Summary      Delete PVC
// @Tags         cluster
// @Description  Requires admin role.
// @Produce      json
// @Param        namespace path string true "Namespace"
// @Param        name path string true "PVC name"
// @Success      200 {object} apidocs.MessageResponse
// @Failure      403 {object} apidocs.ProblemDetail
// @Security     BearerAuth
// @Router       /cluster/pvcs/{namespace}/{name} [delete]
func (h *ClusterHandler) DeletePVC(c *gin.Context) {
	namespace := c.Param("namespace")
	name := c.Param("name")
	if namespace == "" || name == "" {
		httputil.RespondError(c, apierr.ErrBadRequest.WithDetail("namespace and name are required"))
		return
	}
	if h.volumes == nil {
		httputil.RespondError(c, apierr.ErrNotImplemented.WithDetail("deploy target does not support volumes"))
		return
	}
	if err := h.volumes.DeleteVolume(c.Request.Context(), name, namespace); err != nil {
		httputil.RespondError(c, err)
		return
	}
	h.notif.NotifyResourceDeleted(middleware.GetOrgID(c), model.EventPVCDeleted,
		name, fmt.Sprintf("Persistent volume claim %q was deleted from namespace %q", name, namespace))
	httputil.RespondOK(c, gin.H{"message": "PVC deleted"})
}

// GetCleanupStats godoc
// @Summary      Get cluster cleanup stats
// @Tags         cluster
// @Description  Requires admin role.
// @Produce      json
// @Success      200 {object} map[string]interface{}
// @Failure      403 {object} apidocs.ProblemDetail
// @Security     BearerAuth
// @Router       /cluster/cleanup/stats [get]
func (h *ClusterHandler) GetCleanupStats(c *gin.Context) {
	ctx := c.Request.Context()
	stats, err := h.k8s.GetCleanupStats(ctx)
	if err != nil {
		httputil.RespondError(c, err)
		return
	}

	// Enrich with orphan ingress detection (requires DB access)
	validHosts := h.getValidDomainHosts(ctx)
	systemIngresses := h.getSystemIngresses(ctx)
	orphans, orphanErr := h.k8s.GetOrphanIngresses(ctx, validHosts, systemIngresses)
	if orphanErr == nil && orphans != nil {
		stats.OrphanIngresses = len(orphans)
		stats.OrphanIngressNames = orphans
	}
	if stats.OrphanIngressNames == nil {
		stats.OrphanIngressNames = []string{}
	}

	httputil.RespondOK(c, stats)
}

// getValidDomainHosts returns the set of all app domain hosts in the database.
func (h *ClusterHandler) getValidDomainHosts(ctx context.Context) map[string]bool {
	hosts := make(map[string]bool)
	// Single query for every domain host instead of listing all apps and then
	// fetching domains app-by-app (an N+1 that scaled with the app count).
	allHosts, _ := h.store.Domains().ListAllHosts(ctx)
	for _, host := range allHosts {
		hosts[host] = true
	}
	return hosts
}

// getSystemIngresses returns system-managed ingresses keyed by "namespace/name"
// with their currently expected host. These are validated by resource identity
// (not the global host list) so that an app ingress sharing the same host is
// not accidentally exempt from orphan cleanup.
func (h *ClusterHandler) getSystemIngresses(ctx context.Context) map[string]string {
	si := make(map[string]string)
	if panelDomain, _ := h.store.Settings().Get(ctx, model.SettingPanelDomain); panelDomain != "" {
		si["orkai/orkai-panel"] = panelDomain
		si["orkai/orkai-panel-http"] = panelDomain
	}
	return si
}

// CleanupEvictedPods godoc
// @Summary      Cleanup evicted pods
// @Tags         cluster
// @Description  Requires admin role.
// @Produce      json
// @Success      200 {object} map[string]interface{}
// @Failure      403 {object} apidocs.ProblemDetail
// @Security     BearerAuth
// @Router       /cluster/cleanup/evicted-pods [post]
func (h *ClusterHandler) CleanupEvictedPods(c *gin.Context) {
	result, err := h.k8s.CleanupEvictedPods(c.Request.Context())
	if err != nil {
		httputil.RespondError(c, err)
		return
	}
	httputil.RespondOK(c, result)
}

// CleanupFailedPods godoc
// @Summary      Cleanup failed pods
// @Tags         cluster
// @Description  Requires admin role.
// @Produce      json
// @Success      200 {object} map[string]interface{}
// @Failure      403 {object} apidocs.ProblemDetail
// @Security     BearerAuth
// @Router       /cluster/cleanup/failed-pods [post]
func (h *ClusterHandler) CleanupFailedPods(c *gin.Context) {
	result, err := h.k8s.CleanupFailedPods(c.Request.Context())
	if err != nil {
		httputil.RespondError(c, err)
		return
	}
	httputil.RespondOK(c, result)
}

// CleanupCompletedPods godoc
// @Summary      Cleanup completed pods
// @Tags         cluster
// @Description  Requires admin role.
// @Produce      json
// @Success      200 {object} map[string]interface{}
// @Failure      403 {object} apidocs.ProblemDetail
// @Security     BearerAuth
// @Router       /cluster/cleanup/completed-pods [post]
func (h *ClusterHandler) CleanupCompletedPods(c *gin.Context) {
	result, err := h.k8s.CleanupCompletedPods(c.Request.Context())
	if err != nil {
		httputil.RespondError(c, err)
		return
	}
	httputil.RespondOK(c, result)
}

// CleanupStaleReplicaSets godoc
// @Summary      Cleanup stale ReplicaSets
// @Tags         cluster
// @Description  Requires admin role.
// @Produce      json
// @Success      200 {object} map[string]interface{}
// @Failure      403 {object} apidocs.ProblemDetail
// @Security     BearerAuth
// @Router       /cluster/cleanup/stale-replicasets [post]
func (h *ClusterHandler) CleanupStaleReplicaSets(c *gin.Context) {
	result, err := h.k8s.CleanupStaleReplicaSets(c.Request.Context())
	if err != nil {
		httputil.RespondError(c, err)
		return
	}
	httputil.RespondOK(c, result)
}

// CleanupCompletedJobs godoc
// @Summary      Cleanup completed jobs
// @Tags         cluster
// @Description  Requires admin role.
// @Produce      json
// @Success      200 {object} map[string]interface{}
// @Failure      403 {object} apidocs.ProblemDetail
// @Security     BearerAuth
// @Router       /cluster/cleanup/completed-jobs [post]
func (h *ClusterHandler) CleanupCompletedJobs(c *gin.Context) {
	result, err := h.k8s.CleanupCompletedJobs(c.Request.Context())
	if err != nil {
		httputil.RespondError(c, err)
		return
	}
	httputil.RespondOK(c, result)
}

// CleanupOrphanIngresses godoc
// @Summary      Cleanup orphan ingresses
// @Tags         cluster
// @Description  Requires admin role.
// @Produce      json
// @Success      200 {object} map[string]interface{}
// @Failure      403 {object} apidocs.ProblemDetail
// @Security     BearerAuth
// @Router       /cluster/cleanup/orphan-ingresses [post]
func (h *ClusterHandler) CleanupOrphanIngresses(c *gin.Context) {
	ctx := c.Request.Context()
	validHosts := h.getValidDomainHosts(ctx)
	systemIngresses := h.getSystemIngresses(ctx)
	result, err := h.k8s.CleanupOrphanIngresses(ctx, validHosts, systemIngresses)
	if err != nil {
		httputil.RespondError(c, err)
		return
	}
	httputil.RespondOK(c, result)
}

// ExpandPVC godoc
// @Summary      Expand PVC
// @Tags         cluster
// @Description  Requires admin role.
// @Accept       json
// @Produce      json
// @Param        namespace path string true "Namespace"
// @Param        name path string true "PVC name"
// @Param        body body object true "Request body"
// @Success      200 {object} apidocs.MessageResponse
// @Failure      403 {object} apidocs.ProblemDetail
// @Security     BearerAuth
// @Router       /cluster/pvcs/{namespace}/{name}/expand [put]
func (h *ClusterHandler) ExpandPVC(c *gin.Context) {
	namespace := c.Param("namespace")
	name := c.Param("name")
	if namespace == "" || name == "" {
		httputil.RespondError(c, apierr.ErrBadRequest.WithDetail("namespace and name are required"))
		return
	}
	var body struct {
		Size string `json:"size" binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		httputil.RespondError(c, apierr.ErrBadRequest.WithDetail(err.Error()))
		return
	}
	if h.volumes == nil {
		httputil.RespondError(c, apierr.ErrNotImplemented.WithDetail("deploy target does not support volumes"))
		return
	}
	if err := h.volumes.ExpandVolume(c.Request.Context(), name, namespace, body.Size); err != nil {
		httputil.RespondError(c, err)
		return
	}
	httputil.RespondOK(c, gin.H{"message": "PVC expanded"})
}

// RestartTraefik godoc
// @Summary      Restart Traefik
// @Tags         cluster
// @Description  Requires admin role.
// @Produce      json
// @Success      200 {object} apidocs.MessageResponse
// @Failure      403 {object} apidocs.ProblemDetail
// @Security     BearerAuth
// @Router       /cluster/traefik-restart [post]
func (h *ClusterHandler) RestartTraefik(c *gin.Context) {
	if err := h.k8s.RestartTraefik(c.Request.Context()); err != nil {
		httputil.RespondError(c, apierr.ErrBadRequest.WithDetail(err.Error()))
		return
	}
	httputil.RespondOK(c, gin.H{"message": "Traefik restarting"})
}

// GetTraefikStatus godoc
// @Summary      Get Traefik status
// @Description  Requires admin role.
// @Tags         cluster
// @Produce      json
// @Success      200 {object} map[string]interface{}
// @Failure      401 {object} apidocs.ProblemDetail
// @Failure      403 {object} apidocs.ProblemDetail
// @Security     BearerAuth
// @Router       /cluster/traefik-status [get]
func (h *ClusterHandler) GetTraefikStatus(c *gin.Context) {
	status, err := h.k8s.GetTraefikStatus(c.Request.Context())
	if err != nil {
		httputil.RespondError(c, err)
		return
	}
	httputil.RespondOK(c, status)
}
