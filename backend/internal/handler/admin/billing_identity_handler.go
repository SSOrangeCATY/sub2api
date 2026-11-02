package admin

import (
	"github.com/Wei-Shaw/sub2api/internal/pkg/adminaccess"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

func (h *UserHandler) MigrateBillingIdentity(c *gin.Context) {
	// A global pricing migration requires a full administrator, never a scoped key.
	if adminaccess.FromContext(c.Request.Context()) != nil {
		response.Forbidden(c, "Full administrator required")
		return
	}
	var req struct {
		Confirm bool `json:"confirm" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || !req.Confirm {
		response.BadRequest(c, "Explicit confirmation required")
		return
	}
	svc, ok := h.adminService.(service.BillingIdentityMigrator)
	if !ok {
		response.InternalError(c, "Billing identity migration unavailable")
		return
	}
	result, err := svc.MigratePersonalRatesToPurchase(c.Request.Context(), getAdminIDFromContext(c))
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, result)
}
