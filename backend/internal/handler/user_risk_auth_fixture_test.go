//go:build unit

package handler

import (
	"context"
	"crypto/rand"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	coderws "github.com/coder/websocket"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type gatewayRiskTestUserRepo struct {
	service.UserRepository
	blocked atomic.Bool
}

func (r *gatewayRiskTestUserRepo) BeginUserRiskRequest(_ context.Context, userID int64, policy, scope string) (service.UserRiskAdmission, error) {
	if r.blocked.Load() {
		return service.UserRiskAdmission{}, infraerrors.Forbidden("USER_RISK_RESTRICTED", "API access is restricted")
	}
	return service.UserRiskAdmission{UserID: userID, PolicyKey: policy, Scope: scope,
		EventID: rand.Text(), Generation: 1, AdmittedAt: time.Now()}, nil
}

func newGatewayRiskAPIKeyService(
	keyRepo service.APIKeyRepository, userRepo service.UserRepository,
	groupRepo service.GroupRepository, subscriptionRepo service.UserSubscriptionRepository,
	groupRateRepo service.UserGroupRateRepository, cache service.APIKeyCache, cfg *config.Config,
) *service.APIKeyService {
	if userRepo == nil {
		userRepo = &gatewayRiskTestUserRepo{}
	}
	return service.NewAPIKeyService(keyRepo, userRepo, groupRepo, subscriptionRepo, groupRateRepo, cache, cfg)
}

func newGatewayRiskEmptyAPIKeyService() *service.APIKeyService {
	return newGatewayRiskAPIKeyService(nil, nil, nil, nil, nil, nil, &config.Config{})
}

func (r *qqWSGateRepository) BeginUserRiskRequest(ctx context.Context, userID int64, policy, scope string) (service.UserRiskAdmission, error) {
	return (&gatewayRiskTestUserRepo{}).BeginUserRiskRequest(ctx, userID, policy, scope)
}

func TestUserRiskWebSocketRechecksRestrictionAfterHandshakeBeforeFirstFrame(t *testing.T) {
	gin.SetMode(gin.TestMode)
	var acquired atomic.Int32
	cache := &concurrencyCacheMock{acquireUserSlotFn: func(context.Context, int64, int, string) (bool, error) {
		acquired.Add(1)
		return true, nil
	}}
	h := newOpenAIHandlerForPreviousResponseIDValidation(t, cache)
	users := &gatewayRiskTestUserRepo{}
	h.apiKeyService = newGatewayRiskAPIKeyService(nil, users, nil, nil, nil, nil, nil)
	server := newOpenAIWSHandlerTestServer(t, h, middleware.AuthSubject{UserID: 7, Concurrency: 1})
	defer server.Close()
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	conn, _, err := coderws.Dial(ctx, "ws"+strings.TrimPrefix(server.URL, "http")+"/openai/v1/responses", nil)
	require.NoError(t, err)
	defer func() { _ = conn.CloseNow() }()
	// The upgraded connection is idle when another request restricts this user.
	users.blocked.Store(true)
	require.NoError(t, conn.Write(ctx, coderws.MessageText, []byte(`{"type":"response.create","model":"gpt-test","input":"hello"}`)))
	_, _, err = conn.Read(ctx)
	var closed coderws.CloseError
	require.ErrorAs(t, err, &closed)
	require.Equal(t, coderws.StatusPolicyViolation, closed.Code)
	require.Zero(t, acquired.Load(), "the first frame must be rejected before billing or upstream acquisition")
}
