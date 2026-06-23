package v1

import (
	"testing"

	"github.com/google/uuid"
	"github.com/orkai-dev/orkai/apps/api/internal/testsupport"
	"github.com/stretchr/testify/assert"
)

func newClusterHandler() *ClusterHandler {
	return NewClusterHandler(testsupport.NewFakeTargetRegistry(), testsupport.NewFakeStore(), nil)
}

func TestClusterGetCapabilities(t *testing.T) {
	h := newClusterHandler()
	r, _, _ := newAuthedRouter()
	r.GET("/capabilities", h.GetCapabilities)
	resp := doJSON(r, "GET", "/capabilities", nil)
	assert.Equal(t, 200, resp.Code)
}

func TestClusterReadEndpoints(t *testing.T) {
	h := newClusterHandler()
	r, _, _ := newAuthedRouter()
	r.GET("/nodes", h.GetNodes)
	r.GET("/metrics", h.GetMetrics)
	r.GET("/pods", h.GetAllPods)
	r.GET("/events", h.GetEvents)
	r.GET("/pvcs", h.GetPVCs)
	r.GET("/namespaces", h.GetNamespaces)
	r.GET("/node-metrics", h.GetNodeMetrics)
	r.GET("/topology", h.GetTopology)
	r.GET("/pools", h.GetNodePools)
	r.GET("/traefik/config", h.GetTraefikConfig)
	r.GET("/traefik/status", h.GetTraefikStatus)
	r.GET("/helm", h.GetHelmReleases)
	r.GET("/daemonsets", h.GetDaemonSets)
	r.GET("/cleanup/stats", h.GetCleanupStats)

	for _, path := range []string{
		"/nodes", "/metrics", "/pods", "/events?limit=50", "/pvcs", "/namespaces",
		"/node-metrics", "/topology", "/pools", "/traefik/config", "/traefik/status",
		"/helm", "/daemonsets", "/cleanup/stats",
	} {
		assert.Equal(t, 200, doJSON(r, "GET", path, nil).Code, path)
	}
}

func TestClusterCleanupEndpoints(t *testing.T) {
	h := newClusterHandler()
	r, _, _ := newAuthedRouter()
	r.POST("/cleanup/evicted", h.CleanupEvictedPods)
	r.POST("/cleanup/failed", h.CleanupFailedPods)
	r.POST("/cleanup/completed", h.CleanupCompletedPods)
	r.POST("/cleanup/replicasets", h.CleanupStaleReplicaSets)
	r.POST("/cleanup/jobs", h.CleanupCompletedJobs)
	r.POST("/cleanup/ingresses", h.CleanupOrphanIngresses)
	r.POST("/traefik/restart", h.RestartTraefik)

	for _, path := range []string{
		"/cleanup/evicted", "/cleanup/failed", "/cleanup/completed",
		"/cleanup/replicasets", "/cleanup/jobs", "/cleanup/ingresses", "/traefik/restart",
	} {
		assert.Equal(t, 200, doJSON(r, "POST", path, nil).Code, path)
	}
}

func TestClusterSetNodePool(t *testing.T) {
	h := newClusterHandler()
	r, _, _ := newAuthedRouter()
	r.POST("/nodes/:name/pool", h.SetNodePool)
	assert.Equal(t, 200, doJSON(r, "POST", "/nodes/n1/pool", map[string]any{"pool": "gpu"}).Code)
	assert.Equal(t, 200, doJSON(r, "POST", "/nodes/n1/pool", map[string]any{"pool": ""}).Code)
}

func TestClusterUpdateTraefikConfig(t *testing.T) {
	h := newClusterHandler()
	r, _, _ := newAuthedRouter()
	r.PUT("/traefik/config", h.UpdateTraefikConfig)
	assert.Equal(t, 400, doJSON(r, "PUT", "/traefik/config", map[string]any{}).Code)
	assert.Equal(t, 200, doJSON(r, "PUT", "/traefik/config", map[string]any{"yaml": "x: 1"}).Code)
}

func TestClusterPVCEndpoints(t *testing.T) {
	h := newClusterHandler()
	r, _, _ := newAuthedRouter()
	r.DELETE("/pvc/:namespace/:name", h.DeletePVC)
	r.POST("/pvc/:namespace/:name/expand", h.ExpandPVC)
	assert.Equal(t, 200, doJSON(r, "DELETE", "/pvc/default/"+uuid.New().String(), nil).Code)
	assert.Equal(t, 400, doJSON(r, "POST", "/pvc/default/v1/expand", map[string]any{}).Code)
	assert.Equal(t, 200, doJSON(r, "POST", "/pvc/default/v1/expand", map[string]any{"size": "2Gi"}).Code)
}
