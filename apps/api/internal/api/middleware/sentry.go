package middleware

import (
	"github.com/getsentry/sentry-go"
	sentrygin "github.com/getsentry/sentry-go/gin"
	"github.com/gin-gonic/gin"
)

// SentryMiddleware captures errors and panics to Sentry.
// No-op if Sentry is not initialized (no DSN configured).
func Sentry() gin.HandlerFunc {
	if sentry.CurrentHub().Client() == nil {
		return func(c *gin.Context) { c.Next() }
	}
	return sentrygin.New(sentrygin.Options{Repanic: true})
}
