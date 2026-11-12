package handler

import (
	"context"
	"errors"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/service"
	coderws "github.com/coder/websocket"
	"github.com/gin-gonic/gin"
)

const userRiskPersistenceFailedKey = "ops_user_risk_persistence_failed"
const userRiskTurnAdmissionKey = "ops_user_risk_turn_admission"

type userRiskTurnAdmission struct {
	Turn      int
	Admission service.UserRiskAdmission
}

func (h *OpenAIGatewayHandler) prepareUserRiskTurn(c *gin.Context, ctx context.Context, userID int64, turn int) error {
	unavailable := func(cause error) error {
		return service.NewOpenAIWSClientCloseError(coderws.StatusTryAgainLater, "User policy state is unavailable; reconnect to retry", cause)
	}
	if c == nil || h == nil || h.apiKeyService == nil || turn <= 0 {
		return unavailable(infraerrors.ServiceUnavailable("USER_RISK_STORE_UNAVAILABLE", "User policy check unavailable"))
	}
	if c.GetBool(userRiskPersistenceFailedKey) {
		return unavailable(infraerrors.ServiceUnavailable("USER_RISK_PERSISTENCE_UNAVAILABLE", "A prior policy observation could not be persisted"))
	}
	if value, ok := c.Get(userRiskTurnAdmissionKey); ok {
		if cached, ok := value.(userRiskTurnAdmission); ok && cached.Admission.UserID == userID {
			c.Set(service.UserRiskAdmissionKey, cached.Admission)
			return nil
		}
	}
	admission, err := h.apiKeyService.BeginUserRiskRequest(ctx, userID)
	if err != nil {
		if infraerrors.Code(err) >= 500 {
			return unavailable(err)
		}
		return service.NewOpenAIWSClientCloseError(coderws.StatusPolicyViolation, "API access is restricted; check your account or contact an administrator", err)
	}
	c.Set(userRiskTurnAdmissionKey, userRiskTurnAdmission{Turn: turn, Admission: admission})
	c.Set(service.UserRiskAdmissionKey, admission)
	return nil
}

// A logical request may restart the upstream proxy with its local turn number
// reset. Keep its admission through safe account failover, and retire it only
// after the logical request terminates so the next client frame checks again.
func finishUserRiskTurn(c *gin.Context, turnErr error) {
	var failover *service.UpstreamFailoverError
	if errors.As(turnErr, &failover) {
		return
	}
	c.Set(userRiskTurnAdmissionKey, nil)
	c.Set(service.UserRiskAdmissionKey, nil)
}

type userRiskCyberObserver interface {
	ApplyCyberPolicyUserEvent(context.Context, service.CyberPolicyRecordInput, service.UserRiskAdmission) (*service.UserRiskDecision, error)
}

// This call is intentionally synchronous. Only supplementary logs and mail may
// be queued after the ledger decision; failure fences the remaining WS turns.
func observeUserRiskCyber(c *gin.Context, ctx context.Context, observer userRiskCyberObserver, input service.CyberPolicyRecordInput) (*service.UserRiskDecision, error) {
	value, _ := c.Get(service.UserRiskAdmissionKey)
	admission, _ := value.(service.UserRiskAdmission)
	if observer == nil {
		c.Set(userRiskPersistenceFailedKey, true)
		return nil, infraerrors.ServiceUnavailable("USER_RISK_STORE_UNAVAILABLE", "User policy observation unavailable")
	}
	decision, err := observer.ApplyCyberPolicyUserEvent(ctx, input, admission)
	if err != nil {
		c.Set(userRiskPersistenceFailedKey, true)
	}
	return decision, err
}
