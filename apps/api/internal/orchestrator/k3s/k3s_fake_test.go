//go:build integration

package k3s

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestK3sFakeGetNodes(t *testing.T) {
	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "node-1",
			Labels: map[string]string{"node-role.kubernetes.io/control-plane": ""},
		},
		Status: corev1.NodeStatus{
			Conditions: []corev1.NodeCondition{{Type: "Ready", Status: "True"}},
			Addresses:  []corev1.NodeAddress{{Type: corev1.NodeInternalIP, Address: "10.0.0.1"}},
		},
	}
	o := &Orchestrator{client: fake.NewSimpleClientset(node), logger: fakeLogger()}

	nodes, err := o.GetNodes(context.Background())
	require.NoError(t, err)
	require.Len(t, nodes, 1)
	assert.Equal(t, "node-1", nodes[0].Name)
	assert.Equal(t, "Ready", nodes[0].Status)
	assert.Contains(t, nodes[0].Roles, "control-plane")
}

func TestK3sFakeNamespaceLifecycle(t *testing.T) {
	o := &Orchestrator{client: fake.NewSimpleClientset(), logger: fakeLogger()}
	ctx := context.Background()

	require.NoError(t, o.CreateNamespace(ctx, "team-alpha"))

	namespaces, err := o.GetNamespaces(ctx)
	require.NoError(t, err)
	found := false
	for _, ns := range namespaces {
		if ns.Name == "team-alpha" {
			found = true
		}
	}
	assert.True(t, found, "created namespace should be listed")

	require.NoError(t, o.DeleteNamespace(ctx, "team-alpha"))
	// Deleting a missing namespace is a no-op (no error).
	require.NoError(t, o.DeleteNamespace(ctx, "does-not-exist"))
}
