package middleware

import (
	"log/slog"
	"net/url"
	"time"

	"github.com/gin-gonic/gin"
)

// sensitiveQueryParams holds query keys whose values must never be logged.
// WS/SSE routes authenticate via ?token=<JWT>, which is bearer-equivalent.
var sensitiveQueryParams = map[string]bool{
	"token":         true,
	"access_token":  true,
	"refresh_token": true,
}

// redactQuery returns the raw query string with sensitive values replaced by
// "REDACTED". If the query cannot be parsed it is dropped entirely to avoid
// leaking credentials.
func redactQuery(raw string) string {
	values, err := url.ParseQuery(raw)
	if err != nil {
		return "REDACTED"
	}
	redacted := false
	for key, vals := range values {
		if sensitiveQueryParams[key] {
			for i := range vals {
				vals[i] = "REDACTED"
			}
			redacted = true
		}
	}
	if !redacted {
		return raw
	}
	return values.Encode()
}

// Logger returns a Gin middleware that logs requests using slog.
func Logger(logger *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		query := c.Request.URL.RawQuery

		c.Next()

		latency := time.Since(start)
		status := c.Writer.Status()

		attrs := []slog.Attr{
			slog.Int("status", status),
			slog.String("method", c.Request.Method),
			slog.String("path", path),
			slog.Duration("latency", latency),
			slog.String("ip", c.ClientIP()),
		}
		if query != "" {
			attrs = append(attrs, slog.String("query", redactQuery(query)))
		}
		if len(c.Errors) > 0 {
			attrs = append(attrs, slog.String("errors", c.Errors.ByType(gin.ErrorTypePrivate).String()))
		}

		level := slog.LevelInfo
		if status >= 500 {
			level = slog.LevelError
		} else if status >= 400 {
			level = slog.LevelWarn
		}

		logger.LogAttrs(c.Request.Context(), level, "http request", attrs...)
	}
}
