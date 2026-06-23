package ws

import (
	"bufio"
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/orkai-dev/orkai/apps/api/internal/api/middleware"
	"github.com/orkai-dev/orkai/apps/api/internal/auth"
	"github.com/orkai-dev/orkai/apps/api/internal/event"
	"github.com/orkai-dev/orkai/apps/api/internal/model"
	"github.com/orkai-dev/orkai/apps/api/internal/orchestrator"
	"github.com/orkai-dev/orkai/apps/api/internal/service"
	"github.com/orkai-dev/orkai/apps/api/internal/testsupport"
)

func init() { gin.SetMode(gin.TestMode) }

func wsTestLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func wsTestJWT() *auth.JWTManager {
	return auth.NewJWTManager("ws-test-secret", time.Hour, 24*time.Hour)
}

type wsTestAuth struct {
	userID uuid.UUID
	orgID  uuid.UUID
	token  string
}

func newWSTestAuth(role string) wsTestAuth {
	jm := wsTestJWT()
	uid, oid := uuid.New(), uuid.New()
	tokens, err := jm.GenerateTokenPair(uid, oid, role, 0)
	if err != nil {
		panic(err)
	}
	return wsTestAuth{userID: uid, orgID: oid, token: tokens.AccessToken}
}

func newWSRouter(authInfo wsTestAuth, register func(r gin.IRoutes)) *httptest.Server {
	jm := wsTestJWT()
	fs := testsupport.NewFakeStore()
	uid := authInfo.userID
	fs.UsersStore.GetByIDFn = func(_ context.Context, id uuid.UUID) (*model.User, error) {
		if id == uid {
			return &model.User{BaseModel: model.BaseModel{ID: uid}, TokenVersion: 0}, nil
		}
		return nil, errors.New("not found")
	}
	sv := middleware.NewSessionValidator(jm, fs.UsersStore, service.NewAPIKeyService(fs, wsTestLogger(), nil))
	r := gin.New()
	r.Use(sv.WSAuth())
	register(r)
	return httptest.NewServer(r)
}

type chanSubscriber struct {
	ch   chan event.Change
	once sync.Once
}

func newChanSubscriber() *chanSubscriber {
	return &chanSubscriber{ch: make(chan event.Change, 8)}
}

func (s *chanSubscriber) Changes() <-chan event.Change { return s.ch }
func (s *chanSubscriber) Close() error {
	s.once.Do(func() { close(s.ch) })
	return nil
}

func TestSSEBrokerDeliversChange(t *testing.T) {
	broker := NewSSEBroker(wsTestLogger())
	sub := newChanSubscriber()
	go broker.Run(sub)

	authInfo := newWSTestAuth("admin")
	srv := newWSRouter(authInfo, func(r gin.IRoutes) {
		r.GET("/ws/events", broker.ServeHTTP)
	})
	defer srv.Close()

	req, err := http.NewRequest(http.MethodGet, srv.URL+"/ws/events?token="+authInfo.token, nil)
	require.NoError(t, err)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	assert.Equal(t, "text/event-stream", resp.Header.Get("Content-Type"))

	bodyCh := make(chan string, 1)
	go func() {
		scanner := bufio.NewScanner(resp.Body)
		var b strings.Builder
		for scanner.Scan() {
			line := scanner.Text()
			b.WriteString(line)
			b.WriteByte('\n')
			if strings.Contains(line, "data:") && strings.Contains(line, "applications") {
				bodyCh <- b.String()
				return
			}
		}
	}()

	// Wait for ServeHTTP to subscribe before publishing the change.
	require.Eventually(t, func() bool {
		broker.mu.RLock()
		defer broker.mu.RUnlock()
		return len(broker.clients) > 0
	}, time.Second, 10*time.Millisecond)

	sub.ch <- event.Change{Table: "applications", Op: "UPDATE", ID: uuid.NewString()}

	select {
	case body := <-bodyCh:
		assert.Contains(t, body, "applications")
	case <-time.After(10 * time.Second):
		t.Fatal("timed out waiting for SSE payload")
	}

	_ = sub.Close()
}

type fakeTerminalSession struct {
	readCh  chan []byte
	writeCh chan []byte
	mu      sync.Mutex
	resizes [][2]uint16
	once    sync.Once
}

