package leader

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/uptrace/bun"
)

const (
	// ServerLeaderKey elects the API server replica that runs singleton loops
	// (metrics collect/evaluate/cleanup, ECR refresh, version check).
	ServerLeaderKey int64 = 91001
	// WorkerLeaderKey elects the worker replica that runs singleton schedulers
	// (backup schedulers, stale-deployment recovery).
	WorkerLeaderKey int64 = 91002

	electionInterval = 10 * time.Second
)

// Elector acquires a Postgres session advisory lock so only one process per
// role acts as leader. The lock is held on a dedicated *sql.Conn for the
// lifetime of leadership.
type Elector struct {
	db     *bun.DB
	key    int64
	logger *slog.Logger

	mu     sync.RWMutex
	leader bool
	conn   bun.Conn
}

// NewElector creates an elector for the given advisory-lock key.
func NewElector(db *bun.DB, key int64, logger *slog.Logger) *Elector {
	return &Elector{db: db, key: key, logger: logger}
}

// IsLeader reports whether this process currently holds the advisory lock.
// A nil elector is treated as always leader (tests / noop paths).
func (e *Elector) IsLeader() bool {
	if e == nil {
		return true
	}
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.leader
}

// TryElect attempts to acquire leadership once. Call synchronously before
// starting singleton background loops so IsLeader() is accurate on startup.
func (e *Elector) TryElect(ctx context.Context) {
	e.tick(ctx)
}

// Run loops until ctx is cancelled, attempting to acquire or renew leadership.
// Callers should invoke TryElect synchronously before starting Run so IsLeader()
// is accurate on startup.
func (e *Elector) Run(ctx context.Context) {
	ticker := time.NewTicker(electionInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			e.release()
			return
		case <-ticker.C:
			e.tick(ctx)
		}
	}
}

func (e *Elector) tick(ctx context.Context) {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.leader {
		if err := e.conn.Conn.PingContext(ctx); err != nil {
			e.logger.Warn("leader connection lost, stepping down",
				slog.Int64("key", e.key),
				slog.Any("error", err),
			)
			e.discardConnLocked()
		}
		return
	}

	conn, err := e.db.Conn(ctx)
	if err != nil {
		e.logger.Warn("leader election: failed to acquire connection",
			slog.Int64("key", e.key),
			slog.Any("error", err),
		)
		return
	}

	var acquired bool
	if err := conn.QueryRowContext(ctx, "SELECT pg_try_advisory_lock(?)", e.key).Scan(&acquired); err != nil {
		_ = conn.Conn.Close()
		e.logger.Warn("leader election: lock query failed",
			slog.Int64("key", e.key),
			slog.Any("error", err),
		)
		return
	}

	if !acquired {
		_ = conn.Conn.Close()
		return
	}

	e.conn = conn
	e.leader = true
	e.logger.Info("became leader", slog.Int64("key", e.key))
}

func (e *Elector) release() {
	e.mu.Lock()
	defer e.mu.Unlock()
	if !e.leader {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, _ = e.conn.ExecContext(ctx, "SELECT pg_advisory_unlock(?)", e.key)
	e.discardConnLocked()
	e.logger.Info("released leadership", slog.Int64("key", e.key))
}

func (e *Elector) discardConnLocked() {
	if e.conn.Conn != nil {
		_ = e.conn.Conn.Close()
		e.conn = bun.Conn{}
	}
	e.leader = false
}
