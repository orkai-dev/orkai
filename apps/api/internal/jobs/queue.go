package jobs

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/uptrace/bun"
	"github.com/uptrace/bun/driver/pgdriver"
)

// Postgres SQLSTATE codes for "object already exists" raised when the queue's
// backing table/function/etc. is created concurrently or already present.
var queueExistsSQLStates = map[string]struct{}{
	"42P07": {}, // duplicate_table
	"42P06": {}, // duplicate_schema
	"42723": {}, // duplicate_function
	"42710": {}, // duplicate_object
	"23505": {}, // unique_violation (pgmq.meta row)
}

// Message is a PGMQ message read from the queue.
type Message struct {
	MsgID  int64
	ReadCt int
	Job    Job
}

// Queue wraps PGMQ SQL functions over a Bun DB connection.
type Queue struct {
	db *bun.DB
}

// NewQueue creates a Queue backed by the given Bun DB.
func NewQueue(db *bun.DB) *Queue {
	return &Queue{db: db}
}

// EnsureSchema creates the PGMQ extension and orkai_jobs queue if missing.
func (q *Queue) EnsureSchema(ctx context.Context) error {
	if _, err := q.db.ExecContext(ctx, `CREATE EXTENSION IF NOT EXISTS pgmq`); err != nil {
		return fmt.Errorf("create pgmq extension: %w", err)
	}
	if _, err := q.db.ExecContext(ctx, `SELECT pgmq.create(?)`, QueueName); err != nil {
		if !isQueueExistsError(err) {
			return fmt.Errorf("create queue %q: %w", QueueName, err)
		}
	}
	return nil
}

func isQueueExistsError(err error) bool {
	if err == nil {
		return false
	}
	var pgErr pgdriver.Error
	if !errors.As(err, &pgErr) {
		return false
	}
	_, ok := queueExistsSQLStates[pgErr.Field('C')]
	return ok
}

// Enqueue sends a job to the orkai_jobs queue.
func (q *Queue) Enqueue(ctx context.Context, job Job) error {
	payload, err := json.Marshal(job)
	if err != nil {
		return fmt.Errorf("marshal job: %w", err)
	}
	_, err = q.db.ExecContext(ctx, `SELECT pgmq.send(?, ?::jsonb)`, QueueName, string(payload))
	if err != nil {
		return fmt.Errorf("pgmq.send: %w", err)
	}
	return nil
}

type pgmqRow struct {
	MsgID   int64           `bun:"msg_id"`
	ReadCt  int             `bun:"read_ct"`
	Message json.RawMessage `bun:"message"`
}

// Read pulls up to qty messages with the given visibility timeout (seconds).
func (q *Queue) Read(ctx context.Context, vtSeconds int, qty int) ([]Message, error) {
	var rows []pgmqRow
	err := q.db.NewRaw(
		`SELECT msg_id, read_ct, message FROM pgmq.read(?, ?, ?)`,
		QueueName, vtSeconds, qty,
	).Scan(ctx, &rows)
	if err != nil {
		return nil, fmt.Errorf("pgmq.read: %w", err)
	}

	out := make([]Message, 0, len(rows))
	for _, row := range rows {
		var job Job
		if err := json.Unmarshal(row.Message, &job); err != nil {
			return nil, fmt.Errorf("unmarshal job msg_id=%d: %w", row.MsgID, err)
		}
		out = append(out, Message{MsgID: row.MsgID, ReadCt: row.ReadCt, Job: job})
	}
	return out, nil
}

// Delete removes a message from the queue after successful processing.
func (q *Queue) Delete(ctx context.Context, msgID int64) error {
	_, err := q.db.ExecContext(ctx, `SELECT pgmq.delete(?, ?)`, QueueName, msgID)
	if err != nil {
		return fmt.Errorf("pgmq.delete: %w", err)
	}
	return nil
}

// Archive moves a message to the archive table (permanent failure / poison).
func (q *Queue) Archive(ctx context.Context, msgID int64) error {
	_, err := q.db.ExecContext(ctx, `SELECT pgmq.archive(?, ?)`, QueueName, msgID)
	if err != nil {
		return fmt.Errorf("pgmq.archive: %w", err)
	}
	return nil
}