func newFakeTerminalSession(initial string) *fakeTerminalSession {
	s := &fakeTerminalSession{
		readCh:  make(chan []byte, 1),
		writeCh: make(chan []byte, 1),
	}
	if initial != "" {
		s.readCh <- []byte(initial)
	}
	return s
}

func (s *fakeTerminalSession) Read(p []byte) (int, error) {
	select {
	case b, ok := <-s.readCh:
		if !ok {
			return 0, io.EOF
		}
		return copy(p, b), nil
	case <-time.After(2 * time.Second):
		return 0, io.EOF
	}
}

func (s *fakeTerminalSession) Write(p []byte) (int, error) {
	s.writeCh <- append([]byte(nil), p...)
	return len(p), nil
}

func (s *fakeTerminalSession) Resize(width, height uint16) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.resizes = append(s.resizes, [2]uint16{width, height})
	return nil
}

func (s *fakeTerminalSession) lastResize() (uint16, uint16, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.resizes) == 0 {
		return 0, 0, false
	}
	last := s.resizes[len(s.resizes)-1]
	return last[0], last[1], true
}

func (s *fakeTerminalSession) Close() error {
	s.once.Do(func() { close(s.readCh) })
	return nil
}

func wireAppOrg(fs *testsupport.FakeStore, orgID uuid.UUID) (appID, projectID uuid.UUID) {
	appID = uuid.New()
	projectID = uuid.New()
	fs.ApplicationsStore.GetByIDFn = func(_ context.Context, id uuid.UUID) (*model.Application, error) {
		return &model.Application{BaseModel: model.BaseModel{ID: appID}, ProjectID: projectID, Name: "web"}, nil
	}
	fs.ProjectsStore.GetByIDFn = func(_ context.Context, id uuid.UUID) (*model.Project, error) {
		return &model.Project{BaseModel: model.BaseModel{ID: projectID}, OrgID: orgID}, nil
	}
	return appID, projectID
}

func TestTerminalHandlerPreUpgradeErrors(t *testing.T) {
	authInfo := newWSTestAuth("admin")
	fs := testsupport.NewFakeStore()
	orch := testsupport.NewFakeOrchestrator()
	h := NewTerminalHandler(fs, testsupport.NewFakeTargetRegistry(orch), wsTestLogger())

	t.Run("bad app id", func(t *testing.T) {
		srv := newWSRouter(authInfo, func(r gin.IRoutes) {
			r.GET("/ws/terminal/:appId", h.Handle)
		})
		defer srv.Close()
		resp, err := http.Get(srv.URL + "/ws/terminal/not-a-uuid?token=" + authInfo.token)
		require.NoError(t, err)
		defer func() { _ = resp.Body.Close() }()
		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	})

	t.Run("org mismatch", func(t *testing.T) {
		appID, _ := wireAppOrg(fs, uuid.New())
		srv := newWSRouter(authInfo, func(r gin.IRoutes) {
			r.GET("/ws/terminal/:appId", h.Handle)
		})
		defer srv.Close()
		resp, err := http.Get(srv.URL + "/ws/terminal/" + appID.String() + "?token=" + authInfo.token)
		require.NoError(t, err)
		defer func() { _ = resp.Body.Close() }()
		assert.Equal(t, http.StatusForbidden, resp.StatusCode)
	})
}

func TestTerminalHandlerStreamsAndResize(t *testing.T) {
	authInfo := newWSTestAuth("admin")
	fs := testsupport.NewFakeStore()
	appID, _ := wireAppOrg(fs, authInfo.orgID)
	session := newFakeTerminalSession("hello shell")
	orch := testsupport.NewFakeOrchestrator()
	orch.ExecTerminalFn = func(_ context.Context, _ *model.Application, _ orchestrator.ExecOpts) (orchestrator.TerminalSession, error) {
		return session, nil
	}
	h := NewTerminalHandler(fs, testsupport.NewFakeTargetRegistry(orch), wsTestLogger())

	srv := newWSRouter(authInfo, func(r gin.IRoutes) {
		r.GET("/ws/terminal/:appId", h.Handle)
	})
	defer srv.Close()

	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http") + "/ws/terminal/" + appID.String() + "?token=" + authInfo.token
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	require.NoError(t, err)
	defer func() { _ = conn.Close() }()

	_, msg, err := conn.ReadMessage()
	require.NoError(t, err)
	assert.Equal(t, "hello shell", string(msg))

	require.NoError(t, conn.WriteMessage(websocket.TextMessage, []byte(`{"type":"resize","cols":120,"rows":40}`)))
	require.Eventually(t, func() bool {
		w, h, ok := session.lastResize()
		return ok && w == 120 && h == 40
	}, time.Second, 10*time.Millisecond)
}

