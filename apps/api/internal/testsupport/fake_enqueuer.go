package testsupport

import (
	"context"
	"sync"

	"github.com/orkai-dev/orkai/apps/api/internal/jobs"
)

// FakeEnqueuer records enqueued jobs for tests.
type FakeEnqueuer struct {
	mu    sync.Mutex
	Jobs  []jobs.Job
	Err   error
	Calls int
}

func NewFakeEnqueuer() *FakeEnqueuer {
	return &FakeEnqueuer{}
}

func (f *FakeEnqueuer) Enqueue(ctx context.Context, job jobs.Job) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.Calls++
	if f.Err != nil {
		return f.Err
	}
	f.Jobs = append(f.Jobs, job)
	return nil
}

func (f *FakeEnqueuer) LastJob() (jobs.Job, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.Jobs) == 0 {
		return jobs.Job{}, false
	}
	return f.Jobs[len(f.Jobs)-1], true
}
