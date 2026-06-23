package k3s

import (
	"context"
	"io"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/tools/remotecommand"

	"github.com/orkai-dev/orkai/apps/api/internal/model"
	"github.com/orkai-dev/orkai/apps/api/internal/orchestrator"
)

func TestTerminalSessionResizeAfterCloseDoesNotPanic(t *testing.T) {
	s := &terminalSession{}
	s.stdinR, s.stdinW = io.Pipe()
	s.stdoutR, s.stdoutW = io.Pipe()
	require.NoError(t, s.Resize(80, 24))
	require.NoError(t, s.Close())
	require.NotPanics(t, func() {
		_ = s.Resize(120, 40)
	})
}

func TestTerminalSessionConcurrentReadWriteResize(t *testing.T) {
	s := &terminalSession{}
	s.stdinR, s.stdinW = io.Pipe()
	s.stdoutR, s.stdoutW = io.Pipe()

	done := make(chan struct{})
	go func() {
		defer close(done)
		buf := make([]byte, 16)
		_, _ = s.Read(buf)
	}()
	require.NoError(t, s.Resize(80, 24))
	go func() {
		_, _ = s.stdoutW.Write([]byte("prompt"))
	}()
	<-done
	require.NoError(t, s.Close())
}

func TestTerminalSessionNextAfterClose(t *testing.T) {
	s := &terminalSession{}
	s.stdinR, s.stdinW = io.Pipe()
	s.stdoutR, s.stdoutW = io.Pipe()
	require.NoError(t, s.Resize(80, 24))
	require.NotNil(t, s.Next())
	require.NoError(t, s.Close())
	assert.Nil(t, s.Next())
}

func TestTerminalSessionDoubleClose(t *testing.T) {
	s := &terminalSession{}
	s.stdinR, s.stdinW = io.Pipe()
	s.stdoutR, s.stdoutW = io.Pipe()
	require.NoError(t, s.Close())
	require.NoError(t, s.Close())
}

func TestExecTerminalNoRunningPods(t *testing.T) {
	app := &model.Application{
		Name:      "web",
		Namespace: "team-a",
		K8sName:   "web",
	}
	o := &Orchestrator{
		client: fake.NewSimpleClientset(),
		logger: fakeLogger(),
	}
	_, err := o.ExecTerminal(context.Background(), app, orchestrator.ExecOpts{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no running pods found")
}

func TestTerminalSessionResizeQueuesSize(t *testing.T) {
	s := &terminalSession{}
	s.stdinR, s.stdinW = io.Pipe()
	s.stdoutR, s.stdoutW = io.Pipe()
	require.NoError(t, s.Resize(80, 24))

	done := make(chan *remotecommand.TerminalSize, 1)
	go func() {
		done <- s.Next()
	}()

	select {
	case size := <-done:
		require.NotNil(t, size)
		assert.Equal(t, uint16(80), size.Width)
		assert.Equal(t, uint16(24), size.Height)
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for resize")
	}
}
