package pg

import (
	"context"
	"database/sql"
	"errors"

	"github.com/google/uuid"
	"github.com/uptrace/bun"

	"github.com/orkai-dev/orkai/apps/api/internal/model"
	"github.com/orkai-dev/orkai/apps/api/internal/secret"
)

type serverNodeStore struct {
	db      bun.IDB
	secrets secret.Store
}

func (s *serverNodeStore) GetByID(ctx context.Context, id uuid.UUID) (*model.ServerNode, error) {
	node := new(model.ServerNode)
	err := s.db.NewSelect().Model(node).Where("id = ?", id).Scan(ctx)
	if err != nil {
		return nil, err
	}
	if err := decryptNode(node, s.secrets); err != nil {
		return nil, err
	}
	return node, nil
}

func (s *serverNodeStore) Create(ctx context.Context, node *model.ServerNode) error {
	enc, err := encryptNodePassword(s.secrets, node.Password)
	if err != nil {
		return err
	}
	node.Password = enc
	_, err = s.db.NewInsert().Model(node).Exec(ctx)
	if err != nil {
		return err
	}
	return decryptNode(node, s.secrets)
}

func (s *serverNodeStore) Update(ctx context.Context, node *model.ServerNode) error {
	enc, err := encryptNodePassword(s.secrets, node.Password)
	if err != nil {
		return err
	}
	node.Password = enc
	_, err = s.db.NewUpdate().Model(node).WherePK().Exec(ctx)
	if err != nil {
		return err
	}
	return decryptNode(node, s.secrets)
}

func (s *serverNodeStore) Delete(ctx context.Context, id uuid.UUID) error {
	_, err := s.db.NewDelete().Model((*model.ServerNode)(nil)).Where("id = ?", id).Exec(ctx)
	return err
}

func (s *serverNodeStore) List(ctx context.Context) ([]model.ServerNode, error) {
	var nodes []model.ServerNode
	err := s.db.NewSelect().Model(&nodes).OrderExpr("created_at DESC").Scan(ctx)
	if err != nil {
		return nil, err
	}
	if err := decryptNodes(nodes, s.secrets); err != nil {
		return nil, err
	}
	return nodes, nil
}

func (s *serverNodeStore) FindBySSHKey(ctx context.Context, sshKeyID uuid.UUID) (*model.ServerNode, error) {
	node := new(model.ServerNode)
	// Only id+name are needed for the delete-guard; skipping other columns also
	// avoids decrypting the stored password for an unrelated check.
	err := s.db.NewSelect().
		Model(node).
		Column("id", "name").
		Where("ssh_key_id = ?", sshKeyID).
		Limit(1).
		Scan(ctx)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return node, nil
}

func (s *serverNodeStore) UpdateStatus(ctx context.Context, id uuid.UUID, status model.NodeStatus, msg string) error {
	_, err := s.db.NewUpdate().
		Model((*model.ServerNode)(nil)).
		Set("status = ?", status).
		Set("status_msg = ?", msg).
		Where("id = ?", id).
		Exec(ctx)
	return err
}