func TestLogsHandlerOrgMismatch(t *testing.T) {
	authInfo := newWSTestAuth("admin")
	fs := testsupport.NewFakeStore()
	appID, _ := wireAppOrg(fs, uuid.New())
	orch := testsupport.NewFakeOrchestrator()
	h := NewLogsHandler(fs, testsupport.NewFakeTargetRegistry(orch), wsTestLogger())

	srv := newWSRouter(authInfo, func(r gin.IRoutes) {
		r.GET("/ws/logs/:appId", h.Handle)
	})
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/ws/logs/" + appID.String() + "?token=" + authInfo.token)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	assert.Equal(t, http.StatusForbidden, resp.StatusCode)
}

func TestLogsHandlerStreamsLines(t *testing.T) {
	authInfo := newWSTestAuth("admin")
	fs := testsupport.NewFakeStore()
	appID, _ := wireAppOrg(fs, authInfo.orgID)
	orch := testsupport.NewFakeOrchestrator()
	orch.StreamLogsFn = func(_ context.Context, _ *model.Application, _ orchestrator.LogOpts) (io.ReadCloser, error) {
		return io.NopCloser(strings.NewReader("line-one\nline-two\n")), nil
	}
	h := NewLogsHandler(fs, testsupport.NewFakeTargetRegistry(orch), wsTestLogger())

	srv := newWSRouter(authInfo, func(r gin.IRoutes) {
		r.GET("/ws/logs/:appId", h.Handle)
	})
	defer srv.Close()

	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http") + "/ws/logs/" + appID.String() + "?token=" + authInfo.token
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	require.NoError(t, err)
	defer func() { _ = conn.Close() }()

	_, first, err := conn.ReadMessage()
	require.NoError(t, err)
	assert.Equal(t, "line-one", string(first))

	_, second, err := conn.ReadMessage()
	require.NoError(t, err)
	assert.Equal(t, "line-two", string(second))
}

func TestNodeLogsHandlerBadID(t *testing.T) {
	authInfo := newWSTestAuth("admin")
	svc := service.NewNodeService(testsupport.NewFakeStore(), testsupport.NewFakeTargetRegistry(testsupport.NewFakeOrchestrator()), wsTestLogger(), nil)
	h := NewNodeLogsHandler(svc, wsTestLogger())

	srv := newWSRouter(authInfo, func(r gin.IRoutes) {
		r.GET("/ws/nodes/:id/logs", h.Handle)
	})
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/ws/nodes/not-a-uuid/logs?token=" + authInfo.token)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestNodeLogsHandlerStreamsLines(t *testing.T) {
	authInfo := newWSTestAuth("admin")
	svc := service.NewNodeService(testsupport.NewFakeStore(), testsupport.NewFakeTargetRegistry(testsupport.NewFakeOrchestrator()), wsTestLogger(), nil)
	h := NewNodeLogsHandler(svc, wsTestLogger())
	nodeID := uuid.New()

	srv := newWSRouter(authInfo, func(r gin.IRoutes) {
		r.GET("/ws/nodes/:id/logs", h.Handle)
	})
	defer srv.Close()

	msgCh := make(chan string, 1)
	go func() {
		wsURL := "ws" + strings.TrimPrefix(srv.URL, "http") + "/ws/nodes/" + nodeID.String() + "/logs?token=" + authInfo.token
		conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
		if err != nil {
			return
		}
		defer func() { _ = conn.Close() }()
		_, msg, err := conn.ReadMessage()
		if err != nil {
			return
		}
		msgCh <- string(msg)
	}()

	time.Sleep(100 * time.Millisecond)
	svc.PushLogLine(nodeID, "booting node")

	select {
	case line := <-msgCh:
		assert.Equal(t, "booting node", line)
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for node log line")
	}
}
