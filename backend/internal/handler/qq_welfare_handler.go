package handler

import (
	"net/http"

	"github.com/Wei-Shaw/sub2api/internal/pkg/adminaccess"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

type QQWelfareHandler struct{ svc *service.QQWelfareService }

func NewQQWelfareHandler(svc *service.QQWelfareService) *QQWelfareHandler {
	return &QQWelfareHandler{svc: svc}
}
func (h *QQWelfareHandler) Binding(c *gin.Context) {
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "请先登录。 ")
		return
	}
	data, err := h.svc.Binding(c.Request.Context(), subject.UserID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, data)
}
func (h *QQWelfareHandler) BindingCode(c *gin.Context) {
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "请先登录。 ")
		return
	}
	code, err := h.svc.BindingCode(c.Request.Context(), subject.UserID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	c.Header("Cache-Control", "no-store")
	response.Success(c, gin.H{"code": code, "expires_in": 600})
}
func (h *QQWelfareHandler) Center(c *gin.Context) {
	h.userAction(c, true)
}
func (h *QQWelfareHandler) UserAction(c *gin.Context) {
	h.userAction(c, false)
}
func (h *QQWelfareHandler) userAction(c *gin.Context, overview bool) {
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "请先登录。")
		return
	}
	var req struct {
		Action         string `json:"action"`
		RequestID      string `json:"request_id"`
		Code           string `json:"code"`
		ProductID      int64  `json:"product_id"`
		ProductVersion int64  `json:"product_version"`
	}
	if overview {
		req.Action = "overview"
	} else {
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 4096)
		if err := c.ShouldBindJSON(&req); err != nil {
			response.BadRequest(c, "福利请求格式错误。")
			return
		}
	}
	data, err := h.svc.UserAction(c.Request.Context(), subject.UserID, req.Action, req.RequestID, req.Code, req.ProductID, req.ProductVersion)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	c.Header("Cache-Control", "no-store")
	response.Success(c, data)
}
func (h *QQWelfareHandler) Bot(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 4096)
	if err := h.svc.Authenticate(c.Request.Context(), c.GetHeader("X-Welfare-Bot-Key")); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	var event service.QQWelfareEvent
	if err := c.ShouldBindJSON(&event); err != nil {
		response.BadRequest(c, "机器人请求格式错误。 ")
		return
	}
	data, err := h.svc.Event(c.Request.Context(), event)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	c.Header("Cache-Control", "no-store")
	response.Success(c, data)
}
func welfareFullAdmin(c *gin.Context) bool {
	// Account-scoped admin keys cannot manage cross-user rewards or bot credentials.
	if adminaccess.FromContext(c.Request.Context()) != nil {
		response.Forbidden(c, "福利管理需要完整管理员权限。")
		return false
	}
	return true
}
func (h *QQWelfareHandler) Admin(c *gin.Context) {
	if !welfareFullAdmin(c) {
		return
	}
	data, err := h.svc.Admin(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	c.Header("Cache-Control", "no-store")
	response.Success(c, data)
}
func (h *QQWelfareHandler) Config(c *gin.Context) {
	if !welfareFullAdmin(c) {
		return
	}
	var req struct {
		Enabled     bool   `json:"enabled"`
		GroupNumber string `json:"group_number"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "配置格式错误。")
		return
	}
	if err := h.svc.UpdateConfig(c.Request.Context(), req.Enabled, req.GroupNumber); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"saved": true})
}
func (h *QQWelfareHandler) GroupCode(c *gin.Context) {
	if !welfareFullAdmin(c) {
		return
	}
	code, err := h.svc.GroupCode(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	c.Header("Cache-Control", "no-store")
	response.Success(c, gin.H{"code": code, "expires_in": 600})
}
func (h *QQWelfareHandler) Product(c *gin.Context) {
	if !welfareFullAdmin(c) {
		return
	}
	var p service.QQWelfareProduct
	if err := c.ShouldBindJSON(&p); err != nil {
		response.BadRequest(c, "商品格式错误。")
		return
	}
	if err := h.svc.SaveProduct(c.Request.Context(), &p); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, p)
}
func (h *QQWelfareHandler) Coupon(c *gin.Context) {
	if !welfareFullAdmin(c) {
		return
	}
	var p service.QQWelfareCoupon
	if err := c.ShouldBindJSON(&p); err != nil {
		response.BadRequest(c, "兑换码格式错误。")
		return
	}
	if err := h.svc.SaveCoupon(c.Request.Context(), &p); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, p)
}
