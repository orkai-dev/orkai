package service

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/google/uuid"

	"github.com/orkai-dev/orkai/apps/api/internal/model"
	"github.com/orkai-dev/orkai/apps/api/internal/store"
)

// NotifyResourceDeleted fires an async deletion notification for an org whose ID
// is already known (projects, shared resources, teams, cluster-scoped objects).
// It no-ops when the service is nil so delete paths stay decoupled from the
// notification subsystem in tests and kubeconfig-less setups.
func (s *NotificationService) NotifyResourceDeleted(orgID uuid.UUID, event model.NotifyEvent, name, detail string) {
	if s == nil {
		return
	}
	s.NotifyAsync(orgID, event, fmt.Sprintf("%s deleted", name), detail)
}

// notifyProjectResourceDeleted resolves the owning org from a project and fires
// an async deletion notification. Used by resources that belong to a project
// (apps, pages, workers, databases, cron jobs, domains). It no-ops when notif is
// nil and logs (without failing the delete) if the project can't be resolved.
func notifyProjectResourceDeleted(notif *NotificationService, st store.Store, logger *slog.Logger, projectID uuid.UUID, event model.NotifyEvent, name, detail string) {
	if notif == nil {
		return
	}
	project, err := st.Projects().GetByID(context.Background(), projectID)
	if err != nil {
		if logger != nil {
			logger.Warn("failed to resolve org for delete notification",
				slog.String("event", string(event)),
				slog.Any("error", err),
			)
		}
		return
	}
	notif.NotifyResourceDeleted(project.OrgID, event, name, detail)
}
