package handler

import (
	"context"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	coderws "github.com/coder/websocket"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type qqWSGateRepository struct {
	service.UserRepository
	blocked atomic.Bool
}

func (r *qqWSGateRepository) CheckQQAPIAccess(context.Context, int64) error {
	if r.blocked.Load() {
		return infraerrors.Forbidden("QQ_VERIFICATION_REQUIRED", "QQ verification required")
	}
	return nil
}

func TestQQWebSocketRechecksPolicyAfterHandshakeBeforeFirstFrame(t *testing.T) {
	gin.SetMode(gin.TestMode)
	var acquired atomic.Int32
	cache := &concurrencyCacheMock{acquireUserSlotFn: func(context.Context, int64, int, string) (bool, error) { acquired.Add(1); return true, nil }}
	h := newOpenAIHandlerForPreviousResponseIDValidation(t, cache)
	users := &qqWSGateRepository{}
	h.apiKeyService = service.NewAPIKeyService(nil, users, nil, nil, nil, nil, nil)
	require.NoError(t, h.apiKeyService.CheckQQAPIAccess(t.Context(), 7))
	server := newOpenAIWSHandlerTestServer(t, h, middleware.AuthSubject{UserID: 7, Concurrency: 1})
	defer server.Close()
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	conn, _, err := coderws.Dial(ctx, "ws"+strings.TrimPrefix(server.URL, "http")+"/openai/v1/responses", nil)
	require.NoError(t, err)
	defer func() { _ = conn.CloseNow() }()
	// The administrator enables verification while the upgraded connection is idle.
	users.blocked.Store(true)
	require.NoError(t, conn.Write(ctx, coderws.MessageText, []byte(`{"type":"response.create","model":"gpt-test","input":"hello"}`)))
	_, _, err = conn.Read(ctx)
	var closed coderws.CloseError
	require.ErrorAs(t, err, &closed)
	require.Equal(t, coderws.StatusPolicyViolation, closed.Code)
	require.Contains(t, closed.Reason, "QQ verification")
	require.Zero(t, acquired.Load(), "QQ rejection precedes user concurrency/billing/upstream work")
}

func TestQQWebSocketRechecksPolicyOnSubsequentTurns(t *testing.T) {
	for _, mode := range []string{service.OpenAIWSIngressModePassthrough, service.OpenAIWSIngressModeCtxPool} {
		t.Run(mode, func(t *testing.T) {
			users := &qqWSGateRepository{}
			keys := newWSGroupPricingAPIKeyRepoStub(3)
			apiKeys := service.NewAPIKeyService(keys, users, nil, nil, nil, nil, nil)
			got := runOpenAIResponsesWebSocketUsageLogCase(t, openAIResponsesWSUsageLogCase{
				firstPayload: wsGroupPricingFirst, secondPayload: wsGroupPricingSecond, ingressMode: mode,
				apiKeyService: apiKeys, apiKeyCredential: wsGroupPricingKey, secondTurnCloseExpected: true, closeReason: "QQ verification",
				afterFirstUpstreamRequest: func(*service.ChannelService) error { users.blocked.Store(true); return nil },
			})
			require.Len(t, got.upstreamPayloads, 1, "second turn must not reach upstream")
			require.Len(t, got.logs, 1, "only the completed first turn is billed")
		})
	}
}
