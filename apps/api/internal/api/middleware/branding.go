package middleware

import (
	"github.com/gin-gonic/gin"
	"github.com/orkai-dev/orkai/apps/api/internal/version"
)

func Branding() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("X-Powered-By", version.Name)
		c.Next()
	}
}
