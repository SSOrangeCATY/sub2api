//go:build unit

package handler

import (
	"context"
	"fmt"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type wsRiskAdmissionRepo struct {
	service.UserRepository
	calls int
}

func (r *wsRiskAdmissionRepo) BeginUserRiskRequest(_ context.Context, userID int64, policy, scope string) (service.UserRiskAdmission, error) {
	r.calls++
	return service.UserRiskAdmission{UserID: userID, PolicyKey: policy, Scope: scope,
		EventID: fmt.Sprintf("server-event-%d", r.calls), Generation: 1, AdmittedAt: time.Unix(10, 0)}, nil
}

func TestUserRiskGatewayTurnIdentitySurvivesAccountRetries(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := &wsRiskAdmissionRepo{}
	keys := service.NewAPIKeyService(nil, repo, nil, nil, nil, nil, &config.Config{RunMode: config.RunModeSimple})
	h := &OpenAIGatewayHandler{apiKeyService: keys}
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx := context.Background()
	c.Set(service.UserRiskAdmissionKey, service.UserRiskAdmission{EventID: "handshake-only", Generation: 99})
	require.NoError(t, h.prepareUserRiskTurn(c, ctx, 17, 1))
	first, _ := c.Get(service.UserRiskAdmissionKey)
	require.NoError(t, h.prepareUserRiskTurn(c, ctx, 17, 1))
	retry, _ := c.Get(service.UserRiskAdmissionKey)
	require.Equal(t, first, retry)
	require.Equal(t, 1, repo.calls)
	finishUserRiskTurn(c, nil)
	require.NoError(t, h.prepareUserRiskTurn(c, ctx, 17, 2))
	next, _ := c.Get(service.UserRiskAdmissionKey)
	require.NotEqual(t, first, next)
	require.Equal(t, 2, repo.calls)
	finishUserRiskTurn(c, &service.UpstreamFailoverError{})
	require.NoError(t, h.prepareUserRiskTurn(c, ctx, 17, 1))
	restarted, _ := c.Get(service.UserRiskAdmissionKey)
	require.Equal(t, next, restarted, "proxy re-entry retains the pending logical request")
	require.Equal(t, 2, repo.calls)
	finishUserRiskTurn(c, nil)
	require.NoError(t, h.prepareUserRiskTurn(c, ctx, 17, 2))
	later, _ := c.Get(service.UserRiskAdmissionKey)
	require.NotEqual(t, next, later, "a reused local turn number must get fresh admission after completion")
	require.Equal(t, 3, repo.calls)
	c.Set(userRiskPersistenceFailedKey, true)
	require.Error(t, h.prepareUserRiskTurn(c, ctx, 17, 3))
	require.Equal(t, 3, repo.calls, "another turn cannot begin after an unpersisted policy observation")
}
