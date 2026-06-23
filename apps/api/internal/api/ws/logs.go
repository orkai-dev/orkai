package ws

import (
	"bufio"
	"context"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"

	"github.com/orkai-dev/orkai/apps/api/internal/api/middleware"
	"github.com/orkai-dev/orkai/apps/api/internal/model"
	"github.com/orkai-dev/orkai/apps/api/internal/orchestrator"
	"github.com/orkai-dev/orkai/apps/api/internal/store"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: checkSameOrigin,
}

// checkSameOrigin allows requests without an Origin header (non-browser
// clients) and otherwise requires the Origin host to match the request host,
// preventing cross-site WebSocket hijacking.
//
// The Origin header is browser-controlled and cannot be forged by cross-site
// JavaScript, so it is the trustworthy side of the comparison. The expected
// host is taken from r.Host and, when a proxy rewrites Host, from
// X-Forwarded-Host — so legitimate upgrades survive proxies that do not
// preserve the original Host header.
func checkSameOrigin(r *http.Request) bool {
	origin := r.Header.Get("Origin")
	if origin == "" {
		return true
	}
	u, err := url.Parse(origin)
	if err != nil || u.Host == "" {
		return false
	}
	if strings.EqualFold(u.Host, r.Host) {
		return true
	}
	// Fall back to the forwarded host for proxies that rewrite Host
	// (e.g. changeOrigin-style reverse proxies). Only the first value matters.
	if fwd := r.Header.Get("X-Forwarded-Host"); fwd != "" {
		fwd = strings.TrimSpace(strings.Split(fwd, ",")[0])
		if strings.EqualFold(u.Host, fwd) {
			return true
		}
	}
	return false
}

type LogsHandler struct {
	store   store.Store
	targets *orchestrator.TargetRegistry
	logger  *slog.Logger
}

func NewLogsHandler(s store.Store, targets *orchestrator.TargetRegistry, logger *slog.Logger) *LogsHandler {
	return &LogsHandler{store: s, targets: targets, logger: logger}
}

func (h *LogsHandler) Handle(c *gin.Context) {
	appID, err := uuid.Parse(c.Param("appId"))
	if err != nil {
		c.JSON(400, gin.H{"error": "invalid app ID"})
		return
	}

	app, err := h.store.Applications().GetByID(c.Request.Context(), appID)
	if err != nil {
		c.JSON(404, gin.H{"error": "app not found"})
		return
	}

	// Verify app belongs to the authenticated user's org
	project, err := h.store.Projects().GetByID(c.Request.Context(), app.ProjectID)
	if err != nil || project.OrgID != middleware.GetOrgID(c) {
		c.JSON(403, gin.H{"error": "access denied"})
		return
	}

	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		h.logger.Error("websocket upgrade failed", slog.Any("error", err))
		return
	}
	defer func() { _ = conn.Close() }()

	podName := c.Query("pod")
	timestamps := c.Query("timestamps") == "true"

	h.streamLogs(conn, app, podName, timestamps)
}

func (h *LogsHandler) streamLogs(conn *websocket.Conn, app *model.Application, podName string, timestamps bool) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Read pump: detect client disconnect
	go func() {
		defer cancel()
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				return
			}
		}
	}()

	opts := orchestrator.LogOpts{
		Follow:     true,
		TailLines:  100,
		Timestamps: timestamps,
	}

	target, terr := h.targets.For(app)
	if terr != nil {
		_ = conn.WriteMessage(websocket.TextMessage, []byte("Error: "+terr.Error()))
		return
	}
	logs, lerr := orchestrator.AsCapability[orchestrator.LogStreamer](target, orchestrator.CapLogs)
	if lerr != nil {
		_ = conn.WriteMessage(websocket.TextMessage, []byte("Error: "+lerr.Error()))
		return
	}

	var reader interface {
		Read(p []byte) (n int, err error)
		Close() error
	}
	var err error

	if podName != "" {
		reader, err = logs.StreamPodLogs(ctx, app, podName, opts)
	} else {
		reader, err = logs.StreamLogs(ctx, app, opts)
	}
	if err != nil {
		_ = conn.WriteMessage(websocket.TextMessage, []byte("Error: "+err.Error()))
		return
	}
	defer func() { _ = reader.Close() }()

	h.logger.Info("log stream started", slog.String("app", app.Name))

	// Stream logs in a goroutine
	done := make(chan struct{})
	go func() {
		defer close(done)
		scanner := bufio.NewScanner(reader)
		for scanner.Scan() {
			line := scanner.Text()
			if err := conn.WriteMessage(websocket.TextMessage, []byte(line)); err != nil {
				return
			}
		}
	}()

	// Wait for either stream end or client disconnect
	select {
	case <-done:
		// Stream ended, keep alive with heartbeats until client disconnects
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
					return
				}
			}
		}
	case <-ctx.Done():
		return
	}
}
