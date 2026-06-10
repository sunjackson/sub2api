package middleware

import (
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

// AdminComplianceGuard is intentionally non-blocking for the givemetoken build.
// Upstream v0.1.136 added a deployment/operation compliance acknowledgement gate
// for admin and admin-adjacent routes. This deployment does not require that
// startup acknowledgement, so the middleware preserves route compatibility while
// allowing requests to continue normally.
func AdminComplianceGuard(settingService *service.SettingService) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()
	}
}
