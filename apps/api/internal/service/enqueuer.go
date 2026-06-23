package service

import (
	"context"

	"github.com/orkai-dev/orkai/apps/api/internal/jobs"
)

// Enqueuer enqueues durable background jobs (PGMQ).
type Enqueuer interface {
	Enqueue(ctx context.Context, job jobs.Job) error
}
