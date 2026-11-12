//go:build unit

package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	coderws "github.com/coder/websocket"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestUserRiskWebSocketQueuedTurnWaitsForDurableCyberDecision(t *testing.T) {
	gin.SetMode(gin.TestMode)
	entered := make(chan service.UserRiskEventInput, 1)
	commit := make(chan struct{})
	secondUpstream := make(chan []byte, 1)
	upstreamDone := make(chan struct{})
	var release sync.Once
	t.Cleanup(func() { release.Do(func() { close(commit) }) })

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer close(upstreamDone)
		conn, err := coderws.Accept(w, r, nil)
		if err != nil {
			return
		}
		defer func() { _ = conn.CloseNow() }()
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()
		if _, _, err = conn.Read(ctx); err != nil {
			return
		}
		failed := []byte(`{"type":"response.failed","response":{"id":"synthetic_policy_barrier","model":"gpt-5.1","error":{"code":"cyber_policy","message":"synthetic policy refusal"},"usage":{"input_tokens":2,"output_tokens":1}}}`)
		if err = conn.Write(ctx, coderws.MessageText, failed); err != nil {
			return
		}
		if _, payload, readErr := conn.Read(ctx); readErr == nil {
			secondUpstream <- append([]byte(nil), payload...)
		}
	}))
	defer upstream.Close()
	harness := newOpenAIWSPassthroughHandlerHarness(t, upstream.URL)
	harness.moderationRepo.setRiskRecord(func(ctx context.Context, input service.UserRiskEventInput) (*service.UserRiskDecision, error) {
		select {
		case entered <- input:
		default:
			t.Error("one logical Cyber refusal must be observed once")
		}
		select {
		case <-commit:
			return &service.UserRiskDecision{
				UserID: input.Admission.UserID, EventID: input.Admission.EventID,
				PolicyKey: input.Admission.PolicyKey, Scope: input.Policy.Scope,
				Generation: input.Admission.Generation, PolicyRevision: input.Policy.Revision,
				HitCount: 1, RecordedAt: time.Now(),
			}, nil
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	})
	ctx, cancel := context.WithTimeout(t.Context(), 7*time.Second)
	defer cancel()
	payload := []byte(`{"type":"response.create","model":"gpt-5.1","input":"synthetic barrier request"}`)
	require.NoError(t, harness.clientConn.Write(ctx, coderws.MessageText, payload))
	select {
	case input := <-entered:
		require.NotEmpty(t, input.Admission.EventID)
		require.Equal(t, harness.apiKey.UserID, input.Admission.UserID)
	case <-time.After(3 * time.Second):
		t.Fatal("the real upstream refusal never reached the durable decision boundary")
	}
	// Queue another real client frame while persistence remains blocked.
	require.NoError(t, harness.clientConn.Write(ctx, coderws.MessageText, payload))
	select {
	case second := <-secondUpstream:
		t.Fatalf("forwarded a queued request before committing the policy decision: %s", second)
	case <-time.After(100 * time.Millisecond):
	}
	release.Do(func() { close(commit) })
	_, terminal, err := harness.clientConn.Read(ctx)
	require.NoError(t, err)
	require.Equal(t, "response.failed", gjson.GetBytes(terminal, "type").String())
	_, _, err = harness.clientConn.Read(ctx)
	var closed coderws.CloseError
	require.ErrorAs(t, err, &closed)
	require.Equal(t, coderws.StatusPolicyViolation, closed.Code)
	select {
	case <-upstreamDone:
	case <-time.After(3 * time.Second):
		t.Fatal("the upstream connection did not close after rejecting the queued turn")
	}
	select {
	case second := <-secondUpstream:
		t.Fatalf("a rejected next turn reached the upstream: %s", second)
	default:
	}
	require.Len(t, harness.moderationRepo.riskEventSnapshot(), 1)
	require.Eventually(t, func() bool { return len(harness.moderationRepo.logSnapshot()) == 1 },
		time.Second, 10*time.Millisecond)
}
