package admin

import (
	"fmt"
	"strconv"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

func parseUserRiskListFilter(c *gin.Context) (service.UserRiskListFilter, error) {
	filter := service.UserRiskListFilter{
		Cursor: c.Query("cursor"),
		Page:   1, PageSize: 20, Search: c.Query("search"),
		ReleaseMode: c.Query("release_mode"), Action: c.Query("action"),
	}
	for name, target := range map[string]*int{"page": &filter.Page, "page_size": &filter.PageSize} {
		raw := c.Query(name)
		if raw == "" {
			continue
		}
		value, err := strconv.Atoi(raw)
		if err != nil || value <= 0 {
			return filter, fmt.Errorf("%s must be a positive integer", name)
		}
		*target = value
	}
	if raw := c.Query("user_id"); raw != "" {
		value, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || value <= 0 {
			return filter, fmt.Errorf("user_id must be a positive integer")
		}
		filter.UserID = value
	}
	return filter, nil
}

func (h *ContentModerationHandler) ListUserRestrictions(c *gin.Context) {
	filter, err := parseUserRiskListFilter(c)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	result, err := h.service.ListUserRiskRestrictions(c.Request.Context(), filter)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, result)
}

func (h *ContentModerationHandler) ListUserRestrictionActions(c *gin.Context) {
	filter, err := parseUserRiskListFilter(c)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	result, err := h.service.ListUserRiskActions(c.Request.Context(), filter)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, result)
}

type userRiskReleaseRequest struct {
	OperationID     string `json:"operation_id"`
	Reason          string `json:"reason"`
	ExpectedVersion int64  `json:"expected_version"`
}

func (h *ContentModerationHandler) ReleaseUserRestriction(c *gin.Context) {
	userID, err := strconv.ParseInt(c.Param("user_id"), 10, 64)
	if err != nil || userID <= 0 {
		response.BadRequest(c, "user_id must be a positive integer")
		return
	}
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok || subject.UserID <= 0 {
		response.ErrorFrom(c, infraerrors.Forbidden("ADMIN_ACTOR_REQUIRED", "请使用具备管理员身份的账户执行解封"))
		return
	}
	var request userRiskReleaseRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		response.BadRequest(c, "Invalid release request")
		return
	}
	result, err := h.service.ReleaseUserRiskRestriction(c.Request.Context(), service.UserRiskReleaseInput{
		UserID: userID, ActorID: subject.UserID, OperationID: request.OperationID,
		Reason: request.Reason, ExpectedVersion: request.ExpectedVersion,
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, result)
}
