package leader

import (
	"log/slog"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNilElectorIsAlwaysLeader(t *testing.T) {
	var e *Elector
	assert.True(t, e.IsLeader())
}

func TestElectorStartsNotLeader(t *testing.T) {
	e := &Elector{key: ServerLeaderKey}
	assert.False(t, e.IsLeader())
}

func TestElectorLeaderState(t *testing.T) {
	e := &Elector{
		key:    WorkerLeaderKey,
		logger: slog.New(slog.NewTextHandler(os.Stdout, nil)),
	}
	e.mu.Lock()
	e.leader = true
	e.mu.Unlock()
	assert.True(t, e.IsLeader())
}
