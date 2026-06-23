package service

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/orkai-dev/orkai/apps/api/internal/model"
	"github.com/orkai-dev/orkai/apps/api/internal/orchestrator"
	"github.com/orkai-dev/orkai/apps/api/internal/testsupport"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newNodeService(fs *testsupport.FakeStore, orch *testsupport.FakeOrchestrator) *NodeService {
	return NewNodeService(fs, testsupport.NewFakeTargetRegistry(orch), testsupport.NewTestLogger(), nil)
}

func TestNodeSubscribeBroadcastUnsubscribe(t *testing.T) {
	s := newNodeService(testsupport.NewFakeStore(), testsupport.NewFakeOrchestrator())
	id := uuid.New()
	ch := s.SubscribeLogs(id)
	s.broadcast(id, "hello")
	assert.Equal(t, "hello", <-ch)
	s.UnsubscribeLogs(id, ch)
	_, ok := <-ch
	assert.False(t, ok, "channel should be closed after unsubscribe")
}

func TestNodeList(t *testing.T) {
	fs := testsupport.NewFakeStore()
	fs.ServerNodesStore.ListFn = func(ctx context.Context) ([]model.ServerNode, error) {
		return []model.ServerNode{{}}, nil
	}
	s := newNodeService(fs, testsupport.NewFakeOrchestrator())
	nodes, err := s.List(context.Background())
	require.NoError(t, err)
	assert.Len(t, nodes, 1)
}

func TestNodeGetByID(t *testing.T) {
	fs := testsupport.NewFakeStore()
	id := uuid.New()
	fs.ServerNodesStore.GetByIDFn = func(ctx context.Context, _ uuid.UUID) (*model.ServerNode, error) {
		return &model.ServerNode{BaseModel: model.BaseModel{ID: id}}, nil
	}
	s := newNodeService(fs, testsupport.NewFakeOrchestrator())
	got, err := s.GetByID(context.Background(), id)
	require.NoError(t, err)
	assert.Equal(t, id, got.ID)
}

func TestNodeCreateDefaults(t *testing.T) {
	fs := testsupport.NewFakeStore()
	s := newNodeService(fs, testsupport.NewFakeOrchestrator())
	node, err := s.Create(context.Background(), uuid.New(), CreateNodeInput{Name: "n1", Host: "1.2.3.4"})
	require.NoError(t, err)
	assert.Equal(t, 22, node.Port)
	assert.Equal(t, "root", node.SSHUser)
	assert.Equal(t, "worker", node.Role)
	assert.Equal(t, "password", node.AuthType)
}

func TestNodeCreateStoresPassword(t *testing.T) {
	fs := testsupport.NewFakeStore()
	s := newNodeService(fs, testsupport.NewFakeOrchestrator())
	node, err := s.Create(context.Background(), uuid.New(), CreateNodeInput{Name: "n1", Host: "h", Password: "secret"})
	require.NoError(t, err)
	assert.Equal(t, "secret", node.Password)
}

func TestNodeCreateSSHKeyNotFound(t *testing.T) {
	fs := testsupport.NewFakeStore()
	keyID := uuid.New()
	fs.SharedResourcesStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.SharedResource, error) {
		return nil, errors.New("missing")
	}
	s := newNodeService(fs, testsupport.NewFakeOrchestrator())
	_, err := s.Create(context.Background(), uuid.New(), CreateNodeInput{Name: "n1", Host: "h", SSHKeyID: &keyID})
	require.ErrorContains(t, err, "SSH key not found")
}

func TestNodeCreateSSHKeyWrongOrg(t *testing.T) {
	fs := testsupport.NewFakeStore()
	keyID := uuid.New()
	fs.SharedResourcesStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.SharedResource, error) {
		return &model.SharedResource{OrgID: uuid.New()}, nil
	}
	s := newNodeService(fs, testsupport.NewFakeOrchestrator())
	_, err := s.Create(context.Background(), uuid.New(), CreateNodeInput{Name: "n1", Host: "h", SSHKeyID: &keyID})
	require.ErrorContains(t, err, "does not belong")
}

func TestNodeCreateStoreError(t *testing.T) {
	fs := testsupport.NewFakeStore()
	fs.ServerNodesStore.CreateFn = func(ctx context.Context, n *model.ServerNode) error {
		return errors.New("insert failed")
	}
	s := newNodeService(fs, testsupport.NewFakeOrchestrator())
	_, err := s.Create(context.Background(), uuid.New(), CreateNodeInput{Name: "n1", Host: "h"})
	require.Error(t, err)
}

func TestNodeDelete(t *testing.T) {
	fs := testsupport.NewFakeStore()
	fs.ServerNodesStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.ServerNode, error) {
		return &model.ServerNode{Name: "n1", Host: "h"}, nil
	}
	s := newNodeService(fs, testsupport.NewFakeOrchestrator())
	require.NoError(t, s.Delete(context.Background(), uuid.New(), uuid.New()))
}

func TestNodeInitializeGetError(t *testing.T) {
	fs := testsupport.NewFakeStore()
	fs.ServerNodesStore.GetByIDFn = func(ctx context.Context, id uuid.UUID) (*model.ServerNode, error) {
		return nil, errors.New("missing")
	}
	s := newNodeService(fs, testsupport.NewFakeOrchestrator())
	require.Error(t, s.Initialize(context.Background(), uuid.New()))
}

func TestNodeGetK3sToken(t *testing.T) {
	fs := testsupport.NewFakeStore()
	s := newNodeService(fs, testsupport.NewFakeOrchestrator())
	assert.Empty(t, s.getK3sToken(context.Background()))

	fs.SettingsStore.GetFn = func(ctx context.Context, key string) (string, error) {
		return "tok123", nil
	}
	assert.Equal(t, "tok123", s.getK3sToken(context.Background()))
}

func TestNodeGetK3sServerInfo(t *testing.T) {
	fs := testsupport.NewFakeStore()
	orch := testsupport.NewFakeOrchestrator()
	orch.GetNodesFn = func(ctx context.Context) ([]orchestrator.NodeInfo, error) {
		return nil, errors.New("no cluster")
	}
	s := newNodeService(fs, orch)
	ip, _ := s.getK3sServerInfo()
	assert.Equal(t, "127.0.0.1", ip)

	orch.GetNodesFn = func(ctx context.Context) ([]orchestrator.NodeInfo, error) {
		return []orchestrator.NodeInfo{
			{Name: "w1", IP: "10.0.0.2", Roles: []string{"worker"}},
			{Name: "cp", IP: "10.0.0.1", Roles: []string{"control-plane"}},
		}, nil
	}
	ip, name := s.getK3sServerInfo()
	assert.Equal(t, "10.0.0.1", ip)
	assert.Equal(t, "cp", name)
}

func TestNodeGetK3sServerInfoNoRoleMatch(t *testing.T) {
	fs := testsupport.NewFakeStore()
	orch := testsupport.NewFakeOrchestrator()
	orch.GetNodesFn = func(ctx context.Context) ([]orchestrator.NodeInfo, error) {
		return []orchestrator.NodeInfo{{Name: "w1", IP: "10.0.0.2", Roles: []string{"worker"}}}, nil
	}
	s := newNodeService(fs, orch)
	ip, name := s.getK3sServerInfo()
	assert.Equal(t, "10.0.0.2", ip)
	assert.Equal(t, "w1", name)
}
